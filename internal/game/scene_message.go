package game

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// messageScene is the modal box used for signs, dialogue, chests, and any
// "here is what just happened, press a key" moment. It draws the scene beneath
// it so the world stays visible behind the box.
type messageScene struct {
	under   Scene
	speaker string
	body    []string
	// choices, when non-empty, turns the box into a prompt. onChoose receives
	// the selected index; -1 means the player backed out.
	choices  []string
	menu     ui.Menu
	onChoose func(g *Game, choice int)
	// portrait and role turn the box into a conversation rather than a
	// notice. Empty for a sign, a chest, or the game telling you something:
	// those have no face, and drawing an empty frame beside them would be
	// worse than the plain box they get now.
	portrait string
	role     string

	// pages is body cut into screenfuls, page is which one is up, and tall is
	// the longest of them.
	//
	// Everything that used to be true of one box is now true of one page: the
	// panel is sized off the *whole* of the longest page rather than off what
	// has arrived, because a box that grew as it filled would move the words
	// already in it. Sizing to the longest rather than to the current one is
	// the same rule applied across the page turn — a panel that resized between
	// page two and page three would move the text for the same reason.
	pages [][]string
	page  int
	tall  int

	// typed is the current page arriving a character at a time.
	typed *ui.Typewriter
	// rate is kept so that turning the page can start another one. A message
	// box outlives the frame it was pushed on and the pace setting is read
	// once, at the top of the conversation, so that changing it mid-sentence
	// cannot make one paragraph of a speech arrive faster than the last.
	rate float64
}

// Long text, and the boxes that hold it.
//
// A message box has never had a limit, because nothing that used one had ever
// been long: a sign is two lines, a chest is three, and a companion's beat was
// written to the size of the box it was going to be read in. That is the
// constraint being lifted — the writing should decide how long a speech is —
// and the first thing that falls out of lifting it is that both layouts
// overflow. The conversation panel clamps its own height and then draws the
// text past the bottom of it; the notice panel computes its top edge from its
// height and puts it off the top of the screen.
//
// So the body is cut into pages that fit, and the chevron that always meant
// "press to go on" now sometimes means the next page and sometimes the end of
// it. Which one is the only thing the player has to be told, and the counter in
// the corner tells them.
const (
	// plainMaxH stops the notice box short of the top of the screen.
	plainMaxH = render.ScreenH - 30
	// The insets each layout puts around its text, which is what turns a height
	// into a number of rows. Kept beside the two places that add them back on.
	plainInset = 26
	talkInset  = 24
	// pageLookback is how far up a page will hunt for a paragraph break to end
	// on instead of filling.
	//
	// Four rows. A page that stops where the gap is reads as deliberate; a page
	// that stops halfway up because that was the nearest gap reads as a
	// mistake, and the two are the same rule with a different number in it.
	pageLookback = 4
)

// rows is how many lines of body this box can show at once.
func (m *messageScene) rows() int {
	avail, inset := float64(plainMaxH), float64(plainInset)
	if m.portrait != "" {
		avail, inset = talkMaxH, talkInset
	}
	avail -= inset
	if len(m.choices) > 0 {
		// The choices are only drawn on the last page, but the box is sized for
		// them on every page: a panel that grew when the question arrived would
		// move the answer the player is reading it for.
		avail -= m.menu.Height() + 10
	}
	if n := int(avail / render.LineH); n > 0 {
		return n
	}
	return 1
}

// paginate cuts already-wrapped lines into pages of at most rows lines.
//
// It ends a page on a paragraph break when there is one within pageLookback of
// the bottom, because the blank line between two paragraphs is where a page
// wants to turn and breaking one line past it leaves an orphan at the top of
// the next. A page never begins with a blank line and never ends with one:
// a leading gap is a line the reader has lost, and a trailing one makes the
// chevron float away from the words it belongs to.
func paginate(lines []string, rows int) [][]string {
	if rows < 1 {
		rows = 1
	}
	var pages [][]string
	for len(lines) > 0 {
		for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
			lines = lines[1:]
		}
		if len(lines) == 0 {
			break
		}
		n := rows
		if n >= len(lines) {
			n = len(lines)
		} else {
			for i := n; i > rows-pageLookback && i > 1; i-- {
				if strings.TrimSpace(lines[i-1]) == "" {
					n = i - 1
					break
				}
			}
		}
		page := lines[:n]
		for len(page) > 0 && strings.TrimSpace(page[len(page)-1]) == "" {
			page = page[:len(page)-1]
		}
		if len(page) > 0 {
			pages = append(pages, page)
		}
		lines = lines[n:]
	}
	if len(pages) == 0 {
		// A box with nothing in it is still a box. Somebody pushed it, and
		// returning no pages would leave the typewriter with nothing to read.
		pages = [][]string{{}}
	}
	return pages
}

