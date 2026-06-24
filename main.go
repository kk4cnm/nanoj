// Command nanoj is a terminal app for viewing and editing JSON safely.
//
// The user navigates and edits a typed tree built from the file; the only path
// back to disk is a serializer that always emits well-formed JSON, so it is
// structurally impossible to save malformed output.
//
// Usage:
//
//	nanoj [flags] <file.json>
//
// Flags:
//
//	--config <path>   use a specific config file
//	--write-config    write an example config to the default location and exit
//
// Configuration (theme, colors, default view, indent) lives in a JSON file; see
// `nanoj --write-config`. The NO_COLOR environment variable is honored.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/config"
	"github.com/kk4cnm/nanoj/internal/document"
	"github.com/kk4cnm/nanoj/internal/ui"
)

func main() {
	configPath := flag.String("config", "", "path to a config file (overrides the default location)")
	writeConfig := flag.Bool("write-config", false, "write an example config file and exit")
	lenient := flag.Bool("lenient", false, "tolerate // and /* */ comments and trailing commas (JSONC) when reading; comments are dropped on save")
	flag.Parse()

	if *writeConfig {
		written, err := config.WriteExample(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nanoj: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote example config to %s\n", written)
		return
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: nanoj [flags] <file.json>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	path := flag.Arg(0)

	cfg, cfgPath, err := config.Load(*configPath)
	if err != nil {
		// A broken config shouldn't stop the user from editing; warn and use
		// the defaults that Load returned.
		fmt.Fprintf(os.Stderr, "nanoj: ignoring config %s: %v\n", cfgPath, err)
	}

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %v\n", err)
		os.Exit(1)
	}
	var (
		doc             *document.Node
		commentsDropped bool
	)
	if *lenient {
		doc, commentsDropped, err = document.ParseLenient(f)
	} else {
		doc, err = document.Parse(f)
	}
	f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %s is not valid JSON: %v\n", path, err)
		os.Exit(1)
	}

	model := ui.NewWithConfig(doc, path, cfg)
	if commentsDropped {
		model = model.WithCommentWarning()
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %v\n", err)
		os.Exit(1)
	}
}
