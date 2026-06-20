package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kk4cnm/nanoj/internal/config"
)

func TestBuildThemeDefault(t *testing.T) {
	th := BuildTheme(config.Default())
	if th.Key.GetForeground() != lipgloss.Color("4") {
		t.Errorf("default key color should be 4, got %v", th.Key.GetForeground())
	}
	if !th.Header.GetBold() {
		t.Error("default header should be bold")
	}
}

func TestBuildThemeUnknownFallsBack(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "nonexistent"
	th := BuildTheme(cfg)
	// Falls back to default palette.
	if th.String.GetForeground() != lipgloss.Color("2") {
		t.Errorf("unknown theme should fall back to default, got %v", th.String.GetForeground())
	}
}

func TestColorblindThemeUsesAttributes(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "colorblind"
	th := BuildTheme(cfg)
	// Bool is distinguished by both a color and bold, so it doesn't rely on hue
	// alone — the whole point of the accessible theme.
	if !th.Bool.GetBold() {
		t.Error("colorblind bool style should be bold for redundancy")
	}
}

func TestNoColorForcesMonoAndStripsColor(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "colorblind" // would normally be colorful
	cfg.NoColor = true
	th := BuildTheme(cfg)
	if th.Key.GetForeground() != (lipgloss.NoColor{}) {
		t.Errorf("NoColor should remove foreground color, got %v", th.Key.GetForeground())
	}
	// Mono still distinguishes via attributes.
	if !th.Key.GetBold() {
		t.Error("mono key should still be bold")
	}
	if !th.Number.GetUnderline() {
		t.Error("mono number should be underlined")
	}
}

func TestStyleOverrideApplied(t *testing.T) {
	cfg := config.Default()
	cfg.Styles = map[string]config.Style{
		"string": {Fg: "201", Italic: true},
	}
	th := BuildTheme(cfg)
	if th.String.GetForeground() != lipgloss.Color("201") {
		t.Errorf("override fg not applied, got %v", th.String.GetForeground())
	}
	if !th.String.GetItalic() {
		t.Error("override italic not applied")
	}
	// Unmentioned elements keep the base theme.
	if th.Number.GetForeground() != lipgloss.Color("3") {
		t.Errorf("non-overridden number should keep base color, got %v", th.Number.GetForeground())
	}
}

func TestNoColorStripsUserColorOverride(t *testing.T) {
	cfg := config.Default()
	cfg.NoColor = true
	cfg.Styles = map[string]config.Style{
		"string": {Fg: "201", Bold: true},
	}
	th := BuildTheme(cfg)
	if th.String.GetForeground() != (lipgloss.NoColor{}) {
		t.Error("NoColor should strip even user-specified colors")
	}
	if !th.String.GetBold() {
		t.Error("NoColor should keep non-color attributes from overrides")
	}
}
