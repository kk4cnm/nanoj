package ui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
)

// This file holds the editing actions triggered from modeNormal and the prompt
// handlers that complete them. The actual tree mutations live in the document
// package, so these methods are thin glue: they set up a prompt, then apply the
// validated result and refresh the view.

// beginInput opens a nano-style text prompt prefilled with the given text.
func (m *Model) beginInput(a action, prompt, prefill string) {
	m.mode = modeInput
	m.action = a
	m.prompt = prompt
	m.promptErr = ""
	m.input.SetValue(prefill)
	m.input.CursorEnd()
	m.input.Focus()
}

// beginChoice opens a single-key prompt (type pick or confirmation).
func (m *Model) beginChoice(a action, prompt, hint string) {
	m.mode = modeChoice
	m.action = a
	m.prompt = prompt
	m.choiceHint = hint
	m.promptErr = ""
}

// endPrompt returns to normal mode and clears all prompt state.
func (m *Model) endPrompt() {
	m.mode = modeNormal
	m.action = actNone
	m.prompt = ""
	m.promptErr = ""
	m.choiceHint = ""
	m.pendingNode = nil
	m.editTarget = nil
	m.input.Blur()
	m.input.SetValue("")
}

// beginEditValue opens a prompt to edit a string or number value in place. The
// node is captured as editTarget so the same prompt works from either view.
func (m *Model) beginEditValue(n *document.Node) {
	switch n.Kind {
	case document.KindString:
		m.beginInput(actEditValue, "Value (string): ", n.Str)
		m.editTarget = n
	case document.KindNumber:
		m.beginInput(actEditValue, "Value (number): ", n.Num.String())
		m.editTarget = n
	}
}

// beginChangeType opens the type picker for the selected node.
func (m *Model) beginChangeType() {
	if len(m.rows) == 0 {
		return
	}
	n := m.rows[m.cursor].Node
	if n.IsContainer() && n.ChildCount() > 0 {
		m.status = "cannot change type of a non-empty " + n.Kind.String()
		return
	}
	m.beginChoice(actTypePick, "Change type:", "s)tr  n)um  b)ool  u)null  o)bj  a)rr")
}

// beginAddNode adds a child to the selected container, or a sibling within the
// selected node's parent. Objects prompt for a key; arrays append immediately.
func (m *Model) beginAddNode() {
	if len(m.rows) == 0 {
		return
	}
	row := m.rows[m.cursor]
	target := row.Node
	// If the selected node is a container, add into it (and make sure it is
	// expanded so the new child is visible). Otherwise add a sibling within the
	// selected node's parent, which is already expanded.
	if target.IsContainer() {
		delete(m.collapsed, row.Path)
	} else {
		target = row.Parent
	}
	if target == nil {
		m.status = "nothing to add to here"
		return
	}
	switch target.Kind {
	case document.KindObject:
		m.pendingNode = target
		m.beginInput(actAddKey, "New key: ", "")
	case document.KindArray:
		m.pushUndo()
		_ = target.AppendItem(document.NewNull())
		m.dirty = true
		m.rebuild()
		m.selectChild(target, target.ChildCount()-1)
		m.status = "added array element"
	default:
		m.status = "can only add inside an object or array"
	}
}

// beginDelete asks for confirmation before removing the selected node.
func (m *Model) beginDelete() {
	if len(m.rows) == 0 {
		return
	}
	if m.rows[m.cursor].Parent == nil {
		m.status = "cannot delete the root node"
		return
	}
	m.beginChoice(actConfirmDelete, "Delete this node?", "Y Yes   N No")
}

// beginSave opens the "write out" prompt prefilled with the current path.
func (m *Model) beginSave() {
	m.beginInput(actSaveAs, "File Name to Write: ", m.path)
}

