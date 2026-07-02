package document

import (
	"strings"
	"testing"
)

func mustParseDoc(t *testing.T, s string) *Node {
	t.Helper()
	n, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return n
}

func TestDiffChangedScalar(t *testing.T) {
	base := mustParseDoc(t, `{"a": 1, "b": 2}`)
	work := mustParseDoc(t, `{"a": 1, "b": 3}`)
	d := Diff(base, work)
	if d.Kind("/0") != DiffNone {
		t.Errorf("/0 should be unchanged")
	}
	if d.Kind("/1") != DiffChanged {
		t.Errorf("/1 should be changed, got %v", d.Kind("/1"))
	}
	if d.Changed != 1 || d.Added != 0 || d.Removed != 0 {
		t.Errorf("counts = +%d ~%d -%d", d.Added, d.Changed, d.Removed)
	}
}

func TestDiffAddedKey(t *testing.T) {
	base := mustParseDoc(t, `{"a": 1}`)
	work := mustParseDoc(t, `{"a": 1, "b": {"c": 2}}`)
	d := Diff(base, work)
	if d.Kind("/1") != DiffAdded {
		t.Errorf("/1 should be added")
	}
	// Descendants of an added subtree are all marked.
	if d.Kind("/1/0") != DiffAdded {
		t.Errorf("/1/0 should be added")
	}
	if d.Added != 1 {
		t.Errorf("Added should count the subtree root once, got %d", d.Added)
	}
}

func TestDiffRemovedKey(t *testing.T) {
	base := mustParseDoc(t, `{"a": 1, "b": 2}`)
	work := mustParseDoc(t, `{"a": 1}`)
	d := Diff(base, work)
	if d.Removed != 1 {
		t.Errorf("Removed = %d, want 1", d.Removed)
	}
}

func TestDiffTypeChange(t *testing.T) {
	base := mustParseDoc(t, `{"a": 1}`)
	work := mustParseDoc(t, `{"a": "one"}`)
	d := Diff(base, work)
	if d.Kind("/0") != DiffChanged {
		t.Errorf("/0 should be changed (type)")
	}
}

func TestDiffArrayPositional(t *testing.T) {
	base := mustParseDoc(t, `[1, 2, 3]`)
	work := mustParseDoc(t, `[1, 9, 3, 4]`)
	d := Diff(base, work)
	if d.Kind("/1") != DiffChanged {
		t.Errorf("/1 should be changed")
	}
	if d.Kind("/3") != DiffAdded {
		t.Errorf("/3 should be added")
	}
	if d.Added != 1 || d.Changed != 1 {
		t.Errorf("counts +%d ~%d", d.Added, d.Changed)
	}
}

func TestDiffArrayShrink(t *testing.T) {
	base := mustParseDoc(t, `[1, 2, 3]`)
	work := mustParseDoc(t, `[1, 2]`)
	d := Diff(base, work)
	if d.Removed != 1 {
		t.Errorf("Removed = %d, want 1", d.Removed)
	}
}

func TestDiffIdentical(t *testing.T) {
	base := mustParseDoc(t, `{"a": [1, {"b": true}], "c": null}`)
	work := mustParseDoc(t, `{"a": [1, {"b": true}], "c": null}`)
	d := Diff(base, work)
	if len(d.ByPath) != 0 || d.Added+d.Changed+d.Removed != 0 {
		t.Errorf("identical docs should have no diff, got %+v", d)
	}
}
