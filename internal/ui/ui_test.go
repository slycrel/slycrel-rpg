package ui_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// TestSelectRefusesRowsTheNextKeypressWouldDecline.
//
// Select exists so a menu can open on what was chosen last time, and the whole
// value of it is in what it will not do. Parking the cursor on a technique the
// player can no longer pay for would mean the first thing they press is
// refused — which is the opposite of the courtesy the greying-out is for, and
// worse than simply opening where a fresh menu would have.
func TestSelectRefusesRowsTheNextKeypressWouldDecline(t *testing.T) {
	var m ui.Menu
	m.SetItems([]ui.MenuItem{
		{Label: "first"},
		{Label: "broke", Disabled: true},
		{Label: "a heading", Header: true},
		{Label: "last"},
	})

	if !m.Select(3) {
		t.Fatal("selecting an ordinary row was refused")
	}
	if m.Index != 3 {
		t.Fatalf("the cursor is on row %d, want 3", m.Index)
	}

	for _, c := range []struct {
		row  int
		what string
	}{
		{1, "a disabled row"},
		{2, "a heading"},
		{-1, "before the list"},
		{9, "past the end"},
	} {
		if m.Select(c.row) {
			t.Errorf("selecting %s was allowed", c.what)
		}
		if m.Index != 3 {
			t.Errorf("selecting %s moved the cursor to %d", c.what, m.Index)
		}
	}
}

// TestSelectScrollsTheRowIntoView. A menu with a window on it can remember a
// choice that is below the fold, and a cursor parked on a row nobody can see is
// the same as no cursor at all.
func TestSelectScrollsTheRowIntoView(t *testing.T) {
	var m ui.Menu
	items := make([]ui.MenuItem, 8)
	for i := range items {
		items[i] = ui.MenuItem{Label: "row"}
	}
	m.SetItems(items)
	m.Visible = 3

	if !m.Select(7) {
		t.Fatal("selecting the last row was refused")
	}
	// Whatever the window did, the selected row has to be inside it.
	if got := m.Window(); m.Index < got || m.Index >= got+m.Visible {
		t.Errorf("row %d sits outside the window starting at %d, %d rows tall",
			m.Index, got, m.Visible)
	}
}
