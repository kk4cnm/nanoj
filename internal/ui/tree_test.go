package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
)

func parse(t *testing.T, s string) *document.Node {
	t.Helper()
	n, err := document.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("parse(%s): %v", s, err)
	}
	return n
}

// TestFlattenExpandedShowsAll confirms a fully expanded tree exposes every node.
func TestFlattenExpandedShowsAll(t *testing.T) {
	root := parse(t, `{"a": 1, "b": [10, 20], "c": {"d": true}}`)
	rows := Flatten(root, nil) // nil = nothing collapsed
	// root, a, b, b[0], b[1], c, c.d = 7 rows
	if len(rows) != 7 {
		t.Fatalf("expected 7 rows, got %d", len(rows))
	}
	if rows[0].Depth != 0 || !rows[0].Node.IsContainer() {
		t.Errorf("row 0 should be the root container at depth 0")
	}
	// The two array elements should carry their indices and have no key.
	if rows[3].HasKey || rows[3].Index != 0 {
		t.Errorf("row 3 should be array element index 0, got %+v", rows[3])
	}
	if rows[4].Index != 1 {
		t.Errorf("row 4 should be array element index 1, got %+v", rows[4])
	}
}

// TestFlattenCollapsedHidesChildren confirms collapsing a container removes its
// descendants from the visible rows.
func TestFlattenCollapsedHidesChildren(t *testing.T) {
	root := parse(t, `{"a": 1, "b": [10, 20]}`)
	// Find the path of the "b" array from a full flatten, then collapse it.
	full := Flatten(root, nil)
	var bPath string
	for _, r := range full {
		if r.HasKey && r.Key == "b" {
			bPath = r.Path
		}
	}
	if bPath == "" {
		t.Fatal("could not locate path for key b")
	}

	rows := Flatten(root, map[string]bool{bPath: true})
	// root, a, b (collapsed) = 3 rows; the two array elements are hidden.
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows after collapse, got %d", len(rows))
	}
}

// TestNavigationDownStopsAtEnd confirms moving past the last row clamps.
func TestNavigationDownStopsAtEnd(t *testing.T) {
	m := New(parse(t, `{"a": 1, "b": 2}`), "test.json")
	// root, a, b = 3 rows, indices 0..2.
	for i := 0; i < 10; i++ {
		m = sendKey(m, "down")
	}
	if m.cursor != 2 {
		t.Errorf("cursor should clamp at last row 2, got %d", m.cursor)
	}
}

// TestCollapseThenExpand confirms the toggle round-trips the visible rows.
func TestCollapseThenExpand(t *testing.T) {
	m := New(parse(t, `{"a": [1, 2, 3]}`), "test.json")
	before := len(m.rows) // root, a, three elements = 5
	if before != 5 {
		t.Fatalf("expected 5 rows initially, got %d", before)
	}
	m = sendKey(m, "down")  // move onto "a"
	m = sendKey(m, "enter") // collapse it
	if len(m.rows) != 2 {
		t.Fatalf("expected 2 rows after collapse, got %d", len(m.rows))
	}
	m = sendKey(m, "enter") // expand again
	if len(m.rows) != before {
		t.Errorf("expected %d rows after re-expand, got %d", before, len(m.rows))
	}
}

// TestLeftArrowJumpsToParent confirms left on a leaf moves to its parent row.
func TestLeftArrowJumpsToParent(t *testing.T) {
	m := New(parse(t, `{"a": {"b": 1}}`), "test.json")
	// rows: 0 root, 1 a(obj), 2 b(leaf)
	m = sendKey(m, "down") // a
	m = sendKey(m, "down") // b
	if m.cursor != 2 {
		t.Fatalf("setup: expected cursor 2, got %d", m.cursor)
	}
	m = sendKey(m, "left") // b is a leaf -> jump to parent "a" at row 1
	if m.cursor != 1 {
		t.Errorf("expected cursor to jump to parent row 1, got %d", m.cursor)
	}
}

func sendKey(m Model, key string) Model {
	var msg tea.Msg
	switch key {
	case "up", "down", "left", "right", "enter", "home", "end":
		msg = keyMsgFor(key)
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func keyMsgFor(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	}
	return tea.KeyMsg{}
}