// more reports whether there is another page after this one.
func (m *messageScene) more() bool { return m.page < len(m.pages)-1 }

// turn moves to the next page and starts typing it.
func (m *messageScene) turn(g *Game) {
	m.page++
	m.typed = ui.NewTypewriter(m.pages[m.page], m.rate)
	g.Sound.Play("ui/page")
}

// say builds the scene both constructors need, with the body already wrapped
// to whatever width the layout it is going into allows.
//
// It exists so that no path can add a message box and forget the typewriter.
// There were five of them and the effect had to be on all five: a game where
// the signpost types and the shopkeeper does not is not a game with an effect,
// it is a game with a bug somebody will report as one.
func (g *Game) say(m *messageScene) *messageScene {
	m.rate = g.typeRate()
	m.pages = paginate(m.body, m.rows())
	for _, p := range m.pages {
		if len(p) > m.tall {
			m.tall = len(p)
		}
	}
	m.typed = ui.NewTypewriter(m.pages[0], m.rate)
	return m
}

// Say pushes a plain message box over the current scene.
func (g *Game) Say(speaker, body string) {
	g.Push(g.say(&messageScene{
		under:   g.Top(),
		speaker: speaker,
		body:    render.Wrap(body, render.ScreenW-56),
	}))
}

// SayAs is Say for a person: it carries a face and, when there is one, a word
// for what they are.
//
// The wrapping width is narrower than Say's because the portrait takes the left
// third of the panel. Getting that wrong is not subtle — the text runs under
// the frame and out the other side, since a panel does not clip what you draw
// in it.
func (g *Game) SayAs(name, role, face, body string) {
	g.Push(g.say(&messageScene{
		under:    g.Top(),
		speaker:  name,
		role:     role,
		portrait: face,
		body:     render.Wrap(body, talkTextW),
	}))
}

// AskAs is AskMenu for a person, with the same face and the same width.
func (g *Game) AskAs(name, role, face, body string, items []ui.MenuItem, onChoose func(*Game, int)) {
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	m := &messageScene{
		under:    g.Top(),
		speaker:  name,
		role:     role,
		portrait: face,
		body:     render.Wrap(body, talkTextW),
		choices:  labels,
		onChoose: onChoose,
	}
	m.menu.SetItems(items)
	g.Push(g.say(m))
}

// SayThen pushes a plain message box and runs then once it is dismissed.
//
// It exists so that one event can produce two boxes in the order they happened:
// a chest reports what was in it, and only then asks about the thing with a
// name on it. Pushing both at once would show them back to front.
func (g *Game) SayThen(speaker, body string, then func(*Game)) {
	g.Push(g.say(&messageScene{
		under:    g.Top(),
		speaker:  speaker,
		body:     render.Wrap(body, render.ScreenW-56),
		onChoose: func(g *Game, _ int) { then(g) },
	}))
}

// SayAsThen is SayThen for a person: the same face and width as SayAs, and the
// same guarantee about order.
func (g *Game) SayAsThen(name, role, face, body string, then func(*Game)) {
	g.Push(g.say(&messageScene{
		under:    g.Top(),
		speaker:  name,
		role:     role,
		portrait: face,
		body:     render.Wrap(body, talkTextW),
		onChoose: func(g *Game, _ int) { then(g) },
	}))
}

// Ask pushes a message box with choices. onChoose fires after the box closes.
func (g *Game) Ask(speaker, body string, choices []string, onChoose func(*Game, int)) {
	items := make([]ui.MenuItem, len(choices))
	for i, c := range choices {
		items[i] = ui.MenuItem{Label: c}
	}
	g.AskMenu(speaker, body, items, onChoose)
}

// AskMenu is Ask with the rows built by the caller, so a choice can carry a
// price in its detail column or be greyed out entirely.
//
// Ask only took strings, which meant a box could offer a thing and then refuse
// it: the ending of a backstory quoted a price, let you select it, and only
// then said you could not afford it. A menu that can say no in advance is the
// difference between a choice and a trick, and the same gap is why a section
// header in a list has to be a disabled row with dashes around it.
//
// onChoose receives the row index, which counts disabled rows too, so it lines
// up with what the caller passed in.
func (g *Game) AskMenu(speaker, body string, items []ui.MenuItem, onChoose func(*Game, int)) {
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	m := &messageScene{
		under:    g.Top(),
		speaker:  speaker,
		body:     render.Wrap(body, render.ScreenW-56),
		choices:  labels,
		onChoose: onChoose,
	}
	m.menu.SetItems(items)
	g.Push(g.say(m))
}

