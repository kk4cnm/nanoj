package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// Render without ANSI color/styling so View output is plain text we can
	// assert on directly.
	lipgloss.SetColorProfile(termenv.Ascii)
}

func sized(m Model, w, h int) Model {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(Model)
}

// TestViewRendersStructure checks that the rendered tree contains the title,
// keys, values, disclosure markers, and the nano-style help line.
func TestViewRendersStructure(t *testing.T) {
	m := New(parse(t, `{"name": "Ada", "nums": [1, 2]}`), "people.json")
	m = sized(m, 80, 24)
	out := m.View()

	wants := []string{
		"nanoj — people.json", // title bar
		`"name"`,              // object key
		`"Ada"`,               // string value
		`"nums"`,              // array key
		"▼",                   // expanded container marker
		"^X Exit",             // nano-style help
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("View output missing %q\n---\n%s", w, out)
		}
	}
}

// TestViewCollapsedSummary checks the collapsed container shows a child count.
func TestViewCollapsedSummary(t *testing.T) {
	m := New(parse(t, `{"nums": [1, 2, 3]}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down")  // onto "nums"
	m = sendKey(m, "enter") // collapse
	out := m.View()
	if !strings.Contains(out, "(3 items)") {
		t.Errorf("expected collapsed summary '(3 items)', got:\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Errorf("expected collapsed marker '▸', got:\n%s", out)
	}
}
