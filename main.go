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
//	--from <format>   convert yaml/toml input into a JSON working copy
//
// Configuration (theme, colors, default view, indent) lives in a JSON file; see
// `nanoj --write-config`. The NO_COLOR environment variable is honored.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/config"
	"github.com/kk4cnm/nanoj/internal/convert"
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
	fromFormat := flag.String("from", "", "convert the input from another format (yaml or toml) and open it as a JSON working copy; saving writes JSON and never touches the source file")
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
	switch *fromFormat {
	case "":
		if *lenient {
			doc, commentsDropped, err = document.ParseLenient(f)
		} else {
			doc, err = document.Parse(f)
		}
		if err != nil {
			err = fmt.Errorf("%s is not valid JSON: %w", path, err)
		}
	case "yaml", "yml":
		doc, err = convert.FromYAML(f)
	case "toml":
		doc, err = convert.FromTOML(f)
	default:
		fmt.Fprintf(os.Stderr, "nanoj: --from must be yaml or toml (got %q)\n", *fromFormat)
		os.Exit(2)
	}
	f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nanoj: %v\n", err)
		os.Exit(1)
	}

	// A converted document is a working copy: the buffer points at a .json path
	// beside the source, and the source file itself is never written.
	workPath := path
	if *fromFormat != "" {
		workPath = jsonWorkPath(path)
	}

	model := ui.NewWithConfig(doc, workPath, cfg)
	if commentsDropped {
		model = model.WithCommentWarning()
	}
	if *fromFormat != "" {
		model = model.WithConversionNote(*fromFormat)
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

// jsonWorkPath derives the JSON working-copy path for a converted file:
// the source path with its extension replaced by .json.
func jsonWorkPath(path string) string {
	base := baseName(path)
	if i := strings.LastIndexByte(base, '.'); i > 0 {
		return path[:len(path)-len(base)+i] + ".json"
	}
	return path + ".json"
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
