package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
)

func copyKey(m Model) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6"), Alt: true})
	return updated.(Model)
}

func ctrl(m Model, t tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: t})
	return updated.(Model)
}

// TestTreeCopyPasteSibling copies a value and pastes it as a sibling in the
// parent object, deduping the key.
func TestTreeCopyPasteSibling(t *testing.T) {
	m := New(parse(t, `{"a": 1, "b": 2}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // onto "a"
	m = copyKey(m)
	if m.clipboard == nil {
		t.Fatal("copy should populate the clipboard")
	}
	m = ctrl(m, tea.KeyCtrlU) // paste after "a"
	// Expect a, a_2, b  (pasted clone keyed a_2, inserted after a)
	if m.root.ChildCount() != 3 {
		t.Fatalf("expected 3 members after paste, got %d", m.root.ChildCount())
	}
	if m.root.Members[1].Key != "a_2" {
		t.Errorf("expected deduped key a_2 at index 1, got %q", m.root.Members[1].Key)
	}
	if m.root.Members[1].Value.Num.String() != "1" {
		t.Errorf("pasted value should be 1, got %s", m.root.Members[1].Value.Num)
	}
}

// TestTreePasteIntoContainer pastes as the last child of a selected array.
func TestTreePasteIntoContainer(t *testing.T) {
	m := New(parse(t, `{"items": [10, 20], "x": 99}`), "x.json")
	m = sized(m, 80, 24)
	// Copy "x" (value 99).
	m = sendKey(m, "down") // items
	m = sendKey(m, "down") // 10
	m = sendKey(m, "down") // 20
	m = sendKey(m, "down") // x
	m = copyKey(m)
	// Select the items array and paste into it.
	m = sendKey(m, "home") // root
	m = sendKey(m, "down") // items (array)
	m = ctrl(m, tea.KeyCtrlU)
	items := m.root.Members[0].Value
	if items.ChildCount() != 3 || items.Items[2].Num.String() != "99" {
		t.Errorf("expected 99 appended to items, got %+v", items.Items)
	}
}

// TestTreeCutThenPaste moves a node via cut + paste.
func TestTreeCutThenPaste(t *testing.T) {
	m := New(parse(t, `{"a": 1, "b": 2}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // a
	m = ctrl(m, tea.KeyCtrlK)
	if m.root.ChildCount() != 1 {
		t.Fatalf("cut should remove a, got %d members", m.root.ChildCount())
	}
	// Cursor now on b; paste a back after it.
	m = ctrl(m, tea.KeyCtrlU)
	if m.root.ChildCount() != 2 || m.root.Members[1].Key != "a" {
		t.Errorf("expected b,a after cut+paste, got %+v", m.root.Members)
	}
}

// TestTablePasteRowAddsRow copies a row and pastes it as a new row.
func TestTablePasteRowAddsRow(t *testing.T) {
	m := New(parse(t, `[{"id":1},{"id":2}]`), "t.json")
	m = sized(m, 80, 24)
	m = copyKey(m) // copy row 0
	m = ctrl(m, tea.KeyCtrlU)
	if len(m.table.array.Items) != 3 {
		t.Fatalf("expected 3 rows after paste, got %d", len(m.table.array.Items))
	}
	// Pasted clone of row 0 should be at index 1.
	if m.table.array.Items[1].Members[0].Value.Num.String() != "1" {
		t.Errorf("pasted row should duplicate row 0, got %+v", m.table.array.Items[1])
	}
	if m.tableRow != 1 {
		t.Errorf("cursor should be on the pasted row 1, got %d", m.tableRow)
	}
}

// TestTableCutRow removes the selected row.
func TestTableCutRow(t *testing.T) {
	m := New(parse(t, `[{"id":1},{"id":2},{"id":3}]`), "t.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // row 1 (id 2)
	m = ctrl(m, tea.KeyCtrlK)
	if len(m.table.array.Items) != 2 {
		t.Fatalf("expected 2 rows after cut, got %d", len(m.table.array.Items))
	}
	if m.table.array.Items[1].Members[0].Value.Num.String() != "3" {
		t.Errorf("expected rows id 1,3 to remain, got %+v", m.table.array.Items)
	}
}

// TestTablePasteIntroducesColumn pastes a row with an extra key and confirms the
// table grows a column.
func TestTablePasteIntroducesColumn(t *testing.T) {
	m := New(parse(t, `[{"a":1}]`), "t.json")
	m = sized(m, 80, 24)
	// Put an object with key "b" on the clipboard via the tree view.
	m = ctrl(m, tea.KeyCtrlT) // to tree
	// root is the array; first child is the object {"a":1}. Cut it to clipboard.
	m = sendKey(m, "down")    // the object element
	m = copyKey(m)            // clipboard now holds {"a":1}
	m = ctrl(m, tea.KeyCtrlT) // back to table
	// Manually craft a clipboard object with a new key to test column growth.
	obj := document.NewObject()
	_ = obj.AddMember("a", document.NewNumber("1"))
	_ = obj.AddMember("b", document.NewNumber("2"))
	m.clipboard = obj
	m = ctrl(m, tea.KeyCtrlU)
	cols := m.table.columns
	if len(cols) != 2 || cols[1] != "b" {
		t.Errorf("expected columns to grow to include b, got %v", cols)
	}
}

// TestPasteEmptyClipboard is a no-op with a message.
func TestPasteEmptyClipboard(t *testing.T) {
	m := New(parse(t, `{"a": 1}`), "x.json")
	m = sized(m, 80, 24)
	m = ctrl(m, tea.KeyCtrlU)
	if m.status != "clipboard is empty" {
		t.Errorf("expected empty-clipboard message, got %q", m.status)
	}
}
