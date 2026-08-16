package game

import "testing"

// TestDecorPlacementIsDeterministic is the property the whole scheme rests on:
// scenery is never stored, so walking away and back must reproduce it exactly,
// and a different seed must produce a different world.
func TestDecorPlacementIsDeterministic(t *testing.T) {
	const seed = 1994
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if unitHash(x, y, seed, 3) != unitHash(x, y, seed, 3) {
				t.Fatalf("unitHash is not stable at (%d,%d)", x, y)
			}
		}
	}

	same := 0
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			if unitHash(x, y, seed, 3) == unitHash(x, y, seed+1, 3) {
				same++
			}
		}
	}
	if same > 4 {
		t.Errorf("%d of 3600 cells scatter identically across seeds; the seed is barely mixed in", same)
	}
}

// TestDecorHashIsWellSpread guards against the failure that would show up as
// scenery combed into diagonal stripes: a hash whose low bits track position.
func TestDecorHashIsWellSpread(t *testing.T) {
	const buckets = 10
	var hist [buckets]int
	n := 0
	for y := 0; y < 120; y++ {
		for x := 0; x < 160; x++ {
			hist[int(unitHash(x, y, 7, 0)*buckets)]++
			n++
		}
	}
	want := n / buckets
	for i, got := range hist {
		if got < want*80/100 || got > want*120/100 {
			t.Errorf("bucket %d holds %d of %d, expected near %d: distribution is skewed",
				i, got, n, want)
		}
	}

	// Neighbouring cells must not correlate, or props line up in rows.
	adjacent := 0
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			a := unitHash(x, y, 7, 0) < 0.2
			b := unitHash(x+1, y, 7, 0) < 0.2
			if a && b {
				adjacent++
			}
		}
	}
	// At p=0.2 independent, expect ~4% of pairs; allow generous slack.
	if adjacent > 100*100*8/100 {
		t.Errorf("%d adjacent pairs both placed; neighbours are correlated", adjacent)
	}
}

// TestEveryDecoratedTerrainResolves keeps the frame tables honest: a sheet key
// that does not exist, or an empty frame list, would silently draw nothing.
func TestEveryDecoratedTerrainResolves(t *testing.T) {
	for terrain, sets := range terrainDecor {
		if len(sets) == 0 {
			t.Errorf("terrain %v has an empty decor list", terrain)
		}
		for i, ds := range sets {
			if ds.Sheet == "" {
				t.Errorf("terrain %v set %d has no sheet", terrain, i)
			}
			if len(ds.Frames) == 0 {
				t.Errorf("terrain %v set %d lists no frames", terrain, i)
			}
			if ds.Chance <= 0 || ds.Chance > 1 {
				t.Errorf("terrain %v set %d has chance %v, outside (0,1]", terrain, i, ds.Chance)
			}
			for _, f := range ds.Frames {
				if f < 0 {
					t.Errorf("terrain %v set %d has negative frame %d", terrain, i, f)
				}
			}
		}
	}
}
