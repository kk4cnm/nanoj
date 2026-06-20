package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kk4cnm/nanoj/internal/document"
)

// mode is the model's top-level input mode. In modeNormal keystrokes navigate
// and trigger edits; in modeInput a nano-style text prompt is collecting a
// value; in modeChoice the prompt is waiting for a single-key answer (a type
// pick or a yes/no confirmation).
type mode int

const (
	modeNormal mode = iota
	modeInput
	modeChoice
)

// viewMode selects which rendering of the document is on screen.
type viewMode int

const (
	viewTree  viewMode = iota // collapsible outline (default, works for any JSON)
	viewTable                 // spreadsheet grid for an array of objects
)

// action records what the active prompt will do once answered.
type action int

const (
	actNone action = iota
	actEditValue
	actAddKey
	actSaveAs
	actSearch
	actTypePick
	actConfirmDelete
	actConfirmQuit
)

// snapshot is a point-in-time copy of the editable state for undo/redo. The
// tree is deep-cloned so history entries are independent of later edits.
type snapshot struct {
	root      *document.Node
	collapsed map[string]bool
	cursor    int
	dirty     bool
}

// Model is the Bubble Tea model for the tree view.
//
// The document tree (root) is the source of truth. rows is a cache of the
// currently visible lines, recomputed whenever the tree or the expansion set
// changes. cursor indexes into rows; offset is the first visible row when the
// document is taller than the viewport.
type Model struct {
	root      *document.Node
	path      string          // file path, shown in the title bar and used by save
	collapsed map[string]bool // structural paths that are collapsed (default: expanded)

	rows   []Row
	cursor int
	offset int

	width  int
	height int

	status   string // transient message shown in the help area
	dirty    bool   // unsaved changes
	quitting bool

	undoStack  []snapshot
	redoStack  []snapshot
	lastSearch string

	// Table view state (used when view == viewTable).
	view        viewMode
	table       tableShape
	tableRow    int
	tableCol    int
	tableRowOff int
	tableColOff int

	editTarget *document.Node // node being edited by the value prompt

	// Prompt state (used in modeInput / modeChoice).
	mode        mode
	action      action
	input       textinput.Model
	prompt      string         // label shown before the input or choices
	promptErr   string         // validation error shown alongside the prompt
	choiceHint  string         // key hints shown in modeChoice
	pendingNode *document.Node // container being added to (actAddKey)
}

// New builds a Model for the given parsed document and file path.
func New(root *document.Node, path string) Model {
	ti := textinput.New()
	ti.Prompt = "" // we render our own label before the field
	m := Model{
		root:      root,
		path:      path,
		collapsed: map[string]bool{},
		input:     ti,
	}
	m.rebuild()
	// Auto-detect: if the whole document is an array of objects, open straight
	// into the table view, which is the more natural way to read tabular data.
	if shape, ok := tabularShape(root); ok {
		m.view = viewTable
		m.table = shape
	}
	return m
}

// rebuild recomputes the visible rows after a structural or expansion change,
// keeping the cursor within bounds.
func (m *Model) rebuild() {
	m.rows = Flatten(m.root, m.collapsed)
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) Init() tea.Cmd { return nil }

// Update routes input based on the current mode.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if w := m.width - 24; w > 0 {
			m.input.Width = w
		}
		m.clampScroll()
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeInput:
			return m.updateInput(msg)
		case modeChoice:
			return m.updateChoice(msg)
		default:
			if m.view == viewTable {
				return m.updateTable(msg)
			}
			return m.updateNormal(msg)
		}

	default:
		// Route timer/blink messages to the text input while it is active.
		if m.mode == modeInput {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// updateNormal handles navigation and the keys that start an edit. Navigation
// mirrors nano where an analog exists (Ctrl-P/Ctrl-N); arrows and hjkl work
// too. Expand/collapse uses the intuitive left/right arrows.
func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m
	p.status = ""
	switch msg.String() {
	case "ctrl+x":
		return m, p.beginQuit()
	case "ctrl+o":
		p.beginSave()
		return m, textinput.Blink
	case "up", "ctrl+p", "k":
		p.moveCursor(-1)
	case "down", "ctrl+n", "j":
		p.moveCursor(1)
	case "right", "l":
		p.expandCurrent()
	case "left", "h":
		p.collapseOrParent()
	case "enter":
		p.activateCurrent()
		if p.mode == modeInput {
			return m, textinput.Blink
		}
	case " ":
		p.toggleCurrent()
	case "t":
		p.beginChangeType()
	case "a":
		p.beginAddNode()
		if p.mode == modeInput {
			return m, textinput.Blink
		}
	case "ctrl+k":
		p.beginDelete()
	case "ctrl+w":
		p.beginSearch()
		return m, textinput.Blink
	case "ctrl+t":
		p.enterTable()
	case "alt+u", "ctrl+z":
		p.undo()
	case "alt+e", "ctrl+y":
		p.redo()
	case "home", "ctrl+a":
		p.cursor = 0
		p.clampScroll()
	case "end", "ctrl+e":
		p.cursor = len(p.rows) - 1
		p.clampScroll()
	}
	return m, nil
}

// updateInput feeds keystrokes to the text prompt, committing on Enter and
// cancelling on Esc / Ctrl-C.
func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m, (&m).commitInput()
	case "esc", "ctrl+c":
		(&m).endPrompt()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// updateChoice handles single-key answers for type picks and confirmations.
func (m Model) updateChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		(&m).endPrompt()
		return m, nil
	}
	return m, (&m).handleChoice(msg.String())
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	m.clampScroll()
}

// isExpanded reports whether the container at the given path is expanded.
func (m *Model) isExpanded(path string) bool { return !m.collapsed[path] }

