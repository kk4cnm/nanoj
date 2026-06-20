package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
)

// colValues returns the string form of each row's value for the given column.
func colValues(m Model, key string) []string {
	var out []string
	for _, item := range m.table.array.Items {
		out = append(out, cellText(cellByKey(item, key)))
	}
	return out
}

func TestSortNumbersAscendingThenDescending(t *testing.T) {
	m := New(parse(t, `[{"n":54},{"n":36},{"n":85}]`), "t.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "s") // sort by column 0 ("n") ascending
	if got := colValues(m, "n"); !(got[0] == "36" && got[1] == "54" && got[2] == "85") {
		t.Errorf("ascending sort wrong: %v", got)
	}
	if m.sortCol != 0 || m.sortDesc {
		t.Errorf("expected sortCol 0 ascending, got col=%d desc=%v", m.sortCol, m.sortDesc)
	}
	m = sendKey(m, "s") // toggle descending
	if got := colValues(m, "n"); !(got[0] == "85" && got[1] == "54" && got[2] == "36") {
		t.Errorf("descending sort wrong: %v", got)
	}
	if !m.sortDesc {
		t.Error("second sort on same column should be descending")
	}
}

func TestSortStrings(t *testing.T) {
	m := New(parse(t, `[{"s":"banana"},{"s":"apple"},{"s":"cherry"}]`), "t.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "s")
	got := colValues(m, "s")
	if !(got[0] == `"apple"` && got[1] == `"banana"` && got[2] == `"cherry"`) {
		t.Errorf("string sort wrong: %v", got)
	}
}

func TestSortMixedAndMissingTypes(t *testing.T) {
	// Ranks: missing/null < bool < number < string. Ascending should group them
	// in that order.
	m := New(parse(t, `[{"v":"text"},{"v":5},{"v":true},{"x":1},{"v":null}]`), "t.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "s") // sort by "v"
	got := colValues(m, "v")
	// Expect null-rank first (missing "" then null, stable), then bool, number,
	// string. Missing renders as "" and null as "null".
	if got[0] != "" || got[1] != "null" {
		t.Errorf("missing/null should sort first, got %v", got)
	}
	if got[2] != "true" || got[3] != "5" || got[4] != `"text"` {
		t.Errorf("mixed-type order wrong: %v", got)
	}
}

func TestSortCursorFollowsRow(t *testing.T) {
	m := New(parse(t, `[{"n":54},{"n":36},{"n":85}]`), "t.json")
	m = sized(m, 80, 24)
	// Select the row with 85 (index 2), then sort ascending — it should move to
	// the end and the cursor should follow it there.
	m = sendKey(m, "down")
	m = sendKey(m, "down") // row 2 (85)
	m = sendKey(m, "s")
	if m.tableRow != 2 {
		t.Errorf("cursor should follow the 85 row to index 2, got %d", m.tableRow)
	}
}

func TestSortUndoRestoresOrder(t *testing.T) {
	m := New(parse(t, `[{"n":3},{"n":1},{"n":2}]`), "t.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "s") // ascending -> 1,2,3
	if colValues(m, "n")[0] != "1" {
		t.Fatal("setup: sort did not apply")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u"), Alt: true})
	m = updated.(Model)
	got := colValues(m, "n")
	if !(got[0] == "3" && got[1] == "1" && got[2] == "2") {
		t.Errorf("undo should restore original order, got %v", got)
	}
}

func TestSortSingleRowIsNoop(t *testing.T) {
	m := New(parse(t, `[{"n":1}]`), "t.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "s")
	if m.status != "nothing to sort" {
		t.Errorf("expected nothing-to-sort message, got %q", m.status)
	}
	if m.dirty {
		t.Error("a no-op sort should not mark the document dirty")
	}
}

func TestCompareCellsOrdering(t *testing.T) {
	num := func(s string) *document.Node {
		n, _ := document.ParseNumber(s)
		return document.NewNumber(n)
	}
	if compareCells(num("2"), num("10")) >= 0 {
		t.Error("2 should sort before 10 numerically, not lexically")
	}
	if compareCells(document.NewNull(), document.NewBool(false)) >= 0 {
		t.Error("null should rank before bool")
	}
	if compareCells(nil, document.NewNull()) != 0 {
		t.Error("missing and null should compare equal")
	}
}
