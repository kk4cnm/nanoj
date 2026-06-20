package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kk4cnm/nanoj/internal/document"
)

// The table view renders an array of objects as a spreadsheet-style grid: each
// array element is a row, and the union of the elements' keys forms the
// columns. It is a second *rendering* of the same document tree — the tree
// remains the single source of truth, so edits made here go through the same
// mutation primitives and undo machinery as the tree view.

const (
	tableMaxColWidth = 30 // cap a column's width so one long value can't dominate
	tableColSep      = " │ "
)

// tableShape describes an array that can be shown as a table.
type tableShape struct {
	array   *document.Node // the array node (rows)
	columns []string       // union of member keys, in first-seen order
}

// tabularShape reports whether arr is an array whose elements are all objects,
// and if so returns its column set (the union of keys across all rows, in the
// order they first appear). An empty array is not considered tabular.
func tabularShape(arr *document.Node) (tableShape, bool) {
	if arr == nil || arr.Kind != document.KindArray || len(arr.Items) == 0 {
		return tableShape{}, false
	}
	var cols []string
	seen := map[string]bool{}
	for _, item := range arr.Items {
		if item.Kind != document.KindObject {
			return tableShape{}, false
		}
		for _, m := range item.Members {
			if !seen[m.Key] {
				seen[m.Key] = true
				cols = append(cols, m.Key)
			}
		}
	}
	return tableShape{array: arr, columns: cols}, true
}

// cellNode returns the value at the given row/column, or nil if that row's
// object does not contain the column's key.
func (s tableShape) cellNode(row, col int) *document.Node {
	if row < 0 || row >= len(s.array.Items) || col < 0 || col >= len(s.columns) {
		return nil
	}
	obj := s.array.Items[row]
	key := s.columns[col]
	for _, m := range obj.Members {
		if m.Key == key {
			return m.Value
		}
	}
	return nil
}

// cellText returns the display text for a cell: a scalar's value, a placeholder
// for a nested container, or "" for a missing key.
func cellText(n *document.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case document.KindObject:
		return "{…}"
	case document.KindArray:
		return "[…]"
	default:
		return scalarText(n)
	}
}

// columnWidths computes the display width of each column (header vs. cells),
// capped at tableMaxColWidth.
func (s tableShape) columnWidths() []int {
	widths := make([]int, len(s.columns))
	for c, key := range s.columns {
		w := lipgloss.Width(key)
		for r := range s.array.Items {
			if cw := lipgloss.Width(cellText(s.cellNode(r, c))); cw > w {
				w = cw
			}
		}
		if w > tableMaxColWidth {
			w = tableMaxColWidth
		}
		widths[c] = w
	}
	return widths
}

// visibleColumns returns the half-open range [start,end) of columns that fit in
// screenW starting at colOff, always showing at least the first column.
func visibleColumns(widths []int, idxWidth, screenW, colOff int) (start, end int) {
	start = colOff
	if start < 0 {
		start = 0
	}
	sep := lipgloss.Width(tableColSep)
	used := idxWidth + sep
	end = start
	for end < len(widths) {
		w := widths[end] + sep
		if used+w > screenW && end > start {
			break
		}
		used += w
		end++
	}
	if end == start && start < len(widths) {
		end = start + 1 // guarantee progress even for a very narrow screen
	}
	return start, end
}

// fit pads or truncates s to exactly width display columns. Truncated text ends
// with an ellipsis.
func fit(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	if width <= 1 {
		return strings.Repeat(" ", width)
	}
	// Truncate to width-1 runes then add the ellipsis. Runs on display width.
	runes := []rune(s)
	out := ""
	for _, r := range runes {
		if lipgloss.Width(out+string(r)) > width-1 {
			break
		}
		out += string(r)
	}
	return fit(out+"…", width)
}
