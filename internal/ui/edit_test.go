package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
)

// typeText sends each rune of s to the model as individual key presses.
func typeText(m Model, s string) Model {
	for _, r := range s {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	return m
}

// TestEditStringValue edits a string via Enter and checks the model is dirty
// and the value changed.
func TestEditStringValue(t *testing.T) {
	m := New(parse(t, `{"name": "Ada"}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down")  // onto "name"
	m = sendKey(m, "enter") // open value prompt
	if m.mode != modeInput {
		t.Fatalf("expected modeInput, got %d", m.mode)
	}
	// Clear the prefilled value, then type a new one.
	for range "Ada" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(Model)
	}
	m = typeText(m, "Grace")
	m = sendKey(m, "enter") // commit
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after commit, got %d", m.mode)
	}
	if !m.dirty {
		t.Error("model should be dirty after edit")
	}
	got := m.root.Members[0].Value
	if got.Kind != document.KindString || got.Str != "Grace" {
		t.Errorf("expected string \"Grace\", got %s %q", got.Kind, got.Str)
	}
}

// TestEditNumberRejectsInvalid confirms an invalid number keeps the prompt open
// and does not mutate the document.
func TestEditNumberRejectsInvalid(t *testing.T) {
	m := New(parse(t, `{"n": 1}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down")
	m = sendKey(m, "enter")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace}) // clear "1"
	m = updated.(Model)
	m = typeText(m, "abc")
	m = sendKey(m, "enter") // attempt commit
	if m.mode != modeInput {
		t.Errorf("invalid number should keep prompt open, mode=%d", m.mode)
	}
	if m.promptErr == "" {
		t.Error("expected a validation error message")
	}
	if m.root.Members[0].Value.Num.String() != "1" {
		t.Errorf("document should be unchanged, got %s", m.root.Members[0].Value.Num)
	}
}

// TestToggleBool flips a boolean with Enter, no prompt.
func TestToggleBool(t *testing.T) {
	m := New(parse(t, `{"on": true}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down")
	m = sendKey(m, "enter")
	if m.mode != modeNormal {
		t.Fatalf("bool toggle should not open a prompt")
	}
	if m.root.Members[0].Value.Bool != false {
		t.Error("expected bool to toggle to false")
	}
}

// TestChangeType converts a string to null via the picker.
func TestChangeType(t *testing.T) {
	m := New(parse(t, `{"v": "hi"}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down")
	m = typeText(m, "t") // open type picker
	if m.mode != modeChoice {
		t.Fatalf("expected modeChoice, got %d", m.mode)
	}
	m = typeText(m, "u") // null
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after pick, got %d", m.mode)
	}
	if m.root.Members[0].Value.Kind != document.KindNull {
		t.Errorf("expected null, got %s", m.root.Members[0].Value.Kind)
	}
}

// TestAddArrayElement appends to an array with 'a'.
func TestAddArrayElement(t *testing.T) {
	m := New(parse(t, `{"xs": [1]}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // onto "xs"
	m = typeText(m, "a")   // add element (no prompt for arrays)
	arr := m.root.Members[0].Value
	if arr.ChildCount() != 2 {
		t.Errorf("expected 2 elements after add, got %d", arr.ChildCount())
	}
}

// TestAddObjectKey prompts for a key and inserts a null member.
func TestAddObjectKey(t *testing.T) {
	m := New(parse(t, `{"a": 1}`), "x.json")
	m = sized(m, 80, 24)
	// Cursor starts on the root object.
	m = typeText(m, "a") // add key
	if m.mode != modeInput {
		t.Fatalf("expected key prompt, got mode %d", m.mode)
	}
	m = typeText(m, "b")
	m = sendKey(m, "enter")
	if m.root.ChildCount() != 2 || m.root.Members[1].Key != "b" {
		t.Errorf("expected new key \"b\", members=%+v", m.root.Members)
	}
	if m.root.Members[1].Value.Kind != document.KindNull {
		t.Error("new member should default to null")
	}
}

// TestCutNode confirms ^K removes the selected node immediately (no
// confirmation) and places it on the clipboard.
func TestCutNode(t *testing.T) {
	m := New(parse(t, `{"a": 1, "b": 2}`), "x.json")
	m = sized(m, 80, 24)
	m = sendKey(m, "down") // onto "a"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("cut should not open a prompt, got mode %d", m.mode)
	}
	if m.root.ChildCount() != 1 || m.root.Members[0].Key != "b" {
		t.Errorf("expected only \"b\" to remain, got %+v", m.root.Members)
	}
	if m.clipboard == nil || m.clipboard.Num.String() != "1" {
		t.Errorf("cut node should be on the clipboard, got %v", m.clipboard)
	}
	if !m.clipboardHasKey || m.clipboardKey != "a" {
		t.Errorf("clipboard should remember key \"a\", got hasKey=%v key=%q", m.clipboardHasKey, m.clipboardKey)
	}
}

// TestSaveProducesValidJSON edits a value, saves to a temp file, and reparses
// it to confirm the output is well-formed and reflects the edit.
func TestSaveProducesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	m := New(parse(t, `{"name": "Ada", "age": 36}`), path)
	m = sized(m, 80, 24)
	// Toggle nothing; just change age to 37.
	m = sendKey(m, "down") // name
	m = sendKey(m, "down") // age
	m = sendKey(m, "enter")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	m = typeText(m, "37")
	m = sendKey(m, "enter")

	// Ctrl-O, accept the prefilled path.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(Model)
	m = sendKey(m, "enter")

	if m.dirty {
		t.Error("model should be clean after save")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	reparsed, err := document.Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("saved file is not valid JSON: %v\n%s", err, data)
	}
	if reparsed.Members[1].Value.Num.String() != "37" {
		t.Errorf("expected age 37 in saved file, got %s", reparsed.Members[1].Value.Num)
	}
}
