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
		m.tableRow = 0
	} else if shape, ok := tabularShape(row.Parent); ok {
		m.table = shape
		m.tableRow = row.Index
	} else {
		m.status = "no array-of-objects here to show as a table"
		return
	}

	m.view = viewTable
	m.tableCol = 0
	m.tableColOff = 0
	m.tableRowOff = 0
	m.clampTableScroll()
}

// exitTable returns to the tree view, leaving the cursor on the array.
func (m *Model) exitTable() {
	m.view = viewTree
	m.rebuild()
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

// editCell opens the value prompt for the selected scalar cell. Missing keys
// and nested containers cannot be edited inline (do that in the tree view).
func (m *Model) editCell() {
	n := m.table.cellNode(m.tableRow, m.tableCol)
	if n == nil {
		m.status = "empty cell — add this key in the tree view (^T)"
		return
	}
	if n.IsContainer() {
		m.status = "nested value — edit it in the tree view (^T)"
		return
	}
	if n.Kind == document.KindBool {
		m.pushUndo()
		n.Bool = !n.Bool
		m.dirty = true
		return
	}
	if n.Kind == document.KindNull {
		m.status = "null cell — change its type in the tree view (^T)"
		return
	}
	m.beginEditValue(n)
}

// refreshTableShape recomputes the table's column set after an undo/redo, since
// the underlying array may have changed. Falls back to the tree view if the
// array is no longer tabular (or no longer present).
func (m *Model) refreshTableShape() {
	if shape, ok := tabularShape(m.table.array); ok {
		m.table = shape
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

	widths := m.table.columnWidths()
	idxW := m.indexWidth()
	for {
		start, end := visibleColumns(widths, idxW, m.width, m.tableColOff)
		if m.tableCol < start {
			m.tableColOff = m.tableCol
			continue
		}
		if m.tableCol >= end {
			m.tableColOff++
			continue
		}
		break
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
func (m Model) renderTable() string {
	var b strings.Builder
	b.WriteString(m.titleBar())
	b.WriteByte('\n')

	widths := m.table.columnWidths()
	idxW := m.indexWidth()
	start, end := visibleColumns(widths, idxW, m.width, m.tableColOff)

	// Header.
	var head strings.Builder
	head.WriteString(strings.Repeat(" ", idxW))
	for c := start; c < end; c++ {
		head.WriteString(tableColSep)
		head.WriteString(m.theme.Header.Render(fit(m.table.columns[c], widths[c])))
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
		line.WriteString(m.theme.Structure.Render(fit(strconv.Itoa(r), idxW)))
		for c := start; c < end; c++ {
			line.WriteString(m.theme.Structure.Render(tableColSep))
			cell := fit(cellText(m.table.cellNode(r, c)), widths[c])
			if r == m.tableRow && c == m.tableCol {
				cell = m.theme.Selection.Render(cell)
			}
			line.WriteString(cell)
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
	line1 := pad(" "+strings.Join([]string{"^T Tree view", "Enter Edit cell", "^O Write", "^X Exit"}, "    "), m.width)
	help := " " + strings.Join([]string{"↑↓←→ Move", "^A/^E Col ends"}, "    ")
	line2 := padBetween(help, m.tablePosition()+" ", m.width)
	return line1 + "\n" + line2
}

// tablePosition summarizes the cursor location and signals when columns are
// scrolled off-screen with ‹ (more to the left) and › (more to the right).
func (m Model) tablePosition() string {
	widths := m.table.columnWidths()
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
