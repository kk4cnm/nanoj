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
	"github.com/kk4cnm/nanoj/internal/schema"
	"github.com/kk4cnm/nanoj/internal/ui"
)

func main() {
	configPath := flag.String("config", "", "path to a config file (overrides the default location)")
	writeConfig := flag.Bool("write-config", false, "write an example config file and exit")
	lenient := flag.Bool("lenient", false, "tolerate // and /* */ comments and trailing commas (JSONC) when reading; comments are dropped on save")
	schemaPath := flag.String("schema", "", "validate against a JSON Schema and show a read-only overlay (invalid values, required fields, enums, descriptions)")
	diffPath := flag.String("diff", "", "compare against a baseline JSON file and show a read-only diff overlay")
	readOnly := flag.Bool("view", false, "read-only mode: browse and search without any chance of editing or saving")
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

	// A broken schema or missing baseline shouldn't stop the user from editing:
	// warn and carry on without that overlay.
	if *schemaPath != "" {
		if checker, err := schema.Load(*schemaPath); err != nil {
			fmt.Fprintf(os.Stderr, "nanoj: ignoring schema %s: %v\n", *schemaPath, err)
		} else {
			model = model.WithSchema(checker)
		}
	}
	if *diffPath != "" {
		if baseline, err := loadJSON(*diffPath); err != nil {
			fmt.Fprintf(os.Stderr, "nanoj: ignoring diff baseline %s: %v\n", *diffPath, err)
		} else {
			model = model.WithDiff(baseline, baseName(*diffPath))
		}
	}
	if *readOnly {
		model = model.ReadOnly()
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %v\n", err)
		os.Exit(1)
	}
}

// loadJSON parses a strict-JSON file into a document tree (used for the diff
// baseline).
func loadJSON(path string) (*document.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return document.Parse(f)
}

// baseName returns the final path element, for compact display in the title.
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
