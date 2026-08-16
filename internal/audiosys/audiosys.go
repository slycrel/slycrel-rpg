// Package audiosys plays the game's sound.
//
// It follows the same rule as the art registry: a missing or unplayable sound
// is silence, never a crash. That keeps the game runnable for anyone who does
// not have the purchased sound packs on disk, and it means a broken cue never
// takes the build down.
//
// Two kinds of sound, handled differently:
//
//	One-shots   decoded once into PCM and cached, then played from bytes so
//	            the same hit can overlap itself without re-reading the file
//	Ambience    streamed from disk under an infinite loop, because a two
//	            minute bed is tens of megabytes decoded and only ever needs
//	            one player
//
// Every cue can have several source files. Play picks one at random, which is
// the difference between a sword that lands and a sword that clicks the same
// way forty times a minute.
package audiosys

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/slycrel/slycrel-rpg/internal/core"
)

// SampleRate matches the source packs (44.1 kHz), so nothing is resampled.
const SampleRate = 44100

// maxVoices caps simultaneous one-shots. A three-monster round can fire a
// dozen cues in a second; past this it is mud, and every extra player is
// another goroutine's worth of mixing for no audible gain.
const maxVoices = 16

// entry is one manifest record.
type entry struct {
	Key   string   `json:"key"`
	Files []string `json:"files"`
	Loop  bool     `json:"loop,omitempty"`
}

type manifest struct {
	Entries []entry `json:"entries"`
}

// Settings is the persisted audio preference.
type Settings struct {
	Muted  bool    `json:"muted"`
	Volume float64 `json:"volume"`
}

// ctxOnce guards the single Ebitengine audio context; constructing a second
// one panics.
var (
	ctxOnce sync.Once
	ctx     *audio.Context
)

// Bank owns every sound the game can make.
type Bank struct {
	root string
	man  map[string]entry
	rng  *core.RNG

	// pcm caches decoded one-shots, keyed by file path.
	pcm map[string][]byte

	// live holds one-shots currently sounding, reaped by Update.
	live []*audio.Player

	// ambience is the single looping bed, with the file kept open beneath it.
	ambPlayer *audio.Player
	ambFile   *os.File
	ambKey    string

	settings Settings
	// off disables everything: no manifest, or the caller asked for silence.
	off bool
}

// New opens a bank rooted at the repository root, reading assets/audio.json.
// A missing manifest is not an error; the game simply runs quiet.
func New(root string, seed int64) *Bank {
	b := &Bank{
		root:     root,
		man:      map[string]entry{},
		pcm:      map[string][]byte{},
		rng:      core.NewRNG(seed).Fork("audio", seed),
		settings: Settings{Volume: 0.7},
	}
	b.loadSettings()

	data, err := os.ReadFile(filepath.Join(root, "assets", "audio.json"))
	if err != nil {
		b.off = true
		return b
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "audiosys: audio.json is malformed (%v); running silent\n", err)
		b.off = true
		return b
	}
	for _, e := range m.Entries {
		b.man[e.Key] = e
	}

	ctxOnce.Do(func() { ctx = audio.NewContext(SampleRate) })
	return b
}

// Inventory reports how many cues are wired and how many source files back
// them, so an audit can tell "silent because muted" from "silent because
// nothing was ever indexed".
func (b *Bank) Inventory() (cues, files int) {
	for _, e := range b.man {
		cues++
		files += len(e.Files)
	}
	return cues, files
}

// Verify decodes every cue and reports what failed. Counting manifest entries
// only proves the JSON parsed; this proves the files exist and are in a format
// the engine will actually accept, which is the failure that would otherwise
// surface as a mysteriously silent cue mid-fight.
func (b *Bank) Verify() []error {
	var errs []error
	keys := make([]string, 0, len(b.man))
	for k := range b.man {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		e := b.man[k]
		for _, path := range e.Files {
			f, err := os.Open(filepath.Join(b.root, path))
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", k, err))
				continue
			}
			stream, err := wav.DecodeWithSampleRate(SampleRate, f)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %s: %w", k, filepath.Base(path), err))
				f.Close()
				continue
			}
			// Loops stream, so length is enough; one-shots must read through.
			if e.Loop {
				if stream.Length() == 0 {
					errs = append(errs, fmt.Errorf("%s: %s decodes to nothing", k, filepath.Base(path)))
				}
			} else if n, err := io.Copy(io.Discard, stream); err != nil || n == 0 {
				errs = append(errs, fmt.Errorf("%s: %s: read %d bytes: %v", k, filepath.Base(path), n, err))
			}
			f.Close()
		}
	}
	return errs
}

// Silence disables the bank entirely, for -mute and for headless runs.
func (b *Bank) Silence() { b.off = true; b.Ambience("") }

// Enabled reports whether anything will actually be heard.
func (b *Bank) Enabled() bool { return !b.off && !b.settings.Muted && b.settings.Volume > 0 }

// Muted reports the user's mute preference.
func (b *Bank) Muted() bool { return b.settings.Muted }

// Volume returns the master volume in [0,1].
func (b *Bank) Volume() float64 { return b.settings.Volume }

