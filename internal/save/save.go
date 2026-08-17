// Package save reads and writes game saves.
//
// The format is deliberately small and mostly human-readable. The continent is
// never stored — it is a pure function of the seed — so a save is the seed plus
// whatever the player has changed since: their character, where they are
// standing, which locations they have found, and which chests they have already
// emptied. That keeps a save a few kilobytes instead of a few megabytes, and it
// makes a save file usable as a test fixture you can open and edit by hand.
//
// The one exception is the exploration fog, which is 19,200 bits and would
// drown the rest of the file as JSON booleans. It is packed and base64'd.
package save

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/sky"
	"github.com/slycrel/slycrel-rpg/internal/thread"
)

// Version is the save format revision. Load refuses anything it does not know,
// rather than silently misreading an older layout.
//
// v2 added the party. v3 added the companions' backstories. Older saves are
// still read: a v1 file describes a run with no companions in it and a v2 file
// a company nobody has asked about themselves yet, both of which are lossless
// readings rather than guesses. The bump is about refusing a *future* format,
// not about abandoning old files — and a v2 company is handed threads on the
// way in, so an old save is not stuck without them either.
const Version = 3

// minVersion is the oldest layout this build can still read correctly.
const minVersion = 1

// Dir is the save folder, relative to the repo root.
const Dir = "saves"

// File is a complete save.
type File struct {
	Version int    `json:"version"`
	Saved   string `json:"saved"`   // RFC3339, for the load menu
	Summary string `json:"summary"` // one line describing the run, for the load menu

	// Seed regenerates the entire continent, so no terrain is stored.
	Seed int64 `json:"seed"`

	Player *model.Character `json:"player"`

	// Allies are the hirelings, in marching order. Their positions are not
	// stored: the line re-forms on the hero's tile when the save is loaded,
	// which is indistinguishable from walking into the room together.
	Allies []*model.Character `json:"allies,omitempty"`

	// At is the overworld tile the player occupies. Facing is a core.Dir.
	At     core.Point `json:"at"`
	Facing int        `json:"facing"`

	// Inside is set when the save was taken within a location.
	Inside *Inside `json:"inside,omitempty"`

	// POIs is parallel to the generated world's location list, which is
	// stable for a given seed.
	POIs []POIState `json:"pois"`

	// Fog is a packed bitset of explored tiles, base64'd. Width*Height bits,
	// least significant bit first.
	Fog string `json:"fog"`

	SinceFight int `json:"sinceFight"`

	// Clock is the time of day, in steps taken. Absent in a save written before
	// there was a sky, which reads back as the first dawn of the run — a fair
	// answer, and the only one an old save can honestly give.
	//
	// The weather is deliberately not here. It is derived from the seed, this
	// clock and the biome, so storing it would be storing a second copy of
	// something already recoverable, with the usual consequence: a save whose
	// recorded downpour disagrees with the world it is standing in.
	Clock sky.Clock `json:"clock"`

	// Quests are stored whole. They are a handful of indices and counters, so
	// there is nothing to reconstruct and nothing that can drift out of step
	// with a regenerated world.
	Quests []*quest.Quest `json:"quests,omitempty"`

	// Threads are the companions' backstories, keyed to their owner by name.
	// Like quests they are stored whole, because the cast was frozen when the
	// thread was first handed out and re-deriving it would be re-rolling it.
	Threads []*thread.Thread `json:"threads,omitempty"`
}

// Inside records a position within a location's interior.
type Inside struct {
	POI    int        `json:"poi"` // index into File.POIs
	At     core.Point `json:"at"`
	Facing int        `json:"facing"`
}

// POIState is the mutable half of a point of interest.
type POIState struct {
	Discovered bool `json:"discovered,omitempty"`
	Visited    bool `json:"visited,omitempty"`
	Cleared    bool `json:"cleared,omitempty"`

	// Used records the things the player has already dealt with: chests
	// emptied, foes killed, altars prayed at.
	//
	// These are keyed by kind and position rather than by index into the
	// entity list, because that list is not stable across a save. Clearing a
	// dungeon removes its boss from generation, so every index after it
	// shifts; a position never does.
	Used []UsedEntity `json:"used,omitempty"`
}

// UsedEntity identifies one spent interactable within a location.
type UsedEntity struct {
	Kind string `json:"kind"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

// PackFog compresses a per-tile explored flag list into base64.
func PackFog(explored []bool) string {
	buf := make([]byte, (len(explored)+7)/8)
	for i, v := range explored {
		if v {
			buf[i/8] |= 1 << uint(i%8)
		}
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// UnpackFog expands base64 fog back into n flags. A short or corrupt string
// yields the tiles it could read and leaves the rest unexplored, which loses
// map progress but never fails a load.
func UnpackFog(s string, n int) []bool {
	out := make([]bool, n)
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return out
	}
	for i := 0; i < n && i/8 < len(buf); i++ {
		out[i] = buf[i/8]&(1<<uint(i%8)) != 0
	}
	return out
}

// Path returns the file path for a named slot.
func Path(root, slot string) string {
	return filepath.Join(root, Dir, Clean(slot)+".json")
}

// Clean reduces a slot name to something safe for a filename.
func Clean(slot string) string {
	slot = strings.TrimSpace(strings.ToLower(slot))
	var b strings.Builder
	for _, r := range slot {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == ' ':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "save"
	}
	return b.String()
}

// Write saves f to a slot under root. It writes to a temporary file and
// renames, so an interrupted save cannot destroy the previous one.
func Write(root, slot string, f *File) error {
	f.Version = Version
	f.Saved = time.Now().Format(time.RFC3339)

	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding save: %w", err)
	}
	data = append(data, '\n')

	final := Path(root, slot)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("replacing %s: %w", final, err)
	}
	return nil
}

// Read loads a save from an explicit path.
func Read(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	if f.Version < minVersion || f.Version > Version {
		return nil, fmt.Errorf("%s is save format v%d; this build reads v%d-v%d",
			filepath.Base(path), f.Version, minVersion, Version)
	}
	if f.Player == nil {
		return nil, fmt.Errorf("%s has no character in it", filepath.Base(path))
	}
	return &f, nil
}

// Load reads a named slot under root.
func Load(root, slot string) (*File, error) { return Read(Path(root, slot)) }

// describe returns the load-menu line for a save, deriving one from the
// character when the file carries no summary of its own — which a hand-written
// test fixture generally will not.
func (f *File) describe() string {
	if f.Summary != "" {
		return f.Summary
	}
	if f.Player == nil {
		return "an empty save"
	}
	return fmt.Sprintf("%s - %s L%d", f.Player.Name, f.Player.Class, f.Player.Level)
}

// Slot is one entry in the load menu.
type Slot struct {
	Name    string
	Path    string
	Summary string
	Saved   time.Time
}

// List returns the saves under root, newest first. Unreadable files are
// skipped rather than reported: a corrupt save should not make the load menu
// itself unusable.
func List(root string) []Slot {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		return nil
	}
	var out []Slot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(root, Dir, e.Name())
		f, err := Read(path)
		if err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, f.Saved)
		out = append(out, Slot{
			Name:    strings.TrimSuffix(e.Name(), ".json"),
			Path:    path,
			Summary: f.describe(),
			Saved:   t,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Saved.After(out[j].Saved) })
	return out
}

// Delete removes a slot.
func Delete(root, slot string) error { return os.Remove(Path(root, slot)) }
