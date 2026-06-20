package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kk4cnm/nanoj/internal/config"
)

// Theme holds the resolved lipgloss styles for every element nanoj draws. It is
// built from the user's config so that colors and text attributes — the things
// that matter for accessibility — are fully customizable.
type Theme struct {
	Key       lipgloss.Style
	String    lipgloss.Style
	Number    lipgloss.Style
	Bool      lipgloss.Style
	Null      lipgloss.Style
	Structure lipgloss.Style
	Header    lipgloss.Style
	Selection lipgloss.Style
}

// builtinThemes are the named base palettes. Beyond color, the accessible
// themes (colorblind, high-contrast, mono) lean on text attributes — bold,
// italic, underline, faint — so element types stay distinguishable even when
// hue is hard or impossible to perceive.
var builtinThemes = map[string]map[string]config.Style{
	"default": {
		"key":       {Fg: "4"},
		"string":    {Fg: "2"},
		"number":    {Fg: "3"},
		"bool":      {Fg: "5"},
		"null":      {Fg: "8", Italic: true},
		"structure": {Fg: "8"},
		"header":    {Fg: "4", Bold: true},
		"selection": {Reverse: true},
	},
	// Okabe–Ito-inspired palette (designed to be distinguishable across the
	// common forms of color blindness), reinforced with bold/italic so the
	// distinction never relies on color alone.
	"colorblind": {
		"key":       {Fg: "27", Bold: true},  // blue
		"string":    {Fg: "36"},              // bluish green
		"number":    {Fg: "208"},             // orange
		"bool":      {Fg: "170", Bold: true}, // reddish purple
		"null":      {Fg: "245", Italic: true},
		"structure": {Fg: "245"},
		"header":    {Fg: "27", Bold: true, Underline: true},
		"selection": {Reverse: true},
	},
	// Bright, saturated colors plus weight for low-vision / high-contrast needs.
	"high-contrast": {
		"key":       {Fg: "12", Bold: true},   // bright blue
		"string":    {Fg: "10", Bold: true},   // bright green
		"number":    {Fg: "11", Bold: true},   // bright yellow
		"bool":      {Fg: "13", Bold: true},   // bright magenta
		"null":      {Fg: "15", Italic: true}, // bright white
		"structure": {Fg: "15"},
		"header":    {Fg: "15", Bold: true, Reverse: true},
		"selection": {Reverse: true, Bold: true},
	},
	// No color at all: element types are told apart purely by text attributes,
	// for monochrome terminals or anyone who prefers/needs no color.
	"mono": {
		"key":       {Bold: true},
		"string":    {},
		"number":    {Underline: true},
		"bool":      {Italic: true},
		"null":      {Faint: true, Italic: true},
		"structure": {Faint: true},
		"header":    {Bold: true, Underline: true},
		"selection": {Reverse: true},
	},
}

// BuildTheme resolves a Theme from the configuration: a named base palette with
// the user's per-element overrides applied. When NoColor is set, the mono base
// is used and all color is suppressed (text attributes are kept).
func BuildTheme(cfg config.Config) Theme {
	baseName := cfg.Theme
	if cfg.NoColor {
		baseName = "mono"
	}
	base := builtinThemes[baseName]
	if base == nil {
		base = builtinThemes["default"]
	}

	merged := make(map[string]config.Style, len(base))
	for k, v := range base {
		merged[k] = v
	}
	// User overrides apply on top of the base, except when NoColor forces mono —
	// then we honor only the user's non-color attributes.
	for k, v := range cfg.Styles {
		if cfg.NoColor {
			v.Fg, v.Bg = "", ""
		}
		merged[k] = v
	}

	get := func(name string) lipgloss.Style {
		return toLipgloss(merged[name], cfg.NoColor)
	}
	return Theme{
		Key:       get("key"),
		String:    get("string"),
		Number:    get("number"),
		Bool:      get("bool"),
		Null:      get("null"),
		Structure: get("structure"),
		Header:    get("header"),
		Selection: get("selection"),
	}
}

func toLipgloss(s config.Style, noColor bool) lipgloss.Style {
	st := lipgloss.NewStyle()
	if !noColor {
		if s.Fg != "" {
			st = st.Foreground(lipgloss.Color(s.Fg))
		}
		if s.Bg != "" {
			st = st.Background(lipgloss.Color(s.Bg))
		}
	}
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Underline {
		st = st.Underline(true)
	}
	if s.Faint {
		st = st.Faint(true)
	}
	if s.Reverse {
		st = st.Reverse(true)
	}
	return st
}