// SetMuted turns sound off or back on and persists the choice.
func (b *Bank) SetMuted(m bool) {
	b.settings.Muted = m
	if m {
		b.stopAmbience()
	} else if b.ambKey != "" {
		// Restart the bed that was playing when sound was turned off.
		key := b.ambKey
		b.ambKey = ""
		b.Ambience(key)
	}
	b.saveSettings()
}

// SetVolume sets master volume in [0,1] and persists it.
func (b *Bank) SetVolume(v float64) {
	b.settings.Volume = core.ClampF(v, 0, 1)
	if b.ambPlayer != nil {
		b.ambPlayer.SetVolume(b.settings.Volume * ambienceMix)
	}
	b.saveSettings()
}

// ambienceMix keeps beds well under the one-shots; ambience that competes with
// a sword hit is ambience turned up too far.
const ambienceMix = 0.30

// Play fires a one-shot cue. Unknown keys are ignored.
func (b *Bank) Play(key string) { b.PlayVolume(key, 1) }

// PlayVolume fires a one-shot at a relative volume, for cues that should sit
// back a little (an off-screen monster, a repeated footfall).
func (b *Bank) PlayVolume(key string, rel float64) {
	if !b.Enabled() {
		return
	}
	e, ok := b.man[key]
	if !ok || len(e.Files) == 0 || e.Loop {
		return
	}
	if len(b.live) >= maxVoices {
		return
	}

	path := core.Pick(b.rng, e.Files)
	data, err := b.decode(path)
	if err != nil || len(data) == 0 {
		return
	}
	p := ctx.NewPlayerFromBytes(data)
	p.SetVolume(core.ClampF(b.settings.Volume*rel, 0, 1))
	p.Play()
	b.live = append(b.live, p)
}

// decode reads and caches a one-shot as raw PCM.
func (b *Bank) decode(path string) ([]byte, error) {
	if data, ok := b.pcm[path]; ok {
		return data, nil
	}
	f, err := os.Open(filepath.Join(b.root, path))
	if err != nil {
		b.pcm[path] = nil // remember the failure; do not retry every frame
		return nil, err
	}
	defer f.Close()

	stream, err := wav.DecodeWithSampleRate(SampleRate, f)
	if err != nil {
		b.pcm[path] = nil
		return nil, err
	}
	data, err := io.ReadAll(stream)
	if err != nil {
		b.pcm[path] = nil
		return nil, err
	}
	b.pcm[path] = data
	return data, nil
}

// Ambience switches the looping bed. Passing the key already playing is a
// no-op, so callers can set it every frame from wherever the player is.
// An empty key stops ambience.
func (b *Bank) Ambience(key string) {
	if key == b.ambKey {
		return
	}
	b.stopAmbience()
	b.ambKey = key
	if key == "" || !b.Enabled() {
		return
	}

	e, ok := b.man[key]
	if !ok || len(e.Files) == 0 {
		return
	}
	// The file stays open underneath the player: ambience streams rather than
	// being held in memory, because a two minute bed is tens of megabytes.
	f, err := os.Open(filepath.Join(b.root, e.Files[0]))
	if err != nil {
		return
	}
	stream, err := wav.DecodeWithSampleRate(SampleRate, f)
	if err != nil {
		f.Close()
		return
	}
	p, err := ctx.NewPlayer(audio.NewInfiniteLoop(stream, stream.Length()))
	if err != nil {
		f.Close()
		return
	}
	p.SetVolume(b.settings.Volume * ambienceMix)
	p.Play()
	b.ambPlayer, b.ambFile = p, f
}

func (b *Bank) stopAmbience() {
	if b.ambPlayer != nil {
		b.ambPlayer.Pause()
		_ = b.ambPlayer.Close()
		b.ambPlayer = nil
	}
	if b.ambFile != nil {
		_ = b.ambFile.Close()
		b.ambFile = nil
	}
}

// Update reaps finished one-shots. Call once per tick; without it players
// accumulate for the lifetime of the process.
func (b *Bank) Update() {
	if len(b.live) == 0 {
		return
	}
	keep := b.live[:0]
	for _, p := range b.live {
		if p.IsPlaying() {
			keep = append(keep, p)
			continue
		}
		_ = p.Close()
	}
	b.live = keep
}

// Close stops everything.
func (b *Bank) Close() {
	b.stopAmbience()
	for _, p := range b.live {
		_ = p.Close()
	}
	b.live = nil
}

// settingsPath is where the audio preference lives. It sits beside the saves
// rather than in them, because it describes the player's speakers, not their
// character.
func (b *Bank) settingsPath() string {
	return filepath.Join(b.root, "saves", "settings.json")
}

func (b *Bank) loadSettings() {
	data, err := os.ReadFile(b.settingsPath())
	if err != nil {
		return
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	if s.Volume <= 0 || s.Volume > 1 {
		s.Volume = 0.7
	}
	b.settings = s
}

func (b *Bank) saveSettings() {
	if err := os.MkdirAll(filepath.Dir(b.settingsPath()), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(b.settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(b.settingsPath(), data, 0o644)
}
