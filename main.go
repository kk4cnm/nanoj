// Command nanoj is a terminal app for viewing and editing JSON safely.
//
// The user navigates and edits a typed tree built from the file; the only path
// back to disk is a serializer that always emits well-formed JSON, so it is
// structurally impossible to save malformed output.
//
// Usage:
//
//	nanoj <file.json>
//
// Phase 1 provides a keyboard-driven, collapsible tree view (read-only for
// now; in-place editing and save land next).
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
	"github.com/kk4cnm/nanoj/internal/ui"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: nanoj <file.json>")
		os.Exit(2)
	}
	path := os.Args[1]

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %v\n", err)
		os.Exit(1)
	}
	doc, err := document.Parse(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %s is not valid JSON: %v\n", path, err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(doc, path), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %v\n", err)
		os.Exit(1)
	}
}
