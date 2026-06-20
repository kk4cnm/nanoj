package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestTabularShapeDetection(t *testing.T) {
	// Array of objects with differing keys -> tabular, columns are the union.
	arr := parse(t, `[{"a":1,"b":2},{"a":3,"c":4}]`)
	shape, ok := tabularShape(arr)
	if !ok {
		t.Fatal("expected array of objects to be tabular")
	}
	if got := strings.Join(shape.columns, ","); got != "a,b,c" {
		t.Errorf("expected columns a,b,c in first-seen order, got %s", got)
	}

	// Non-tabular cases.
	for _, src := range []string{
		`[]`,          // empty
		`[1,2,3]`,     // scalars
		`[{"a":1},5]`, // mixed
		`{"a":1}`,     // object, not array
	} {
		if _, ok := tabularShape(parse(t, src)); ok {
			t.Errorf("expected %s to be non-tabular", src)
		}
	}
}

func TestCellNodeMissingKey(t *testing.T) {
	arr := parse(t, `[{"a":1},{"b":2}]`)
	shape, _ := tabularShape(arr)
	// columns: a, b. Row 0 has no "b"; row 1 has no "a".
	if n := shape.cellNode(0, 1); n != nil {
		t.Errorf("expected missing cell (0,b) to be nil, got %v", n.Kind)
	}
	if n := shape.cellNode(0, 0); n == nil || n.Num.String() != "1" {
		t.Errorf("expected cell (0,a)=1")
	}
}

func TestAutoDetectOpensTable(t *testing.T) {
	m := New(parse(t, `[{"x":1},{"x":2}]`), "t.json")
	if m.view != viewTable {
		t.Errorf("array-of-objects document should auto-open in table view")
	}
	// A plain object should stay in tree view.
	m2 := New(parse(t, `{"x":1}`), "t.json")
	if m2.view != viewTree {
		t.Errorf("object document should open in tree view")
	}
}

func TestTableNavigationClamps(t *testing.T) {
	m := New(parse(t, `[{"a":1,"b":2},{"a":3,"b":4}]`), "t.json")
	m = sized(m, 80, 24)
	// Move down/right past the edges; should clamp at (1,1).
	for i := 0; i < 5; i++ {
		m = sendKey(m, "down")
		m = sendKey(m, "right")
	}
	if m.tableRow != 1 || m.tableCol != 1 {
		t.Errorf("expected clamp at (1,1), got (%d,%d)", m.tableRow, m.tableCol)
	}
}

func TestTableEditCell(t *testing.T) {
	m := New(parse(t, `[{"n":1},{"n":2}]`), "t.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down")  // row 1
	m = sendKey(m, "enter") // edit cell (1,0) prefilled "2"
	if m.mode != modeInput {
		t.Fatalf("expected edit prompt, got mode %d", m.mode)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	m = typeText(m, "99")
	m = sendKey(m, "enter")
	if got := m.root.Items[1].Members[0].Value.Num.String(); got != "99" {
		t.Errorf("expected cell edit to set 99, got %s", got)
	}
	if !m.dirty {
		t.Error("editing a cell should mark the model dirty")
	}
}

func TestTableToggleBackToTree(t *testing.T) {
	m := New(parse(t, `[{"a":1}]`), "t.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.view != viewTree {
		t.Errorf("^T should switch back to tree view")
	}
	// Cursor should land on the array node.
	if m.rows[m.cursor].Node != m.table.array {
		t.Errorf("after exit, cursor should be on the array node")
	}
}

func TestEnterTableFromNestedArray(t *testing.T) {
	// Root is an object; the tabular array is nested under "items".
	m := New(parse(t, `{"items":[{"a":1},{"a":2}]}`), "t.json")
	m = sized(m, 80, 24)
	if m.view != viewTree {
		t.Fatal("object root should start in tree view")
	}
	m = sendKey(m, "down") // onto "items" array
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.view != viewTable {
		t.Errorf("^T on a tabular array should enter table view")
	}
	if len(m.table.columns) != 1 || m.table.columns[0] != "a" {
		t.Errorf("unexpected columns: %v", m.table.columns)
	}
}

// wideDoc builds an array of rows each with cols columns, values "r{r}c{c}".
func wideDoc(t *testing.T, rows, cols int) string {
	t.Helper()
	var b strings.Builder
	b.WriteByte('[')
	for r := 0; r < rows; r++ {
		if r > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('{')
		for c := 0; c < cols; c++ {
			if c > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `"col%d":"r%dc%d"`, c, r, c)
		}
		b.WriteByte('}')
	}
	b.WriteByte(']')
	return b.String()
}

