// Package config loads nanoj's user configuration. The config is itself a JSON
// file (fitting for a JSON editor — you can edit it in nanoj), holding display
// preferences and, importantly, accessibility-oriented theming: per-element
// colors and text attributes plus named themes including colorblind-safe and
// monochrome options.
//
// This package is intentionally free of any UI/terminal dependencies. It
// produces plain data; the ui package turns that data into styled output.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Style describes how one kind of element is rendered. All fields are optional;
// the zero value means "inherit / no styling". Colors are terminal color codes:
// "0".."15" for the standard/bright ANSI palette, "0".."255" for 256-color, or
// a hex string like "#ff8800".
type Style struct {
	Fg        string `json:"fg,omitempty"`
	Bg        string `json:"bg,omitempty"`
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
	Faint     bool   `json:"faint,omitempty"`
	Reverse   bool   `json:"reverse,omitempty"`
}

// Config is the full set of user preferences.
type Config struct {
	// DefaultView is "auto" (table for arrays of objects, else tree), "tree",
	// or "table".
	DefaultView string `json:"defaultView,omitempty"`
	// Indent is the per-level indent unit used when saving, e.g. "  " or "\t".
	Indent string `json:"indent,omitempty"`
	// Theme names a built-in base theme: "default", "colorblind",
	// "high-contrast", or "mono".
	Theme string `json:"theme,omitempty"`
	// Styles overrides individual elements on top of the base theme. Keys:
	// key, string, number, bool, null, structure, header, selection.
	Styles map[string]Style `json:"styles,omitempty"`

	// NoColor is resolved from the environment (NO_COLOR / NANOJ_NO_COLOR), not
	// from the file. When set, the UI suppresses all color.
	NoColor bool `json:"-"`
}

// ValidViews and ValidThemes enumerate the accepted enum values.
var (
	ValidViews  = []string{"auto", "tree", "table"}
	ValidThemes = []string{"default", "colorblind", "high-contrast", "mono"}
)

// Default returns the built-in defaults used when no config file is present.
func Default() Config {
	return Config{
		DefaultView: "auto",
		Indent:      "  ",
		Theme:       "default",
	}
}

// Load reads the configuration, layering any file on top of the defaults.
//
// The file path is chosen in this order: the explicit override argument, then
// $NANOJ_CONFIG, then the per-OS user config dir (e.g. ~/.config/nanoj on
// Linux). A missing file is not an error — defaults are returned. A malformed
// file returns the defaults plus an error so the caller can warn but still run.
//
// The returned string is the path that was consulted (useful for messages).
func Load(override string) (Config, string, error) {
	cfg := Default()
	path := ResolvePath(override)

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			// Unmarshalling onto the defaults means absent fields keep their
			// default values — i.e. the file merges over the defaults.
			if jerr := json.Unmarshal(data, &cfg); jerr != nil {
				return Default().withEnv(), path, fmt.Errorf("config %s: %w", path, jerr)
			}
		case !os.IsNotExist(err):
			return cfg.withEnv(), path, fmt.Errorf("config %s: %w", path, err)
		}
	}

	cfg.normalize()
	return cfg.withEnv(), path, nil
}

// ResolvePath returns the config file path that Load would consult.
func ResolvePath(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("NANOJ_CONFIG"); env != "" {
		return env
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "nanoj", "config.json")
	}
	return ""
}

// WriteExample writes a fully-populated example config to path (or the default
// location if path is empty), creating parent directories. It is what
// `nanoj --write-config` produces, so users have something to edit.
func WriteExample(path string) (string, error) {
	if path == "" {
		path = ResolvePath("")
	}
	if path == "" {
		return "", fmt.Errorf("could not determine a config path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(example(), "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// example is the annotated-by-structure config written by WriteExample. It
// spells out the default theme's styles so users can see what is tweakable.
func example() Config {
	return Config{
		DefaultView: "auto",
		Indent:      "  ",
		Theme:       "default",
		Styles: map[string]Style{
			"key":       {Fg: "4"},
			"string":    {Fg: "2"},
			"number":    {Fg: "3"},
			"bool":      {Fg: "5"},
			"null":      {Fg: "8", Italic: true},
			"structure": {Fg: "8"},
			"header":    {Fg: "4", Bold: true},
			"selection": {Reverse: true},
		},
	}
}

// normalize repairs out-of-range enum values so a typo can't break the UI.
func (c *Config) normalize() {
	if !contains(ValidViews, c.DefaultView) {
		c.DefaultView = "auto"
	}
	if !contains(ValidThemes, c.Theme) {
		c.Theme = "default"
	}
	if c.Indent == "" {
		c.Indent = "  "
	}
}

func (c Config) withEnv() Config {
	// https://no-color.org/ — respect the cross-tool NO_COLOR convention, plus
	// a nanoj-specific override.
	if os.Getenv("NO_COLOR") != "" || os.Getenv("NANOJ_NO_COLOR") != "" {
		c.NoColor = true
	}
	return c
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