// beginQuit quits immediately if there are no unsaved changes, otherwise it
// asks whether to save first.
func (m *Model) beginQuit() tea.Cmd {
	if !m.dirty {
		m.quitting = true
		return tea.Quit
	}
	m.beginChoice(actConfirmQuit, "Save modified buffer?", "Y Yes   N No   ^C Cancel")
	return nil
}

// commitInput applies a completed text prompt. On a validation error it leaves
// the prompt open with a message rather than discarding the user's input.
func (m *Model) commitInput() tea.Cmd {
	val := m.input.Value()
	switch m.action {
	case actEditValue:
		n := m.editTarget
		if n.Kind == document.KindNumber {
			// Validate before snapshotting so a rejected entry leaves no
			// empty undo step.
			if _, ok := document.ParseNumber(val); !ok {
				m.promptErr = "invalid number"
				return nil
			}
			m.pushUndo()
			_ = n.SetNumber(val)
		} else {
			m.pushUndo()
			n.SetString(val)
		}
		m.dirty = true
		m.endPrompt()
		m.rebuild()

	case actAddKey:
		key := val
		target := m.pendingNode
		m.pushUndo()
		_ = target.AddMember(key, document.NewNull())
		m.dirty = true
		m.endPrompt()
		m.rebuild()
		m.selectChild(target, target.ChildCount()-1)

	case actSaveAs:
		path := strings.TrimSpace(val)
		if path == "" {
			m.promptErr = "name required"
			return nil
		}
		if err := m.save(path); err != nil {
			m.promptErr = err.Error()
			return nil
		}
		m.path = path
		m.dirty = false
		m.endPrompt()
		m.status = "Wrote " + path

	case actSearch:
		query := val
		m.endPrompt()
		m.doSearch(query)
	}
	return nil
}

// handleChoice applies a single-key answer for a type pick or confirmation.
func (m *Model) handleChoice(key string) tea.Cmd {
	switch m.action {
	case actTypePick:
		k, ok := typeForKey(key)
		if !ok {
			return nil // ignore unrecognized keys; stay in the picker
		}
		n := m.rows[m.cursor].Node
		if n.Kind == k {
			m.endPrompt() // no-op pick; nothing to record
			return nil
		}
		m.pushUndo()
		if err := n.ConvertTo(k); err != nil {
			m.popUndo() // conversion refused; discard the snapshot
			m.status = err.Error()
			m.endPrompt()
			return nil
		}
		m.dirty = true
		m.endPrompt()
		m.rebuild()

	case actConfirmDelete:
		switch key {
		case "y", "Y":
			row := m.rows[m.cursor]
			m.pushUndo()
			_ = row.Parent.RemoveChild(row.Index)
			m.dirty = true
			m.endPrompt()
			m.rebuild()
			m.clampScroll()
		case "n", "N":
			m.endPrompt()
		}

	case actConfirmQuit:
		switch key {
		case "y", "Y":
			if err := m.save(m.path); err != nil {
				m.promptErr = err.Error()
				return nil
			}
			m.quitting = true
			return tea.Quit
		case "n", "N":
			m.quitting = true
			return tea.Quit
		}
	}
	return nil
}

// save serializes the tree and writes it to path. Output is valid by
// construction, so a successful write is always well-formed JSON.
func (m *Model) save(path string) error {
	out, err := m.root.Marshal("  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// selectChild moves the cursor to the row for the given child of parent.
func (m *Model) selectChild(parent *document.Node, index int) {
	for i, r := range m.rows {
		if r.Parent == parent && r.Index == index {
			m.cursor = i
			m.clampScroll()
			return
		}
	}
}

// typeForKey maps a picker keystroke to a JSON kind.
func typeForKey(key string) (document.Kind, bool) {
	switch key {
	case "s":
		return document.KindString, true
	case "n":
		return document.KindNumber, true
	case "b":
		return document.KindBool, true
	case "u":
		return document.KindNull, true
	case "o":
		return document.KindObject, true
	case "a":
		return document.KindArray, true
	}
	return 0, false
}