func TestTableHorizontalScroll(t *testing.T) {
	m := New(parse(t, wideDoc(t, 3, 8)), "w.json")
	m = sized(m, 40, 12) // narrow: only a few columns fit
	if m.tableColOff != 0 {
		t.Fatalf("should start unscrolled, got colOff %d", m.tableColOff)
	}
	for i := 0; i < 7; i++ {
		m = sendKey(m, "right")
	}
	if m.tableCol != 7 {
		t.Fatalf("expected to reach last column, got %d", m.tableCol)
	}
	if m.tableColOff == 0 {
		t.Errorf("reaching the last column should have scrolled columns (colOff>0)")
	}
	// Scrolling back left should bring the offset home.
	for i := 0; i < 7; i++ {
		m = sendKey(m, "left")
	}
	if m.tableColOff != 0 {
		t.Errorf("returning to column 0 should reset colOff, got %d", m.tableColOff)
	}
}

func TestTableColumnEndKeys(t *testing.T) {
	m := New(parse(t, wideDoc(t, 2, 8)), "w.json")
	m = sized(m, 40, 12)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = updated.(Model)
	if m.tableCol != 7 {
		t.Errorf("^E should jump to last column, got %d", m.tableCol)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = updated.(Model)
	if m.tableCol != 0 || m.tableColOff != 0 {
		t.Errorf("^A should jump to first column and unscroll, got col %d off %d", m.tableCol, m.tableColOff)
	}
}

func TestTableLinesNeverWrap(t *testing.T) {
	// A single column far wider than the screen must still not overflow.
	m := New(parse(t, `[{"big":"`+strings.Repeat("x", 200)+`"}]`), "w.json")
	m = sized(m, 30, 10)
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 30 {
			t.Errorf("line exceeds width 30 (got %d): %q", w, line)
		}
	}
}

func TestTablePositionIndicator(t *testing.T) {
	m := New(parse(t, wideDoc(t, 2, 8)), "w.json")
	m = sized(m, 40, 12)
	out := m.View()
	if !strings.Contains(out, "col 1/8") {
		t.Errorf("expected column position 'col 1/8' in view:\n%s", out)
	}
	if !strings.Contains(out, "›") {
		t.Errorf("expected a › indicator showing more columns to the right:\n%s", out)
	}
}

func TestTableSearchFindsCell(t *testing.T) {
	m := New(parse(t, `[{"a":"apple","b":"red"},{"a":"banana","b":"yellow"}]`), "f.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	if m.mode != modeInput {
		t.Fatalf("^W should open the search prompt in table view, got mode %d", m.mode)
	}
	m = typeText(m, "yellow")
	m = sendKey(m, "enter")
	if m.tableRow != 1 || m.tableCol != 1 {
		t.Errorf("expected to land on cell (1,1) for 'yellow', got (%d,%d)", m.tableRow, m.tableCol)
	}
	if m.status != "found: yellow" {
		t.Errorf("expected found status, got %q", m.status)
	}
}

func TestTableSearchWrapsForward(t *testing.T) {
	m := New(parse(t, `[{"x":"one"},{"x":"two"},{"x":"three"}]`), "f.json")
	m = sized(m, 80, 24)
	// Move to the last row, then search for a value earlier in the table.
	m = sendKey(m, "end") // row 2
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "one")
	m = sendKey(m, "enter")
	if m.tableRow != 0 {
		t.Errorf("search should wrap forward to row 0, got %d", m.tableRow)
	}
}

func TestTableSearchNotFound(t *testing.T) {
	m := New(parse(t, `[{"x":"one"}]`), "f.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "zzz")
	m = sendKey(m, "enter")
	if m.status != "not found: zzz" {
		t.Errorf("expected not-found status, got %q", m.status)
	}
}

func TestTableSearchRevealsOffscreenMatch(t *testing.T) {
	// 50 rows but a short viewport: a match deep in the table must scroll into
	// view (tableRowOff advances).
	m := New(parse(t, genRows(50, 2)), "f.json")
	m = sized(m, 80, 8) // only a few data rows visible
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "r40c1")
	m = sendKey(m, "enter")
	if m.tableRow != 40 || m.tableCol != 1 {
		t.Fatalf("expected to find r40c1 at (40,1), got (%d,%d)", m.tableRow, m.tableCol)
	}
	if m.tableRowOff == 0 {
		t.Errorf("match deep in the table should have scrolled the viewport")
	}
}

func TestTableViewRenders(t *testing.T) {
	m := New(parse(t, `[{"id":1,"name":"Ada"},{"id":2,"name":"Linus"}]`), "p.json")
	m = sized(m, 80, 14)
	out := m.View()
	for _, want := range []string{"id", "name", "\"Ada\"", "\"Linus\"", "TABLE", "^T Tree view"} {
		if !strings.Contains(out, want) {
			t.Errorf("table view missing %q\n%s", want, out)
		}
	}
}
