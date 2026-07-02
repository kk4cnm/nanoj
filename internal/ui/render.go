package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kk4cnm/nanoj/internal/document"
)

// scalarText renders a non-container value as display text. Strings are quoted
// and escaped; numbers keep their original text; booleans and null are
// literal. This is for display only — saving goes through document.Marshal.
func scalarText(n *document.Node) string {
	switch n.Kind {
	case document.KindString:
		return strconv.Quote(n.Str)
	case document.KindNumber:
		return n.Num.String()
	case document.KindBool:
		if n.Bool {
			return "true"
		}
		return "false"
	case document.KindNull:
		return "null"
	default:
		return ""
	}
}

// rowDecor is an overlay marker for a row: a single glyph (✗ / + / ~) drawn in
// a left gutter, with the style to render it. The zero value means no marker.
type rowDecor struct {
	marker string
	style  lipgloss.Style
}

// renderRow produces the styled string for a single visible row.
//
// indent shows nesting; a disclosure marker (▼/▸) precedes containers and is
// replaced by blank space for scalars so labels line up. Objects and arrays
// show their delimiter when expanded and a collapsed summary (with child
// count) when not.
//
// When gutter is true a two-column marker gutter is reserved on the left (used
// by the schema and diff overlays); it is omitted otherwise so the plain view
// looks exactly as before.
func renderRow(theme Theme, r Row, expanded, selected bool, decor rowDecor, gutter bool) string {
	var b strings.Builder

	if gutter {
		if decor.marker != "" {
			b.WriteString(decor.style.Render(decor.marker))
			b.WriteByte(' ')
		} else {
			b.WriteString("  ")
		}
	}

	b.WriteString(strings.Repeat("  ", r.Depth))

	if r.Node.IsContainer() {
		if expanded {
			b.WriteString(theme.Structure.Render("▼ "))
		} else {
			b.WriteString(theme.Structure.Render("▸ "))
		}
	} else {
		b.WriteString("  ")
	}

	if r.HasKey {
		b.WriteString(theme.Key.Render(strconv.Quote(r.Key)))
		b.WriteString(theme.Structure.Render(": "))
	}

	switch r.Node.Kind {
	case document.KindObject, document.KindArray:
		b.WriteString(theme.Structure.Render(containerLabel(r.Node, expanded)))
	case document.KindString:
		b.WriteString(theme.String.Render(scalarText(r.Node)))
	case document.KindNumber:
		b.WriteString(theme.Number.Render(scalarText(r.Node)))
	case document.KindBool:
		b.WriteString(theme.Bool.Render(scalarText(r.Node)))
	case document.KindNull:
		b.WriteString(theme.Null.Render(scalarText(r.Node)))
	}

	line := b.String()
	if selected {
		// Reverse-video the whole line, including trailing space, so the
		// selection reads as a full-width bar.
		return theme.Selection.Render(line)
	}
	return line
}

// containerLabel returns the bracket text for an object or array: the opening
// delimiter when expanded, or a collapsed summary with the child count.
func containerLabel(n *document.Node, expanded bool) string {
	open, close := "{", "}"
	if n.Kind == document.KindArray {
		open, close = "[", "]"
	}
	if expanded {
		return open
	}
	count := n.ChildCount()
	noun := "items"
	if count == 1 {
		noun = "item"
	}
	return fmt.Sprintf("%s…%s  (%d %s)", open, close, count, noun)
}
