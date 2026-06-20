package ui

import (
	"fmt"

	"github.com/kk4cnm/nanoj/internal/document"
)

// Cut/copy/paste works in both views and reuses undo and the document mutation
// primitives. The clipboard holds a detached clone, and paste inserts a fresh
// clone of it, so the same item can be pasted repeatedly and editing the
// original never disturbs the clipboard.

// setClipboard stores a clone of n, remembering its object key (if any) so a
// later paste into an object can reuse it.
func (m *Model) setClipboard(n *document.Node, hasKey bool, key string) {
	m.clipboard = n.Clone()
	m.clipboardHasKey = hasKey
	m.clipboardKey = key
}

// --- tree view ---

// copyCurrent copies the selected node to the clipboard.
func (m *Model) copyCurrent() {
	if len(m.rows) == 0 {
		return
	}
	r := m.rows[m.cursor]
	m.setClipboard(r.Node, r.HasKey, r.Key)
	m.status = "copied"
}

// cutCurrent copies the selected node to the clipboard and removes it. There is
// no confirmation: a cut is recoverable via paste (^U) or undo (M-U).
func (m *Model) cutCurrent() {
	if len(m.rows) == 0 {
		return
	}
	r := m.rows[m.cursor]
	if r.Parent == nil {
		m.status = "cannot cut the root node"
		return
	}
	m.pushUndo()
	m.setClipboard(r.Node, r.HasKey, r.Key)
	_ = r.Parent.RemoveChild(r.Index)
	m.dirty = true
	m.rebuild()
	m.clampScroll()
	m.status = "cut"
}

// pasteCurrent inserts a clone of the clipboard relative to the selection: as
// the last child when a container is selected, otherwise as the next sibling.
func (m *Model) pasteCurrent() {
	if m.clipboard == nil {
		m.status = "clipboard is empty"
		return
	}
	if len(m.rows) == 0 {
		return
	}
	r := m.rows[m.cursor]

	var target *document.Node
	var at int
	if r.Node.IsContainer() {
		target = r.Node
		at = r.Node.ChildCount()
		delete(m.collapsed, r.Path) // expand so the pasted child is visible
	} else {
		target = r.Parent
		at = r.Index + 1
	}
	if target == nil {
		m.status = "nowhere to paste here"
		return
	}

	m.pushUndo()
	clone := m.clipboard.Clone()
	switch target.Kind {
	case document.KindArray:
		_ = target.InsertItem(at, clone)
	case document.KindObject:
		_ = target.InsertMember(at, m.pasteKey(target), clone)
	default:
		m.popUndo()
		m.status = "can only paste into an array or object"
		return
	}
	m.dirty = true
	m.rebuild()
	m.selectChild(target, at)
	m.status = "pasted"
}

// --- table view ---

// copyRow copies the selected table row (an object) to the clipboard.
func (m *Model) copyRow() {
	if len(m.table.array.Items) == 0 {
		return
	}
	m.setClipboard(m.table.array.Items[m.tableRow], false, "")
	m.status = "copied row"
}

// cutRow copies the selected row to the clipboard and removes it.
func (m *Model) cutRow() {
	arr := m.table.array
	if len(arr.Items) == 0 {
		return
	}
	m.pushUndo()
	m.setClipboard(arr.Items[m.tableRow], false, "")
	_ = arr.RemoveChild(m.tableRow)
	m.dirty = true
	if m.tableRow >= len(arr.Items) && m.tableRow > 0 {
		m.tableRow--
	}
	m.refreshTableShape() // recompute columns/widths, or exit if no rows remain
	m.status = "cut row"
}

// pasteRow inserts a clone of the clipboard as a row after the selection. Only
// objects can be table rows.
func (m *Model) pasteRow() {
	if m.clipboard == nil {
		m.status = "clipboard is empty"
		return
	}
	if m.clipboard.Kind != document.KindObject {
		m.status = "can only paste an object as a table row"
		return
	}
	arr := m.table.array
	at := m.tableRow + 1
	if len(arr.Items) == 0 {
		at = 0
	}
	m.pushUndo()
	_ = arr.InsertItem(at, m.clipboard.Clone())
	m.dirty = true
	m.tableRow = at
	m.refreshTableShape() // a pasted row may introduce new columns
	m.status = "pasted row"
}

// pasteKey chooses the key for a node pasted into an object: the clipboard's
// remembered key (or "value"), made unique within the target.
func (m *Model) pasteKey(target *document.Node) string {
	base := "value"
	if m.clipboardHasKey && m.clipboardKey != "" {
		base = m.clipboardKey
	}
	return dedupeKey(target, base)
}

// dedupeKey returns base, or base_2/base_3/… if base already exists in obj, so
// pasting doesn't silently create duplicate keys.
func dedupeKey(obj *document.Node, base string) string {
	exists := func(k string) bool {
		for _, mem := range obj.Members {
			if mem.Key == k {
				return true
			}
		}
		return false
	}
	if !exists(base) {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s_%d", base, i)
		if !exists(cand) {
			return cand
		}
	}
}
