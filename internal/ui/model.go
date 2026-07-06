package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/kk4cnm/nanoj/internal/config"
	"github.com/kk4cnm/nanoj/internal/document"
	"github.com/kk4cnm/nanoj/internal/schema"
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
	actSetCell
	actAddKey
	actSaveAs
	actSearch
	actTypePick
	actConfirmQuit
	actEnumPick
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

	rows      []Row
	rowsStale bool // tree rows need rebuilding (deferred while in table view)
	cursor    int
	offset    int

	width  int
	height int

	status          string // transient message shown in the help area
	dirty           bool   // unsaved changes
	quitting        bool
	commentsDropped bool // loaded leniently; comments were removed (warn the user)

	undoStack  []snapshot
	redoStack  []snapshot
	lastSearch string

	// Clipboard for cut/copy/paste. The node is a detached clone; key/hasKey
	// remember the original object key so pasting into an object can reuse it.
	clipboard       *document.Node
	clipboardKey    string
	clipboardHasKey bool

	// Table view state (used when view == viewTable).
	view           viewMode
	table          tableShape
	tablePath      string // structural path of table.array (survives undo clones)
	tableColWidths []int  // cached column widths; recomputed only when the table changes
	tableRow       int
	tableCol       int
	tableRowOff    int
	tableColOff    int
	sortCol        int  // column the table is sorted by, or -1 for none
	sortDesc       bool // sort direction for sortCol

	editTarget *document.Node // node being edited by the value prompt

	theme  Theme  // resolved display styles (from config)
	indent string // per-level indent unit used when saving

	// Read-only mode gates every mutation and disables saving; the document can
	// be browsed and searched but not changed.
	readOnly bool

	// Optional schema overlay (--schema). schema compiles the JSON Schema;
	// schemaOverlay is the current per-path annotation, recomputed after edits.
	schema        *schema.Checker
	schemaOverlay schema.Overlay

	// Optional diff overlay (--diff): the working tree compared to a baseline.
	diffBaseline *document.Node
	diffResult   document.DiffResult
	diffName     string
	hasDiff      bool

	// overlayStale marks that the tree changed and the schema/diff overlays need
	// recomputing before the next render.
	overlayStale bool

	// Enum picker state (actEnumPick): the node being set and the allowed values.
	enumTarget  *document.Node
	enumChoices []any

	// Prompt state (used in modeInput / modeChoice).
	mode        mode
	action      action
	input       textinput.Model
	prompt      string         // label shown before the input or choices
	promptErr   string         // validation error shown alongside the prompt
	choiceHint  string         // key hints shown in modeChoice
	pendingNode *document.Node // container being added to (actAddKey / actSetCell)
	cellKey     string         // column key for a new table cell (actSetCell)
}

// New builds a Model with default configuration. It is a convenience used by
// tests; the program uses NewWithConfig.
func New(root *document.Node, path string) Model {
	return NewWithConfig(root, path, config.Default())
}

// NewWithConfig builds a Model for the given document, file path, and user
// configuration (theme, indent, default view).
func NewWithConfig(root *document.Node, path string, cfg config.Config) Model {
	ti := textinput.New()
	ti.Prompt = "" // we render our own label before the field
	m := Model{
		root:      root,
		path:      path,
		collapsed: map[string]bool{},
		input:     ti,
		theme:     BuildTheme(cfg),
		indent:    cfg.Indent,
		sortCol:   -1,
	}

	// Choose the initial view per config *before* the first rebuild. "table"
	// and "auto" both open the table when the document is an array of objects;
	// "auto" is the default.
	if cfg.DefaultView != "tree" {
		if shape, ok := tabularShape(root); ok {
			m.view = viewTable
			m.table = shape
			m.tablePath = "" // the whole document is the array
			m.recomputeColWidths()
		}
	}

	// rebuild is lazy in table view, so opening a large array of records (the
	// common big-file case) skips flattening the whole tree.
	m.rebuild()
	return m
}

// WithCommentWarning marks the model as having been loaded leniently with
// comments stripped, so it warns the user (in the title and at save time) that
// saving writes strict JSON without those comments.
func (m Model) WithCommentWarning() Model {
	m.commentsDropped = true
	m.status = "comments were removed on load — saving writes strict JSON"
	return m
}

// WithSchema attaches a compiled JSON Schema as a read-only overlay and
// computes the initial annotation. Markers render in both views; when the
// document opened straight into the table, its column widths are recomputed to
// reserve marker space.
func (m Model) WithSchema(c *schema.Checker) Model {
	m.schema = c
	m.schemaOverlay = c.Annotate(m.root)
	m.status = m.schemaOverlay.Summary
	if m.view == viewTable {
		m.recomputeColWidths()
	}
	return m
}

