package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kk4cnm/nanoj/internal/document"
)

func load(t *testing.T, schemaJSON string) *Checker {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(p, []byte(schemaJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return c
}

func parse(t *testing.T, s string) *document.Node {
	t.Helper()
	n, err := document.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("parse doc: %v", err)
	}
	return n
}

func TestValidDocument(t *testing.T) {
	c := load(t, `{
		"type": "object",
		"properties": {"name": {"type": "string"}, "age": {"type": "number"}},
		"required": ["name"]
	}`)
	o := c.Annotate(parse(t, `{"name": "Ada", "age": 36}`))
	if !o.Valid {
		t.Errorf("expected valid, summary=%q", o.Summary)
	}
	if !strings.Contains(o.Summary, "valid") {
		t.Errorf("summary=%q", o.Summary)
	}
}

func TestInvalidTypeFlagsExactNode(t *testing.T) {
	c := load(t, `{
		"type": "object",
		"properties": {"name": {"type": "string"}, "age": {"type": "number"}}
	}`)
	// age is a string here → the "age" node (index 1) should be invalid.
	o := c.Annotate(parse(t, `{"name": "Ada", "age": "old"}`))
	if o.Valid {
		t.Fatal("expected invalid")
	}
	if a, _ := o.At("/1"); !a.Invalid {
		t.Errorf("expected /1 (age) invalid, got %+v", a)
	}
	if a, _ := o.At("/0"); a.Invalid {
		t.Errorf("expected /0 (name) valid, got %+v", a)
	}
	// The root should not be blanket-flagged just because a child failed.
	if a, _ := o.At(""); a.Invalid {
		t.Errorf("root should not be flagged invalid, got %+v", a)
	}
}

func TestRequiredAndMissing(t *testing.T) {
	c := load(t, `{
		"type": "object",
		"properties": {"name": {"type": "string"}, "email": {"type": "string"}},
		"required": ["name", "email"]
	}`)
	o := c.Annotate(parse(t, `{"name": "Ada"}`))
	// name is present and required.
	if a, _ := o.At("/0"); !a.Required {
		t.Errorf("expected /0 required, got %+v", a)
	}
	// root should report email as missing-required.
	root, _ := o.At("")
	found := false
	for _, k := range root.MissingRequired {
		if k == "email" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected email in MissingRequired, got %+v", root.MissingRequired)
	}
}

func TestDescriptionAndType(t *testing.T) {
	c := load(t, `{
		"type": "object",
		"properties": {"port": {"type": "integer", "description": "listen port"}}
	}`)
	o := c.Annotate(parse(t, `{"port": 8080}`))
	a, _ := o.At("/0")
	if a.Description != "listen port" {
		t.Errorf("description=%q", a.Description)
	}
	if !strings.Contains(a.ExpectedType, "integer") {
		t.Errorf("expectedType=%q", a.ExpectedType)
	}
}

func TestEnumValues(t *testing.T) {
	c := load(t, `{
		"type": "object",
		"properties": {"level": {"enum": ["low", "high"]}}
	}`)
	o := c.Annotate(parse(t, `{"level": "low"}`))
	a, _ := o.At("/0")
	disp := a.EnumDisplay()
	if len(disp) != 2 || disp[0] != "low" || disp[1] != "high" {
		t.Errorf("enum display=%v", disp)
	}
}

func TestRefResolution(t *testing.T) {
	c := load(t, `{
		"type": "object",
		"properties": {"user": {"$ref": "#/$defs/person"}},
		"$defs": {"person": {"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}}
	}`)
	// user.name is a number → invalid at /0/0.
	o := c.Annotate(parse(t, `{"user": {"name": 5}}`))
	if o.Valid {
		t.Fatal("expected invalid via $ref")
	}
	if a, _ := o.At("/0/0"); !a.Invalid {
		t.Errorf("expected /0/0 invalid through $ref, got %+v", a)
	}
}

func TestArrayItems(t *testing.T) {
	c := load(t, `{
		"type": "array",
		"items": {"type": "number"}
	}`)
	o := c.Annotate(parse(t, `[1, "two", 3]`))
	if a, _ := o.At("/1"); !a.Invalid {
		t.Errorf("expected /1 invalid, got %+v", a)
	}
	if a, _ := o.At("/0"); a.Invalid {
		t.Errorf("expected /0 valid, got %+v", a)
	}
}
