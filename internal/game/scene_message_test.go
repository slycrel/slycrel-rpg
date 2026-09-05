package game

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// Nothing is lost between the wrap and the page.
//
// Cutting text into screenfuls is the sort of thing that reads as obviously
// correct and drops the last line of every page, or the first, or repeats one
// across a break. So the property checked here is the boring one: the pages,
// laid end to end, are the text.
func TestPagingLosesNothing(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		rows  int
	}{
		{"one short page", []string{"a", "b"}, 8},
		{"exactly a page", []string{"a", "b", "c", "d"}, 4},
		{"one over", []string{"a", "b", "c", "d", "e"}, 4},
		{"many pages", strings.Split("a b c d e f g h i j k l m n o p", " "), 3},
		{"paragraphs", []string{"a", "", "b", "c", "", "d", "e", "f"}, 4},
		{"a gap at the top", []string{"", "", "a", "b"}, 4},
		{"a gap at the bottom", []string{"a", "b", "", ""}, 4},
		{"nothing at all", nil, 6},
		{"one row a page", strings.Split("a b c d", " "), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pages := paginate(tc.lines, tc.rows)
			if len(pages) == 0 {
				t.Fatal("no pages at all; the typewriter has nothing to read")
			}
			var got []string
			for i, p := range pages {
				if len(p) > tc.rows {
					t.Errorf("page %d is %d rows in a box that holds %d", i, len(p), tc.rows)
				}
				if len(p) > 0 {
					if strings.TrimSpace(p[0]) == "" {
						t.Errorf("page %d opens on a blank row", i)
					}
					if strings.TrimSpace(p[len(p)-1]) == "" {
						t.Errorf("page %d ends on a blank row", i)
					}
				}
				got = append(got, p...)
			}
			var want []string
			for _, ln := range tc.lines {
				if strings.TrimSpace(ln) != "" {
					want = append(want, ln)
				}
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("the pages read %q, the text was %q",
					strings.Join(got, "|"), strings.Join(want, "|"))
			}
		})
	}
}

// A page turns where the paragraph does, when it can.
func TestAPageEndsOnAParagraphWhenOneIsNear(t *testing.T) {
	// Six rows to a page, and a gap one row up from the bottom.
	lines := []string{"a", "b", "c", "d", "", "e", "f", "g", "h"}
	pages := paginate(lines, 6)
	if len(pages) < 2 {
		t.Fatalf("wanted more than one page, got %d", len(pages))
	}
	if last := pages[0][len(pages[0])-1]; last != "d" {
		t.Errorf("the first page ends on %q, not on the paragraph break at %q", last, "d")
	}

	// And does not hunt so far back that the page comes out half empty: the
	// gap here is five rows up, past pageLookback, so the page fills instead.
	lines = []string{"a", "", "b", "c", "d", "e", "f", "g"}
	pages = paginate(lines, 6)
	if got := len(pages[0]); got != 6 {
		t.Errorf("the first page is %d rows; a gap five rows up should not have "+
			"shortened it, since a page that stops halfway reads as a mistake", got)
	}
}

// A box never draws past its own frame.
//
// Both layouts compute their height from the number of lines, and neither of
// them used to have a limit — the conversation panel clamped its height and
// then drew the text past the bottom of it anyway, and the notice panel put its
// own top edge off the screen. Nothing complained, because nothing had ever
// been long enough. The writing is meant to get longer, so this is the guard
// that says how much longer it may get before it has to turn a page.
func TestNoBoxDrawsPastItsOwnFrame(t *testing.T) {
	long := strings.Repeat("The letters keep arriving and nobody delivers them. ", 60)
	for _, tc := range []struct {
		name  string
		build func(m *messageScene)
		maxH  float64
	}{
		{"a notice", func(m *messageScene) {
			m.body = render.Wrap(long, render.ScreenW-56)
		}, plainMaxH},
		{"a notice with a choice", func(m *messageScene) {
			m.body = render.Wrap(long, render.ScreenW-56)
			m.choices = []string{"Yes", "No"}
			m.menu.SetItems([]ui.MenuItem{{Label: "Yes"}, {Label: "No"}})
		}, plainMaxH},
		{"a conversation", func(m *messageScene) {
			m.portrait = "face/x"
			m.body = render.Wrap(long, talkTextW)
		}, talkMaxH},
		{"a conversation with a choice", func(m *messageScene) {
			m.portrait = "face/x"
			m.body = render.Wrap(long, talkTextW)
			m.choices = []string{"Burn it", "Sell it", "Walk away"}
			m.menu.SetItems([]ui.MenuItem{{Label: "Burn it"}, {Label: "Sell it"}, {Label: "Walk away"}})
		}, talkMaxH},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &Game{}
			m := &messageScene{}
			tc.build(m)
			g.say(m)

			// The height every page is drawn at, against the room there is.
			// Checked before the "is the fixture long enough" guard below, so
			// that a build with no paging in it fails on the overflow rather
			// than on the sanity check that would have caught it second.
			// What the contents want, not what the panel was clamped to.
			// The clamped number is at most the maximum by construction, so a
			// test that read it would report a pass while the text ran out of
			// the bottom of the frame — which is the bug itself.
			h := float64(m.tall)*render.LineH + plainInset
			if len(m.choices) > 0 {
				h += m.menu.Height() + 6
			}
			if m.portrait != "" {
				h = m.talkNeeds()
			}
			if h > tc.maxH {
				t.Errorf("the box is %.0f tall in %.0f of room", h, tc.maxH)
			}
			// And the text inside it fits the frame it is drawn in.
			if m.tall > m.rows() {
				t.Errorf("the tallest page is %d rows in a box that shows %d",
					m.tall, m.rows())
			}
			if len(m.pages) < 2 {
				t.Errorf("%d pages out of %d lines: the fixture was meant to be "+
					"long enough to need turning", len(m.pages), len(m.body))
			}
		})
	}
}