// WithDiff attaches a baseline document to compare against, as a read-only
// overlay. name is shown in the title. Like schema, markers render in both
// views.
func (m Model) WithDiff(baseline *document.Node, name string) Model {
	m.diffBaseline = baseline
	m.diffName = name
	m.hasDiff = true
	m.diffResult = document.Diff(baseline, m.root)
	m.status = m.diffSummary()
	if m.view == viewTable {
		m.recomputeColWidths()
	}
	return m
}

// ReadOnly puts the model in read-only mode: navigation and search still work,
// but every edit and save is blocked.
func (m Model) ReadOnly() Model {
	m.readOnly = true
	return m
}

// refreshOverlays recomputes the schema and diff overlays when the tree has
// changed. Called once per key event (after the edit is applied) so a render
// never sees stale annotations.
func (m *Model) refreshOverlays() {
	if !m.overlayStale {
		return
	}
	m.overlayStale = false
	if m.schema != nil {
		m.schemaOverlay = m.schema.Annotate(m.root)
	}
	if m.hasDiff {
		m.diffResult = document.Diff(m.diffBaseline, m.root)
	}
}

// diffSummary is the one-line description of the diff overlay.
func (m Model) diffSummary() string {
	d := m.diffResult
	return fmt.Sprintf("diff vs %s: +%d added  ~%d changed  -%d removed",
		m.diffName, d.Added, d.Changed, d.Removed)
}

// readOnlyBlocked reports read-only mode and sets an explanatory status. Edit
// entry points call it and bail when it returns true.
func (m *Model) readOnlyBlocked() bool {
	if m.readOnly {
		m.status = "read-only mode — editing is disabled"
		return true
	}
	return false
}

