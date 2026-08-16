package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// audioEntry is one game sound. A key can map to several files; the mixer
// picks one at random each time it plays, which is the difference between a
// sword that lands and a sword that clicks the same way forty times a minute.
type audioEntry struct {
	Key   string   `json:"key"`
	Files []string `json:"files"`
	// Loop marks ambience beds, which stream rather than being held as PCM.
	Loop bool `json:"loop,omitempty"`
}

// pick describes how to find a sound: which extracted pack to search, and the
// substrings a filename must contain. Matching on substrings rather than exact
// paths survives the packs' inconsistent folder naming, which embeds things
// like "WAV - 44100 Hz - 16 Bit" in the path.
type pick struct {
	key      string
	pack     string
	contains []string
	// exclude drops false positives, e.g. "_Loop" variants of a one-shot.
	exclude []string
	loop    bool
	limit   int // keep at most this many variants; 0 means all
}

const (
	packCombat = "combatsoundsbundlecollection"
	packUI     = "userinterfacesfxbundle"
	packAmb    = "ambiencesoundspack"
	packMon    = "monstersoundsvolume1"
	packVO     = "oldmagicianvoicepack"
)

// soundPicks is the curated mapping from game event to source audio. Adding a
// sound to the game is a line here plus a Play call at the site.
var soundPicks = []pick{
	// --- interface -------------------------------------------------------
	{key: "ui/move", pack: packUI, contains: []string{"Click_Minimalistic"}, limit: 4},
	{key: "ui/confirm", pack: packUI, contains: []string{"Click_Fantasy_RPG"}, limit: 4},
	{key: "ui/cancel", pack: packUI, contains: []string{"Click_Negative"}, limit: 3},
	{key: "ui/deny", pack: packUI, contains: []string{"Click_Negative"}, limit: 3},
	{key: "ui/page", pack: packUI, contains: []string{"Pass_Page"}, limit: 3},

	// --- world -----------------------------------------------------------
	{key: "world/enter", pack: packUI, contains: []string{"Wood_Interaction"}, limit: 3},
	{key: "world/chest", pack: packUI, contains: []string{"Chest_Open"}, limit: 2},
	{key: "world/coins", pack: packUI, contains: []string{"A_Lot_Of_Coins"}, limit: 3},
	{key: "world/loot", pack: packUI, contains: []string{"Bag_Interaction_Soft"}, limit: 3},
	{key: "world/equip", pack: packUI, contains: []string{"Change_Weapon"}, limit: 3},
	{key: "world/buy", pack: packUI, contains: []string{"Coins", "01"}, limit: 2},
	{key: "world/rest", pack: packUI, contains: []string{"Book_Interaction"}, limit: 2},

	// --- combat ----------------------------------------------------------
	{key: "fight/start", pack: packCombat, contains: []string{"Anime_Clash"}, exclude: []string{"Loop"}, limit: 3},
	{key: "fight/hit", pack: packCombat, contains: []string{"Anime_Combat_Hit"}, limit: 6},
	{key: "fight/crit", pack: packCombat, contains: []string{"Anime_Attack_Critical"}, limit: 3},
	{key: "fight/miss", pack: packCombat, contains: []string{"Anime_Combat_Whoosh"}, limit: 4},
	{key: "fight/hurt", pack: packCombat, contains: []string{"Anime_Armour_Impact"}, limit: 4},
	{key: "fight/spell", pack: packCombat, contains: []string{"Anime_Aggressive_Spell"}, exclude: []string{"Loop"}, limit: 4},
	{key: "fight/heal", pack: packCombat, contains: []string{"Anime_Energy_Up"}, exclude: []string{"Loop"}, limit: 3},
	{key: "fight/die", pack: packCombat, contains: []string{"Anime_Destruction"}, limit: 3},
	{key: "fight/flee", pack: packCombat, contains: []string{"Anime_Movement_Teleport"}, limit: 2},
	{key: "fight/victory", pack: packCombat, contains: []string{"Anime_Power_Up"}, limit: 2},
	{key: "fight/defeat", pack: packCombat, contains: []string{"Anime_Explosion_Aftermath"}, limit: 2},
	{key: "fight/levelup", pack: packUI, contains: []string{"UI_Reward_Obtained"}, limit: 2},
	{key: "fight/monster", pack: packMon, contains: []string{"Attack"}, exclude: []string{"Loop"}, limit: 8},

	// --- the narrator ----------------------------------------------------
	// One retired wizard, doing colour commentary on a life he is not part of.
	{key: "vo/victory", pack: packVO, contains: []string{"Victory"}, limit: 3},
	{key: "vo/death", pack: packVO, contains: []string{"Magician_Death"}, limit: 6},
	{key: "vo/greet", pack: packVO, contains: []string{"Greeting"}, limit: 3},
	{key: "vo/welcome", pack: packVO, contains: []string{"Welcome"}, limit: 3},

	// --- ambience beds ---------------------------------------------------
	{key: "amb/plains", pack: packAmb, contains: []string{"Ambience_Birds_Loop"}, loop: true, limit: 1},
	{key: "amb/forest", pack: packAmb, contains: []string{"Ambience_Forest_Loop"}, loop: true, limit: 1},
	{key: "amb/town", pack: packAmb, contains: []string{"Ambience_Crowd_02_Loop"}, loop: true, limit: 1},
	{key: "amb/dungeon", pack: packAmb, contains: []string{"Ambience_Dungeon_01_Loop"}, loop: true, limit: 1},
}

// buildAudioManifest walks the extracted sound packs and writes
// assets/audio.json. Keys that match nothing are reported and skipped: a
// missing sound is silence, never a failure.
func buildAudioManifest() error {
	// Index each pack once rather than walking it per key.
	index := map[string][]string{}
	for _, p := range []string{packCombat, packUI, packAmb, packMon, packVO} {
		var files []string
		_ = filepath.Walk(filepath.Join(rawRoot, p), func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".wav") {
				files = append(files, path)
			}
			return nil
		})
		sort.Strings(files)
		index[p] = files
	}

	var entries []audioEntry
	var missing []string

	for _, pk := range soundPicks {
		var hits []string
		for _, path := range index[pk.pack] {
			name := filepath.Base(path)
			if !containsAll(name, pk.contains) || containsAny(name, pk.exclude) {
				continue
			}
			hits = append(hits, path)
			if pk.limit > 0 && len(hits) >= pk.limit {
				break
			}
		}
		if len(hits) == 0 {
			missing = append(missing, pk.key)
			continue
		}
		entries = append(entries, audioEntry{Key: pk.key, Files: hits, Loop: pk.loop})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

	if err := os.MkdirAll("assets", 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(struct {
		Entries []audioEntry `json:"entries"`
	}{entries}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("assets", "audio.json"), out, 0o644); err != nil {
		return err
	}

	total := 0
	for _, e := range entries {
		total += len(e.Files)
	}
	fmt.Printf("wrote assets/audio.json (%d keys, %d files)\n", len(entries), total)
	for _, k := range missing {
		fmt.Fprintf(os.Stderr, "  no match for %q; that cue will be silent\n", k)
	}
	return nil
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			return false
		}
	}
	return true
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