// expandCurrent expands the selected container, or moves to its first child if
// it is already expanded — the natural "go deeper" motion on the right arrow.
func (m *Model) expandCurrent() {
	if len(m.rows) == 0 {
		return
	}
	cur := m.rows[m.cursor]
	if !cur.Node.IsContainer() {
		return
	}
	if m.isExpanded(cur.Path) {
		if cur.Node.ChildCount() > 0 {
			m.moveCursor(1)
		}
		return
	}
	delete(m.collapsed, cur.Path)
	m.rebuild()
	m.clampScroll()
}

// collapseOrParent collapses the selected container if expanded; otherwise it
// jumps to the parent row — the natural "back out" motion on the left arrow.
func (m *Model) collapseOrParent() {
	if len(m.rows) == 0 {
		return
	}
	cur := m.rows[m.cursor]
	if cur.Node.IsContainer() && m.isExpanded(cur.Path) {
		m.collapsed[cur.Path] = true
		m.rebuild()
		m.clampScroll()
		return
	}
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].Depth < cur.Depth {
			m.cursor = i
			m.clampScroll()
			return
		}
	}
}

// toggleCurrent flips the expansion of the selected container.
func (m *Model) toggleCurrent() {
	if len(m.rows) == 0 {
		return
	}
	cur := m.rows[m.cursor]
	if !cur.Node.IsContainer() {
		return
	}
	if m.collapsed[cur.Path] {
		delete(m.collapsed, cur.Path)
	} else {
		m.collapsed[cur.Path] = true
	}
	m.rebuild()
	m.clampScroll()
}

// activateCurrent is the Enter action: toggle a container, toggle a boolean, or
// open a value-edit prompt for a string/number. Null has no value to edit.
func (m *Model) activateCurrent() {
	if len(m.rows) == 0 {
		return
	}
	n := m.rows[m.cursor].Node
	switch {
	case n.IsContainer():
		m.toggleCurrent()
	case n.Kind == document.KindBool:
		m.pushUndo()
		n.Bool = !n.Bool
		m.dirty = true
	case n.Kind == document.KindString || n.Kind == document.KindNumber:
		m.beginEditValue(n)
	default: // null
		m.status = "null has no value to edit — press t to change its type"
	}
}

// viewportHeight is the number of rows available for the tree, leaving lines
// for the title bar and the two-line status/help bar.
func (m Model) viewportHeight() int {
	h := m.height - chromeHeight
	if h < 1 {
		return 1
	}
	return h
}

// clampScroll keeps the cursor inside the visible window, scrolling as needed.
func (m *Model) clampScroll() {
	vh := m.viewportHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vh {
		m.offset = m.cursor - vh + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

const chromeHeight = 3 // title line + 2 help/status lines

// View renders the title bar, the visible slice of tree rows, and the
// nano-style status/help bar.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.view == viewTable {
		return m.renderTable()
	}
	var b strings.Builder

	b.WriteString(m.titleBar())
	b.WriteByte('\n')

	vh := m.viewportHeight()
	end := m.offset + vh
	if end > len(m.rows) {
		end = len(m.rows)
	}
	shown := 0
	for i := m.offset; i < end; i++ {
		r := m.rows[i]
		b.WriteString(renderRow(r, m.isExpanded(r.Path), i == m.cursor))
		b.WriteByte('\n')
		shown++
	}
	for ; shown < vh; shown++ {
		b.WriteByte('\n')
	}

	b.WriteString(m.statusBar())
	return b.String()
}

func (m Model) titleBar() string {
	name := m.path
	if name == "" {
		name = "[new file]"
	}
	mark := ""
	if m.dirty {
		mark = "*"
	}
	title := fmt.Sprintf(" nanoj — %s%s ", mark, name)
	pos := ""
	if m.view == viewTable {
		pos = fmt.Sprintf(" r%d c%d ", m.tableRow+1, m.tableCol+1)
	} else if len(m.rows) > 0 {
		pos = fmt.Sprintf(" %d/%d ", m.cursor+1, len(m.rows))
	}
	bar := lipgloss.NewStyle().Reverse(true).Bold(true)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(pos)
	if gap < 0 {
		gap = 0
	}
	return bar.Render(title + strings.Repeat(" ", gap) + pos)
}

// statusBar renders the two bottom lines: a prompt when one is active,
// otherwise nano-style shortcut hints. Keys are shown the nano way: ^X = Ctrl-X.
func (m Model) statusBar() string {
	switch m.mode {
	case modeInput:
		line1 := m.prompt + m.input.View()
		if m.promptErr != "" {
			line1 += "  [" + m.promptErr + "]"
		}
		return pad(" "+line1, m.width) + "\n" + pad(" Enter Confirm    ^C Cancel", m.width)
	case modeChoice:
		line1 := m.prompt + "  " + m.choiceHint
		line2 := " ^C Cancel"
		if m.promptErr != "" {
			line2 = " [" + m.promptErr + "]"
		}
		return pad(" "+line1, m.width) + "\n" + pad(line2, m.width)
	default:
		var line1 string
		if m.status != "" {
			line1 = lipgloss.NewStyle().Reverse(true).Render(pad(" "+m.status, m.width))
		} else {
			line1 = pad(" "+strings.Join([]string{"^X Exit", "^O Write", "^W Search", "Enter Edit", "t Type"}, "    "), m.width)
		}
		line2 := pad(" "+strings.Join([]string{"a Add", "^K Delete", "M-U Undo", "^T Table", "→/← Expand/Collapse"}, "    "), m.width)
		return line1 + "\n" + line2
	}
}

// pad right-pads s with spaces to width w (used to fill status lines).
func pad(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}
