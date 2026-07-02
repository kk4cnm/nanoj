package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/schema"
)

func loadChecker(t *testing.T, schemaJSON string) *schema.Checker {
	t.Helper()
	p := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(p, []byte(schemaJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := schema.Load(p)
	if err != nil {
		t.Fatalf("schema.Load: %v", err)
	}
	return c
}

// --- read-only mode ---

func TestReadOnlyBlocksEdits(t *testing.T) {
	m := New(parse(t, `{"a": 1}`), "x.json").ReadOnly()
	m = sized(m, 80, 24)

	// Change-type is blocked.
	m = sendKey(m, "t")
	if m.mode != modeNormal {
		t.Errorf("t should be blocked in read-only, mode=%d", m.mode)
	}
	if !strings.Contains(m.status, "read-only") {
		t.Errorf("expected read-only status, got %q", m.status)
	}

	// Add is blocked.
	m = sendKey(m, "a")
	if m.mode != modeNormal {
		t.Errorf("a should be blocked in read-only")
	}

	// Save is blocked (no prompt opens).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(Model)
	if m.mode == modeInput {
		t.Errorf("^O should be blocked in read-only")
	}
}

func TestReadOnlyEnterDoesNotEditScalar(t *testing.T) {
	m := New(parse(t, `{"a": "hi"}`), "x.json").ReadOnly()
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // select the "a" value
	m = sendKey(m, "enter")
	if m.mode == modeInput {
		t.Errorf("enter on a scalar should not open an edit prompt in read-only")
	}
}

func TestReadOnlyNavigationStillWorks(t *testing.T) {
	m := New(parse(t, `{"a": 1, "b": 2}`), "x.json").ReadOnly()
	m = sized(m, 80, 24)
	start := m.cursor
	m = sendKey(m, "down")
	if m.cursor == start {
		t.Errorf("navigation should still work in read-only")
	}
}

func TestReadOnlyTitleBadge(t *testing.T) {
	m := New(parse(t, `{"a": 1}`), "x.json").ReadOnly()
	m = sized(m, 80, 24)
	if !strings.Contains(m.titleBar(), "[read-only]") {
		t.Errorf("title should show [read-only]")
	}
}

// --- schema overlay ---

func TestSchemaMarksInvalidRow(t *testing.T) {
	c := loadChecker(t, `{"type":"object","properties":{"age":{"type":"number"}}}`)
	m := New(parse(t, `{"age": "old"}`), "x.json").WithSchema(c)
	m = sized(m, 80, 24)

	if !m.hasOverlay() {
		t.Fatal("schema should activate the overlay gutter")
	}
	if d := m.decorFor(Row{Path: "/0"}); d.marker != "✗" {
		t.Errorf("expected invalid marker on /0, got %q", d.marker)
	}
	if !strings.Contains(m.titleBar(), "[schema ✗]") {
		t.Errorf("title should show schema invalid badge, got %q", m.titleBar())
	}
}

func TestSchemaValidBadge(t *testing.T) {
	c := loadChecker(t, `{"type":"object","properties":{"age":{"type":"number"}}}`)
	m := New(parse(t, `{"age": 36}`), "x.json").WithSchema(c)
	m = sized(m, 80, 24)
	if !strings.Contains(m.titleBar(), "[schema ✓]") {
		t.Errorf("title should show schema valid badge, got %q", m.titleBar())
	}
	if d := m.decorFor(Row{Path: "/0"}); d.marker != "" {
		t.Errorf("valid row should have no marker, got %q", d.marker)
	}
}

func TestSchemaForcesTreeView(t *testing.T) {
	// An array of objects would normally auto-open as a table; a schema overlay
	// switches it to the tree so the markers are visible.
	c := loadChecker(t, `{"type":"array","items":{"type":"object"}}`)
	m := New(parse(t, `[{"a":1},{"a":2}]`), "x.json").WithSchema(c)
	if m.view != viewTree {
		t.Errorf("schema overlay should force the tree view, got view=%d", m.view)
	}
}

func TestSchemaOverlayRecomputesAfterEdit(t *testing.T) {
	c := loadChecker(t, `{"type":"object","properties":{"name":{"type":"string"}},"required":["name","email"]}`)
	m := New(parse(t, `{"name": "Ada"}`), "x.json").WithSchema(c)
	m = sized(m, 80, 24)
	if m.schemaOverlay.Valid {
		t.Fatal("missing required email should be invalid")
	}
	// Add the missing key: select root, press a, type "email", commit.
	m = sendKey(m, "a")
	if m.mode != modeInput {
		t.Fatalf("add should open a key prompt, mode=%d", m.mode)
	}
	m = typeText(m, "email")
	m = sendKey(m, "enter")
	// email is now present (as null) → still needs to be a string, but the
	// required-key error is resolved; overlay must have been recomputed.
	root, _ := m.schemaOverlay.At("")
	for _, k := range root.MissingRequired {
		if k == "email" {
			t.Errorf("email should no longer be reported missing after the edit")
		}
	}
}

func TestSchemaEnumPick(t *testing.T) {
	c := loadChecker(t, `{"type":"object","properties":{"level":{"enum":["low","high"]}}}`)
	m := New(parse(t, `{"level": "low"}`), "x.json").WithSchema(c)
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // select the level value
	m = sendKey(m, "enter")
	if m.mode != modeChoice || m.action != actEnumPick {
		t.Fatalf("enter on an enum field should open the enum picker, mode=%d action=%d", m.mode, m.action)
	}
	m = sendKey(m, "2") // pick "high"
	if got := m.root.Members[0].Value.Str; got != "high" {
		t.Errorf("enum pick should set value to high, got %q", got)
	}
	if m.mode != modeNormal {
		t.Errorf("picker should close after a choice")
	}
}

// --- diff overlay ---

func TestDiffMarksAddedAndChanged(t *testing.T) {
	baseline := parse(t, `{"a": 1, "b": 2}`)
	m := New(parse(t, `{"a": 1, "b": 3, "c": 4}`), "x.json").WithDiff(baseline, "base.json")
	m = sized(m, 80, 24)

	if d := m.decorFor(Row{Path: "/1"}); d.marker != "~" {
		t.Errorf("changed key b (/1) should show ~, got %q", d.marker)
	}
	if d := m.decorFor(Row{Path: "/2"}); d.marker != "+" {
		t.Errorf("added key c (/2) should show +, got %q", d.marker)
	}
	if d := m.decorFor(Row{Path: "/0"}); d.marker != "" {
		t.Errorf("unchanged key a (/0) should have no marker, got %q", d.marker)
	}
	if !strings.Contains(m.titleBar(), "[diff +1~1-0]") {
		t.Errorf("title should summarize the diff, got %q", m.titleBar())
	}
}

func TestDiffRecomputesAfterEdit(t *testing.T) {
	baseline := parse(t, `{"a": 1}`)
	m := New(parse(t, `{"a": 1}`), "x.json").WithDiff(baseline, "base.json")
	m = sized(m, 80, 24)
	if m.diffResult.Changed != 0 {
		t.Fatalf("identical docs should have no changes, got %d", m.diffResult.Changed)
	}
	// Toggle nothing editable here; instead add a key and confirm the diff grows.
	m = sendKey(m, "a")
	m = typeText(m, "b")
	m = sendKey(m, "enter")
	if m.diffResult.Added == 0 {
		t.Errorf("adding a key should register as an addition in the diff")
	}
}
