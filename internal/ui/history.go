package ui

// Undo/redo is snapshot-based: before each mutation the model deep-clones the
// tree onto the undo stack. This is simple and robust — the alternative
// (recording inverse operations per edit) is more code and easier to get
// subtly wrong. Documents in nanoj's scope are small enough that whole-tree
// snapshots are cheap.

// snapshotState captures the current editable state.
func (m *Model) snapshotState() snapshot {
	return snapshot{
		root:      m.root.Clone(),
		collapsed: cloneStrSet(m.collapsed),
		cursor:    m.cursor,
		dirty:     m.dirty,
	}
}

// pushUndo records the current state so the impending mutation can be undone,
// and clears the redo stack (a new edit invalidates the redo history).
func (m *Model) pushUndo() {
	m.undoStack = append(m.undoStack, m.snapshotState())
	m.redoStack = nil
	m.overlayStale = true // the impending mutation invalidates the overlays
}

// popUndo discards the most recent undo snapshot. It is used when a mutation
// was snapshotted but then failed, so no empty step is left behind.
func (m *Model) popUndo() {
	if len(m.undoStack) > 0 {
		m.undoStack = m.undoStack[:len(m.undoStack)-1]
	}
}

// undo restores the previous state, pushing the current state onto redo.
func (m *Model) undo() {
	if m.readOnlyBlocked() {
		return
	}
	if len(m.undoStack) == 0 {
		m.status = "nothing to undo"
		return
	}
	m.redoStack = append(m.redoStack, m.snapshotState())
	s := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	m.restore(s)
	m.status = "undid change"
}

// redo re-applies the most recently undone state.
func (m *Model) redo() {
	if m.readOnlyBlocked() {
		return
	}
	if len(m.redoStack) == 0 {
		m.status = "nothing to redo"
		return
	}
	m.undoStack = append(m.undoStack, m.snapshotState())
	s := m.redoStack[len(m.redoStack)-1]
	m.redoStack = m.redoStack[:len(m.redoStack)-1]
	m.restore(s)
	m.status = "redid change"
}

// restore makes the given snapshot the live state. Because expansion is
// path-based, the cloned tree's collapsed paths line up without translation.
func (m *Model) restore(s snapshot) {
	m.root = s.root
	m.collapsed = s.collapsed
	m.dirty = s.dirty
	m.overlayStale = true // undo/redo swaps the tree; overlays must be recomputed
	m.rebuild()
	m.cursor = s.cursor
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampScroll()
}

func cloneStrSet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
