package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kk4cnm/nanoj/internal/document"
)

// sortByCurrentColumn sorts the table's rows by the selected column. Pressing
// it again on the same column flips the direction. The sort reorders the
// underlying array in the document (so it persists on save and is undoable),
// and the cursor follows the previously-selected row to its new position.
func (m *Model) sortByCurrentColumn() {
	if m.readOnlyBlocked() {
		return
	}
	items := m.table.array.Items
	if len(m.table.columns) == 0 {
		return
	}
	if len(items) < 2 {
		m.status = "nothing to sort"
		return
	}

	col := m.tableCol
	if m.sortCol == col {
		m.sortDesc = !m.sortDesc
	} else {
		m.sortCol = col
		m.sortDesc = false
	}
	key := m.table.columns[col]

	// Remember the selected row so the cursor can follow it after reordering.
	var selected *document.Node
	if m.tableRow >= 0 && m.tableRow < len(items) {
		selected = items[m.tableRow]
	}

	m.pushUndo()
	sort.SliceStable(items, func(i, j int) bool {
		c := compareCells(cellByKey(items[i], key), cellByKey(items[j], key))
		if m.sortDesc {
			return c > 0
		}
		return c < 0
	})
	m.dirty = true

	if selected != nil {
		for idx, it := range items {
			if it == selected {
				m.tableRow = idx
				break
			}
		}
	}
	m.recomputeColWidths()
	m.clampTableScroll()

	arrow := "ascending"
	if m.sortDesc {
		arrow = "descending"
	}
	m.status = fmt.Sprintf("sorted by %q (%s)", key, arrow)
}

// cellByKey returns the value for key in an object row, or nil if absent.
func cellByKey(obj *document.Node, key string) *document.Node {
	for _, mem := range obj.Members {
		if mem.Key == key {
			return mem.Value
		}
	}
	return nil
}

// compareCells orders two cell values, returning a negative, zero, or positive
// result. Different JSON types are ordered by a fixed type rank (missing/null <
// bool < number < string < container) so a column with mixed or missing values
// still sorts deterministically; within a type, values compare naturally.
func compareCells(a, b *document.Node) int {
	if ra, rb := cellRank(a), cellRank(b); ra != rb {
		return ra - rb
	}
	switch cellRank(a) {
	case rankBool:
		return boolToInt(a.Bool) - boolToInt(b.Bool)
	case rankNumber:
		af, aerr := strconv.ParseFloat(a.Num.String(), 64)
		bf, berr := strconv.ParseFloat(b.Num.String(), 64)
		if aerr == nil && berr == nil {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
		return strings.Compare(a.Num.String(), b.Num.String())
	case rankString:
		return strings.Compare(a.Str, b.Str)
	default:
		return 0
	}
}

const (
	rankNull = iota
	rankBool
	rankNumber
	rankString
	rankContainer
)

func cellRank(n *document.Node) int {
	if n == nil {
		return rankNull
	}
	switch n.Kind {
	case document.KindNull:
		return rankNull
	case document.KindBool:
		return rankBool
	case document.KindNumber:
		return rankNumber
	case document.KindString:
		return rankString
	default:
		return rankContainer
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
