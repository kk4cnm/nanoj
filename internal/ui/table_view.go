package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/kk4cnm/nanoj/internal/document"
)

// clip truncates a (possibly styled) line to width display columns, preserving
// ANSI styling. It guards against wrapping when a single column is wider than
// the screen — visibleColumns already prevents overflow in the normal case.
func clip(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "")
}

// enterTable switches to the table view for the tabular array at or containing
// the selection. Structural edits (add/delete/retype) stay in the tree view,
// so the table focuses on fast cell value editing.
func (m *Model) enterTable() {
	row := m.rows[m.cursor]

	// Prefer the selected node if it is itself a tabular array; otherwise try
	// its parent (i.e. the cursor is sitting on one of the array's elements).
	if shape, ok := tabularShape(row.Node); ok {
		m.table = shape
		m.tablePath = row.Path
		m.tableRow = 0
	} else if shape, ok := tabularShape(row.Parent); ok {
		m.table = shape
		m.tablePath = parentPath(row.Path)
		m.tableRow = row.Index
	} else {
		m.status = "no array-of-objects here to show as a table"
		return
	}

	m.view = viewTable
	m.tableCol = 0
	m.tableColOff = 0
	m.tableRowOff = 0
	m.sortCol = -1 // a freshly opened table starts unsorted
	m.recomputeColWidths()
	m.clampTableScroll()
}

// recomputeColWidths refreshes the cached column widths. It is called only when
// the table's content changes (entering the table, an edit, an undo) — never on
// plain navigation, so moving the cursor stays O(viewport) on huge tables.
func (m *Model) recomputeColWidths() {
	m.tableColWidths = m.table.columnWidths()
	// Reserve room in every column for a trailing overlay marker (" ✗" etc.) so
	// marked and unmarked cells stay aligned.
	if m.hasOverlay() {
		for c := range m.tableColWidths {
			m.tableColWidths[c] += 2
		}
	}
	// Reserve room in the sorted column for the " ▲" / " ▼" header marker.
	if m.sortCol >= 0 && m.sortCol < len(m.tableColWidths) {
		m.tableColWidths[m.sortCol] += 2
	}
}

// columnHeader returns the header text for a column, with a sort-direction
// marker appended when it is the active sort column.
func (m Model) columnHeader(c int) string {
	name := m.table.columns[c]
	if c == m.sortCol {
		if m.sortDesc {
			return name + " ▼"
		}
		return name + " ▲"
	}
	return name
}

// exitTable returns to the tree view, leaving the cursor on the array.
func (m *Model) exitTable() {
	m.view = viewTree
	m.ensureRows()
	for i, r := range m.rows {
		if r.Node == m.table.array {
			m.cursor = i
			break
		}
	}
	m.clampScroll()
}

// updateTable handles keystrokes while the table view is active.
func (m Model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m
	p.status = ""
	switch msg.String() {
	case "ctrl+x":
		return m, p.beginQuit()
	case "ctrl+o":
		p.beginSave()
		return m, textinput.Blink
	case "ctrl+t", "esc":
		p.exitTable()
	case "ctrl+w":
		p.beginSearch()
		return m, textinput.Blink
	case "ctrl+k":
		p.cutRow()
	case "alt+6":
		p.copyRow()
	case "ctrl+u":
		p.pasteRow()
	case "s":
		p.sortByCurrentColumn()
	case "up", "ctrl+p", "k":
		p.moveTableCursor(-1, 0)
	case "down", "ctrl+n", "j":
		p.moveTableCursor(1, 0)
	case "left", "h":
		p.moveTableCursor(0, -1)
	case "right", "l":
		p.moveTableCursor(0, 1)
	case "enter":
		p.editCell()
		if p.mode == modeInput {
			return m, textinput.Blink
		}
	case "alt+u", "ctrl+z":
		p.undo()
		p.refreshTableShape()
	case "alt+e", "ctrl+y":
		p.redo()
		p.refreshTableShape()
	case "home":
		p.tableRow = 0
		p.clampTableScroll()
	case "end":
		p.tableRow = len(p.table.array.Items) - 1
		p.clampTableScroll()
	case "ctrl+a":
		p.tableCol = 0
		p.clampTableScroll()
	case "ctrl+e":
		p.tableCol = len(p.table.columns) - 1
		p.clampTableScroll()
	case "pgup":
		p.moveTableCursor(-p.tableBodyHeight(), 0)
	case "pgdown":
		p.moveTableCursor(p.tableBodyHeight(), 0)
	}
	return m, nil
}

