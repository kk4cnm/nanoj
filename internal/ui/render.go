package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kk4cnm/nanoj/internal/document"
)

// Styles used by the tree view. Colors are chosen from the 8/16-color ANSI
// palette so they adapt to the user's terminal theme rather than fighting it.
var (
	styleKey       = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	styleString    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleNumber    = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	styleBool      = lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // magenta
	styleNull      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // gray
	styleStructure = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // brackets/markers
	styleSelected  = lipgloss.NewStyle().Reverse(true)
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

// renderRow produces the styled string for a single visible row.
//
// indent shows nesting; a disclosure marker (▼/▸) precedes containers and is
// replaced by blank space for scalars so labels line up. Objects and arrays
// show their delimiter when expanded and a collapsed summary (with child
// count) when not.
func renderRow(r Row, expanded, selected bool) string {
	var b strings.Builder

	b.WriteString(strings.Repeat("  ", r.Depth))

	if r.Node.IsContainer() {
		if expanded {
			b.WriteString(styleStructure.Render("▼ "))
		} else {
			b.WriteString(styleStructure.Render("▸ "))
		}
	} else {
		b.WriteString("  ")
	}

	if r.HasKey {
		b.WriteString(styleKey.Render(strconv.Quote(r.Key)))
		b.WriteString(styleStructure.Render(": "))
	}

	switch r.Node.Kind {
	case document.KindObject, document.KindArray:
		b.WriteString(styleStructure.Render(containerLabel(r.Node, expanded)))
	case document.KindString:
		b.WriteString(styleString.Render(scalarText(r.Node)))
	case document.KindNumber:
		b.WriteString(styleNumber.Render(scalarText(r.Node)))
	case document.KindBool:
		b.WriteString(styleBool.Render(scalarText(r.Node)))
	case document.KindNull:
		b.WriteString(styleNull.Render(scalarText(r.Node)))
	}

	line := b.String()
	if selected {
		// Reverse-video the whole line, including trailing space, so the
		// selection reads as a full-width bar.
		return styleSelected.Render(line)
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