func (m *messageScene) Update(g *Game) error {
	// The words arrive first, and the first press buys the rest of them rather
	// than the box.
	//
	// One press doing one thing is the whole rule. A key that dismissed a box
	// mid-sentence would make the effect a hazard — the player who presses on
	// instinct loses the line and has no way back to it — and a key that did
	// nothing at all while text arrived would make it a wait. So it completes
	// the text, and the *next* press does what it would always have done.
	//
	// This is also why the choices are not navigable yet: a menu that can be
	// answered before the question has finished being asked is a menu that can
	// be answered without it.
	if !m.typed.Done() {
		m.typed.Tick()
		if g.Dismiss() || g.Accept() || g.Back() {
			m.typed.Finish()
		}
		return nil
	}

	// A page that is not the last one goes on, whatever was pressed.
	//
	// Including cancel, and deliberately: while there is more to read the only
	// thing a key can mean is "go on", and a player who taps back halfway
	// through a companion's story to see the rest of it would otherwise have
	// walked out of the conversation. Backing out is an answer, so it waits
	// until there is a question.
	if m.more() {
		if g.Dismiss() || g.Accept() || g.Back() {
			m.turn(g)
		}
		return nil
	}

	if len(m.choices) == 0 {
		// Anything at all closes a box that is only reporting. See Keystroke.
		if g.Dismiss() {
			g.Pop()
			if m.onChoose != nil {
				m.onChoose(g, 0)
			}
		}
		return nil
	}

	g.MenuNav(&m.menu)
	if g.Accept() {
		// A disabled row is an answer the caller has already said no to; taking
		// it would put the refusal after the decision, which is the thing
		// AskMenu exists to stop.
		if it, ok := m.menu.Selected(); ok && it.Disabled {
			g.Sound.Play("ui/deny")
			return nil
		}
		i := m.menu.Index
		g.Pop()
		if m.onChoose != nil {
			m.onChoose(g, i)
		}
	} else if g.Back() {
		g.Pop()
		if m.onChoose != nil {
			m.onChoose(g, -1)
		}
	}
	return nil
}

func (m *messageScene) Draw(g *Game, dst *ebiten.Image) {
	if m.under != nil {
		m.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, render.ColShadow)

	// A face means a person, and a person gets the big layout.
	if m.portrait != "" {
		m.drawTalk(g, dst)
		return
	}

	h := float64(m.tall)*render.LineH + plainInset
	if len(m.choices) > 0 {
		h += m.menu.Height() + 6
	}
	if h < 54 {
		h = 54
	}
	y := render.ScreenH - h - 14
	ui.TitledPanel(dst, m.speaker, 16, y, render.ScreenW-32, h)

	// The panel was measured against the whole text and the text is drawn as
	// far as it has got. Visible returns a row per row either way, so ty walks
	// the same distance on the first frame as on the last and nothing moves.
	ty := y + 10
	for _, ln := range m.typed.Visible() {
		render.Text(dst, ln, 26, ty, render.ColInk)
		ty += render.LineH
	}
	// Neither the choice nor the "there is more" chevron appears until the
	// sentence has finished: one is an answer to a half-asked question and the
	// other is an invitation to leave in the middle of one.
	if !m.typed.Done() {
		return
	}
	if len(m.choices) > 0 && !m.more() {
		m.menu.Draw(dst, 34, ty+4, render.ScreenW-80)
	} else if (g.Tick()/24)%2 == 0 {
		render.TextRight(dst, "v", render.ScreenW-28, y+h-16, render.ColGold)
	}
	m.drawCount(dst, render.ScreenW-28, y+8)
}

// drawCount is "2/4" in the corner of a box with more than one page in it.
//
// The chevron already says "press to go on" and has always said it; what it
// cannot say is whether going on is another paragraph or the end of the
// conversation. One box in the game used to mean one thing, and now that a
// speech can run to four screens the difference is the whole of what a reader
// needs to know before they decide to keep reading.
func (m *messageScene) drawCount(dst *ebiten.Image, x, y float64) {
	if len(m.pages) < 2 {
		return
	}
	render.TextRight(dst, fmt.Sprintf("%d/%d", m.page+1, len(m.pages)), x, y, render.ColInkFaint)
}