// moveTableCursor moves the selected cell by the given row/column deltas,
// clamping to the grid and scrolling to keep the cell visible.
func (m *Model) moveTableCursor(dRow, dCol int) {
	m.tableRow += dRow
	m.tableCol += dCol
	if m.tableRow < 0 {
		m.tableRow = 0
	}
	if last := len(m.table.array.Items) - 1; m.tableRow > last {
		m.tableRow = last
	}
	if m.tableCol < 0 {
		m.tableCol = 0
	}
	if last := len(m.table.columns) - 1; m.tableCol > last {
		m.tableCol = last
	}
	m.clampTableScroll()
}

// editCell opens a value prompt for the selected cell.
//
//   - A string or number is edited keeping its type.
//   - A bool is toggled in place.
//   - An empty (missing-key) or null cell is filled with a value whose type is
//     inferred from what you type, so you can build out a table inline.
//   - A nested container is edited in the tree view (it doesn't fit a cell).
func (m *Model) editCell() {
	if m.readOnlyBlocked() {
		return
	}
	n := m.table.cellNode(m.tableRow, m.tableCol)

	// If the schema constrains this cell to an enum, offer the pick-list, same
	// as Enter does in the tree view.
	if n != nil && !n.IsContainer() && m.schema != nil {
		if a, ok := m.schemaOverlay.At(m.cellPath(m.tableRow, m.tableCol)); ok && len(a.EnumValues) > 0 {
			m.beginEnumPick(n, a)
			return
		}
	}

	switch {
	case n == nil:
		// Empty cell: add the column's key to this row, type inferred on commit.
		m.pendingNode = m.table.array.Items[m.tableRow]
		m.cellKey = m.table.columns[m.tableCol]
		m.beginInput(actSetCell, "Set "+m.cellKey+" (type inferred): ", "")
	case n.IsContainer():
		m.status = "nested value — edit it in the tree view (^T)"
	case n.Kind == document.KindBool:
		m.pushUndo()
		n.Bool = !n.Bool
		m.dirty = true
		m.recomputeColWidths()
	case n.Kind == document.KindNull:
		// Replace the null with an inferred-type value.
		m.editTarget = n
		m.beginInput(actSetCell, "Set value (type inferred): ", "")
	default: // string or number — keep the existing type
		m.beginEditValue(n)
	}
}

// refreshTableShape re-resolves the table's array and column set after an
// undo/redo. Undo swaps in a freshly cloned tree, so the array must be looked
// up again by its structural path rather than by the now-stale pointer. Falls
// back to the tree view if that path is no longer a tabular array.
func (m *Model) refreshTableShape() {
	if shape, ok := tabularShape(nodeAtPath(m.root, m.tablePath)); ok {
		m.table = shape
		m.recomputeColWidths()
		m.clampTableScroll()
		return
	}
	m.exitTable()
}

// clampTableScroll keeps the selected cell within the visible window.
func (m *Model) clampTableScroll() {
	vh := m.tableBodyHeight()
	if m.tableRow < m.tableRowOff {
		m.tableRowOff = m.tableRow
	}
	if m.tableRow >= m.tableRowOff+vh {
		m.tableRowOff = m.tableRow - vh + 1
	}
	if m.tableRowOff < 0 {
		m.tableRowOff = 0
	}

	widths := m.tableColWidths
	idxW := m.indexWidth()
	// Scroll left if the cursor is left of the window.
	if m.tableColOff > m.tableCol {
		m.tableColOff = m.tableCol
	}
	// Scroll right until the cursor is visible. This terminates because colOff
	// only advances toward tableCol, and once colOff == tableCol the cursor is
	// always within the window (visibleColumns shows at least one column).
	for m.tableColOff < m.tableCol {
		_, end := visibleColumns(widths, idxW, m.width, m.tableColOff)
		if m.tableCol < end {
			break
		}
		m.tableColOff++
	}
}

// indexWidth is the width of the leading row-number column.
func (m Model) indexWidth() int {
	n := len(m.table.array.Items)
	w := len(strconv.Itoa(n))
	if w < 1 {
		w = 1
	}
	return w
}

// tableBodyHeight is the number of data rows that fit (title + header +
// separator + 2 help lines = 5 lines of chrome).
func (m Model) tableBodyHeight() int {
	h := m.height - 5
	if h < 1 {
		return 1
	}
	return h
}

