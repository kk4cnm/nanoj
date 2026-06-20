package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestUndoRedoEdit confirms an edit can be undone and redone.
func TestUndoRedoEdit(t *testing.T) {
	m := New(parse(t, `{"n": 1}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down")  // onto "n"
	m = sendKey(m, "enter") // edit prompt (prefilled "1")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	m = typeText(m, "2")
	m = sendKey(m, "enter") // commit -> value is 2, dirty

	if m.root.Members[0].Value.Num.String() != "2" {
		t.Fatalf("setup: expected 2, got %s", m.root.Members[0].Value.Num)
	}

	// Undo -> back to 1, and dirty should reflect the pre-edit state (clean).
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u"), Alt: true})
	m = updated.(Model)
	if m.root.Members[0].Value.Num.String() != "1" {
		t.Errorf("after undo expected 1, got %s", m.root.Members[0].Value.Num)
	}
	if m.dirty {
		t.Errorf("after undoing the only edit, model should be clean")
	}

	// Redo -> back to 2.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e"), Alt: true})
	m = updated.(Model)
	if m.root.Members[0].Value.Num.String() != "2" {
		t.Errorf("after redo expected 2, got %s", m.root.Members[0].Value.Num)
	}
}

// TestUndoDelete confirms a deleted node comes back on undo.
func TestUndoDelete(t *testing.T) {
	m := New(parse(t, `{"a": 1, "b": 2}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // onto "a"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(Model)
	m = typeText(m, "y") // confirm delete
	if m.root.ChildCount() != 1 {
		t.Fatalf("setup: expected 1 child after delete, got %d", m.root.ChildCount())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u"), Alt: true})
	m = updated.(Model)
	if m.root.ChildCount() != 2 || m.root.Members[0].Key != "a" {
		t.Errorf("undo should restore deleted key \"a\": %+v", m.root.Members)
	}
}

// TestUndoEmptyIsSafe confirms undo with no history just reports a message.
func TestUndoEmptyIsSafe(t *testing.T) {
	m := New(parse(t, `{"a": 1}`), "x.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u"), Alt: true})
	m = updated.(Model)
	if m.status != "nothing to undo" {
		t.Errorf("expected 'nothing to undo', got %q", m.status)
	}
}

// TestSearchFindsValueInCollapsedBranch confirms search reaches into collapsed
// containers, expanding ancestors so the match becomes visible.
func TestSearchFindsValueInCollapsedBranch(t *testing.T) {
	m := New(parse(t, `{"outer": {"inner": "needle"}}`), "x.json")
	m = sized(m, 80, 24)
	// Collapse "outer" so "inner" is hidden.
	m = sendKey(m, "down")  // onto "outer"
	m = sendKey(m, "enter") // collapse
	if len(m.rows) != 2 {
		t.Fatalf("setup: expected outer collapsed (2 rows), got %d", len(m.rows))
	}

	// Search for the hidden value.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "needle")
	m = sendKey(m, "enter")

	cur := m.rows[m.cursor]
	if !cur.HasKey || cur.Key != "inner" {
		t.Errorf("expected cursor on \"inner\" after search, got %+v", cur)
	}
	if got := scalarText(cur.Node); got != `"needle"` {
		t.Errorf("expected value \"needle\", got %s", got)
	}
}

// TestSearchKeyMatch confirms matching against object keys.
func TestSearchKeyMatch(t *testing.T) {
	m := New(parse(t, `{"alpha": 1, "beta": 2, "gamma": 3}`), "x.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "gam")
	m = sendKey(m, "enter")
	if m.rows[m.cursor].Key != "gamma" {
		t.Errorf("expected to land on \"gamma\", got %q", m.rows[m.cursor].Key)
	}
}

// TestSearchNotFound reports a message and leaves the cursor put.
func TestSearchNotFound(t *testing.T) {
	m := New(parse(t, `{"a": 1}`), "x.json")
	m = sized(m, 80, 24)
	before := m.cursor
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "zzz")
	m = sendKey(m, "enter")
	if m.status != "not found: zzz" {
		t.Errorf("expected not-found status, got %q", m.status)
	}
	if m.cursor != before {
		t.Errorf("cursor should not move on failed search")
	}
}