// The conversation layout.
//
// A person talking gets most of the screen, because the alternative — the
// bottom strip every message shared — gave a quest-giver exactly the same
// presence as a signpost. The panel stops just above the status bar rather than
// covering it: the strip says where you are and what you are carrying, and both
// are things you want while deciding whether to take a job.
const (
	talkX = 12
	talkY = 16
	talkW = render.ScreenW - 2*talkX
	// talkMaxH ends above the HUD rather than over it: the strip says where you
	// are and what you are carrying, and both are things you want in hand while
	// deciding whether to take a job.
	talkMaxH = render.ScreenH - hudH - talkY - 6

	// The portrait pane. 76 is as large as the frame goes without the text
	// column dropping below the width a sentence needs to not read as poetry.
	faceSize = 76
	facePad  = 12

	// Text starts to the right of the portrait and wraps well short of the
	// panel edge, since ui.Panel draws a border this must not run into.
	talkTextX = talkX + facePad + faceSize + 12
	talkTextW = talkW - (talkTextX - talkX) - 18

	// The caption's own column, which is wider than the portrait it sits under
	// and shorter than the text column it sits beside.
	//
	// Wrapping it to the frame was the obvious thing and the wrong one: at 76px
	// the word "apothecary," is 77 and overflows on its own, so a whole class of
	// caption could never fit however short the rest of it was. The left column
	// runs to where the body text starts and nothing else is drawn in it, so the
	// caption may have all of it.
	captionW  = talkTextX - talkX - facePad - 6
	roleLines = 3
)

// drawTalk renders the conversation layout: a face on the left, what they said
// on the right, and the choice underneath.
// talkHeight is the panel's height: enough for the face, enough for the words,
// and no more.
//
// Fixed height was tried first and looked worse than the strip it replaced. A
// two-line exchange in a panel sized for eight is a face at the top, a menu
// pinned to the bottom, and a hundred pixels of nothing between them — the
// screen was being used rather than filled. So it grows with the content and
// stops at the face's own height, which is the floor a conversation cannot go
// below anyway.
func (m *messageScene) talkHeight() float64 {
	text := float64(m.tall)*render.LineH + talkInset
	if len(m.choices) > 0 {
		text += m.menu.Height() + 10
	}
	face := float64(faceSize) + 30
	if m.role != "" {
		n := len(render.Wrap(m.role, captionW))
		if n > roleLines {
			n = roleLines
		}
		face += float64(n) * render.LineH
	}
	h := text
	if face > h {
		h = face
	}
	if h > talkMaxH {
		h = talkMaxH
	}
	return h
}

func (m *messageScene) drawTalk(g *Game, dst *ebiten.Image) {
	h := m.talkHeight()
	ui.TitledPanel(dst, m.speaker, talkX, talkY, talkW, h)

	fx, fy := float64(talkX+facePad), float64(talkY+18)
	ui.Slot(dst, fx, fy, faceSize, faceSize, render.ColInkDim)
	if sp := g.Assets.Get(m.portrait); sp != nil {
		render.ScreenFit(dst, sp, 0, fx+2, fy+2, faceSize-4, faceSize-4, nil)
	}

	// What they are, under the face. A word rather than a sentence: the name is
	// already the title, and this is the thing the player is placing them by.
	//
	// Wrapped to the portrait's width over two lines rather than truncated to
	// one. "undead mage" is eleven characters against a box that fits ten, and
	// cutting it produced "undead ma." — which is not a shorter way of saying
	// it, it is a different thing that happens to start the same.
	ry := fy + faceSize + 4
	for i, ln := range render.Wrap(m.role, captionW) {
		if i >= roleLines {
			break
		}
		render.Text(dst, ln, fx, ry, render.ColInkDim)
		ry += render.LineH
	}

	ty := float64(talkY + 20)
	for _, ln := range m.typed.Visible() {
		render.Text(dst, ln, talkTextX, ty, render.ColInk)
		ty += render.LineH
	}

	if !m.typed.Done() {
		return
	}
	// The choice sits under the text, not under the portrait, so the eye goes
	// down one column instead of crossing back.
	if len(m.choices) > 0 && !m.more() {
		m.menu.Draw(dst, talkTextX+8, ty+6, talkW-(talkTextX-talkX)-26)
	} else if (g.Tick()/24)%2 == 0 {
		render.TextRight(dst, "v", talkX+talkW-12, talkY+h-16, render.ColGold)
	}
	m.drawCount(dst, talkX+talkW-12, talkY+6)
}