// rebuild recomputes the visible tree rows after a structural or expansion
// change. In the table view the tree rows aren't shown, so rebuilding is
// deferred (marked stale) — this avoids materializing a huge row slice for a
// large file that opens straight into the table. ensureRows realizes the rows
// when the tree view actually needs them.
func (m *Model) rebuild() {
	if m.view == viewTable {
		m.rowsStale = true
		return
	}
	m.rows = Flatten(m.root, m.collapsed)
	m.rowsStale = false
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// ensureRows builds the tree rows if a deferred rebuild left them stale.
func (m *Model) ensureRows() {
	if m.rowsStale {
		m.rows = Flatten(m.root, m.collapsed)
		m.rowsStale = false
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
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
		var next tea.Model
		var cmd tea.Cmd
		switch m.mode {
		case modeInput:
			next, cmd = m.updateInput(msg)
		case modeChoice:
			next, cmd = m.updateChoice(msg)
		default:
			if m.view == viewTable {
				next, cmd = m.updateTable(msg)
			} else {
				next, cmd = m.updateNormal(msg)
			}
		}
		// Recompute schema/diff overlays if the edit changed the tree.
		if nm, ok := next.(Model); ok && nm.overlayStale {
			nm.refreshOverlays()
			next = nm
		}
		return next, cmd

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
		p.cutCurrent()
	case "alt+6":
		p.copyCurrent()
	case "ctrl+u":
		p.pasteCurrent()
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
	if n.IsContainer() {
		m.toggleCurrent() // expand/collapse is a view change, allowed read-only
		return
	}
	if m.readOnlyBlocked() {
		return
	}
	// If the schema constrains this field to an enum, offer a pick-list instead
	// of free-text entry (works for any current value type, including null).
	if m.schema != nil {
		if a, ok := m.schemaOverlay.At(m.rows[m.cursor].Path); ok && len(a.EnumValues) > 0 {
			m.beginEnumPick(n, a)
			return
		}
	}
	switch {
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
	gutter := m.hasOverlay()
	shown := 0
	for i := m.offset; i < end; i++ {
		r := m.rows[i]
		b.WriteString(renderRow(m.theme, r, m.isExpanded(r.Path), i == m.cursor, m.decorFor(r), gutter))
		b.WriteByte('\n')
		shown++
	}
	for ; shown < vh; shown++ {
		b.WriteByte('\n')
	}

	b.WriteString(m.statusBar())
	return b.String()
}

// hasOverlay reports whether a schema or diff overlay is active (and thus the
// views should reserve marker space).
func (m Model) hasOverlay() bool { return m.schema != nil || m.hasDiff }

// decorAt returns the overlay marker for the node at the given structural
// path. Schema validity takes precedence over diff status when both are active.
func (m Model) decorAt(path string) rowDecor {
	if m.schema != nil {
		if a, ok := m.schemaOverlay.At(path); ok && a.Invalid {
			return rowDecor{marker: "✗", style: m.theme.Invalid}
		}
	}
	if m.hasDiff {
		switch m.diffResult.Kind(path) {
		case document.DiffAdded:
			return rowDecor{marker: "+", style: m.theme.Added}
		case document.DiffChanged:
			return rowDecor{marker: "~", style: m.theme.Changed}
		}
	}
	return rowDecor{}
}

// decorFor returns the overlay marker for a tree row.
func (m Model) decorFor(r Row) rowDecor { return m.decorAt(r.Path) }

func (m Model) titleBar() string {
	name := m.path
	if name == "" {
		name = "[new file]"
	}
	mark := ""
	if m.dirty {
		mark = "*"
	}
	warn := ""
	if m.commentsDropped {
		warn = "⚠ "
	}
	badges := ""
	if m.readOnly {
		badges += " [read-only]"
	}
	if m.schema != nil {
		if m.schemaOverlay.Valid {
			badges += " [schema ✓]"
		} else {
			badges += " [schema ✗]"
		}
	}
	if m.hasDiff {
		d := m.diffResult
		badges += fmt.Sprintf(" [diff +%d~%d-%d]", d.Added, d.Changed, d.Removed)
	}
	title := fmt.Sprintf(" nanoj — %s%s%s%s ", warn, mark, name, badges)
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
	return bar.Render(pad(title+strings.Repeat(" ", gap)+pos, m.width))
}

// selectedInfo returns a one-line schema/diff description for the selected
// tree row, shown in the status area when there is no transient message.
func (m Model) selectedInfo() string {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}
	return m.overlayInfo(m.rows[m.cursor].Path)
}

// overlayInfo returns a one-line schema/diff description for the node at the
// given structural path. Empty when no overlay applies there.
func (m Model) overlayInfo(path string) string {
	var parts []string

	if m.schema != nil {
		if a, ok := m.schemaOverlay.At(path); ok {
			if a.Invalid {
				parts = append(parts, "✗ invalid")
			}
			if a.Required {
				parts = append(parts, "required")
			}
			if a.ExpectedType != "" {
				parts = append(parts, "type: "+a.ExpectedType)
			}
			if len(a.EnumValues) > 0 {
				parts = append(parts, "enum: "+strings.Join(a.EnumDisplay(), ", "))
			}
			if len(a.MissingRequired) > 0 {
				parts = append(parts, "missing required: "+strings.Join(a.MissingRequired, ", "))
			}
			if a.Description != "" {
				parts = append(parts, a.Description)
			}
		}
	}
	if m.hasDiff {
		switch m.diffResult.Kind(path) {
		case document.DiffAdded:
			parts = append(parts, "+ added vs "+m.diffName)
		case document.DiffChanged:
			parts = append(parts, "~ changed vs "+m.diffName)
		}
	}
	return strings.Join(parts, "  •  ")
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
		switch {
		case m.status != "":
			line1 = lipgloss.NewStyle().Reverse(true).Render(pad(" "+m.status, m.width))
		case m.selectedInfo() != "":
			line1 = lipgloss.NewStyle().Reverse(true).Render(pad(" "+m.selectedInfo(), m.width))
		default:
			line1 = pad(" "+strings.Join([]string{"^X Exit", "^O Write", "^W Search", "Enter Edit", "t Type"}, "    "), m.width)
		}
		line2 := pad(" "+strings.Join([]string{"a Add", "^K Cut", "M-6 Copy", "^U Paste", "M-U Undo", "^T Table"}, "    "), m.width)
		return line1 + "\n" + line2
	}
}

// padBetween lays out left and right on a single line of width w, with right
// kept flush to the right edge. When there isn't room for both, the left text
// is truncated first so the (more important) right text stays visible.
func padBetween(left, right string, w int) string {
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	if gap := w - lw - rw; gap >= 1 {
		return left + strings.Repeat(" ", gap) + right
	}
	avail := w - rw - 1
	if avail < 1 {
		return pad(right, w)
	}
	return ansi.Truncate(left, avail, "") + " " + right
}

// pad makes s exactly w display columns wide: right-padding with spaces when
// short, and truncating (ANSI-aware) when long so a line never wraps.
func pad(s string, w int) string {
	width := lipgloss.Width(s)
	switch {
	case width == w:
		return s
	case width < w:
		return s + strings.Repeat(" ", w-width)
	default:
		return ansi.Truncate(s, w, "")
	}
}