// renderTable renders the grid: title, header row, separator, data rows, help.
// When a schema or diff overlay is active, a two-column marker gutter on the
// left carries row-level annotations (e.g. an object missing a required key, a
// row added vs the baseline) and each cell shows its own trailing marker.
func (m Model) renderTable() string {
	var b strings.Builder
	b.WriteString(m.titleBar())
	b.WriteByte('\n')

	widths := m.tableColWidths
	idxW := m.indexWidth()
	gutter := m.hasOverlay()
	start, end := visibleColumns(widths, idxW, m.width, m.tableColOff)

	// Header.
	var head strings.Builder
	if gutter {
		head.WriteString("  ")
	}
	head.WriteString(strings.Repeat(" ", idxW))
	for c := start; c < end; c++ {
		head.WriteString(tableColSep)
		head.WriteString(m.theme.Header.Render(fit(m.columnHeader(c), widths[c])))
	}
	b.WriteString(clip(pad(head.String(), m.width), m.width))
	b.WriteByte('\n')

	// Separator.
	sepWidth := lipgloss.Width(head.String())
	if sepWidth > m.width {
		sepWidth = m.width
	}
	b.WriteString(m.theme.Structure.Render(strings.Repeat("─", sepWidth)))
	b.WriteByte('\n')

	// Data rows.
	vh := m.tableBodyHeight()
	endRow := m.tableRowOff + vh
	if endRow > len(m.table.array.Items) {
		endRow = len(m.table.array.Items)
	}
	shown := 0
	for r := m.tableRowOff; r < endRow; r++ {
		var line strings.Builder
		if gutter {
			if d := m.decorAt(childPath(m.tablePath, r)); d.marker != "" {
				line.WriteString(d.style.Render(d.marker))
				line.WriteByte(' ')
			} else {
				line.WriteString("  ")
			}
		}
		line.WriteString(m.theme.Structure.Render(fit(strconv.Itoa(r), idxW)))
		for c := start; c < end; c++ {
			line.WriteString(m.theme.Structure.Render(tableColSep))
			line.WriteString(m.renderCell(r, c, widths[c]))
		}
		b.WriteString(clip(line.String(), m.width))
		b.WriteByte('\n')
		shown++
	}
	for ; shown < vh; shown++ {
		b.WriteByte('\n')
	}

	b.WriteString(m.tableStatusBar())
	return b.String()
}

// renderCell renders one data cell. While an overlay is active, column widths
// reserve two trailing columns for a marker, so an annotated cell (a schema ✗
// or a diff +/~) shows its glyph without disturbing alignment.
func (m Model) renderCell(r, c, width int) string {
	n := m.table.cellNode(r, c)
	selected := r == m.tableRow && c == m.tableCol
	var d rowDecor
	if n != nil && m.hasOverlay() {
		d = m.decorAt(m.cellPath(r, c))
	}
	if d.marker == "" {
		cell := fit(cellText(n), width)
		if selected {
			return m.theme.Selection.Render(cell)
		}
		return cell
	}
	body := fit(cellText(n), width-2)
	if selected {
		return m.theme.Selection.Render(body + " " + d.marker)
	}
	return body + " " + d.style.Render(d.marker)
}

// cellPath returns the structural path of the cell's value node, or "" when
// the row's object does not contain the column's key.
func (m Model) cellPath(row, col int) string {
	obj := m.table.array.Items[row]
	key := m.table.columns[col]
	for i, mem := range obj.Members {
		if mem.Key == key {
			return childPath(childPath(m.tablePath, row), i)
		}
	}
	return ""
}

func (m Model) tableStatusBar() string {
	if m.mode == modeInput {
		line1 := m.prompt + m.input.View()
		if m.promptErr != "" {
			line1 += "  [" + m.promptErr + "]"
		}
		return pad(" "+line1, m.width) + "\n" + pad(" Enter Confirm    ^C Cancel", m.width)
	}
	if m.status != "" {
		line1 := lipgloss.NewStyle().Reverse(true).Render(pad(" "+m.status, m.width))
		return line1 + "\n" + pad(" "+m.tablePosition(), m.width)
	}
	// With no transient message, show the selected cell's schema/diff info.
	if path := m.cellPath(m.tableRow, m.tableCol); path != "" {
		if info := m.overlayInfo(path); info != "" {
			line1 := lipgloss.NewStyle().Reverse(true).Render(pad(" "+info, m.width))
			return line1 + "\n" + pad(" "+m.tablePosition(), m.width)
		}
	}
	line1 := pad(" "+strings.Join([]string{"^T Tree view", "Enter Edit cell", "s Sort col", "^W Search", "^X Exit"}, "    "), m.width)
	help := " " + strings.Join([]string{"↑↓←→ Move", "^K Cut", "M-6 Copy", "^U Paste row"}, "    ")
	line2 := padBetween(help, m.tablePosition()+" ", m.width)
	return line1 + "\n" + line2
}

// tablePosition summarizes the cursor location and signals when columns are
// scrolled off-screen with ‹ (more to the left) and › (more to the right).
func (m Model) tablePosition() string {
	widths := m.tableColWidths
	start, end := visibleColumns(widths, m.indexWidth(), m.width, m.tableColOff)
	left, right := " ", " "
	if start > 0 {
		left = "‹"
	}
	if end < len(m.table.columns) {
		right = "›"
	}
	return fmt.Sprintf("TABLE  row %d/%d  %scol %d/%d%s",
		m.tableRow+1, len(m.table.array.Items),
		left, m.tableCol+1, len(m.table.columns), right)
}
