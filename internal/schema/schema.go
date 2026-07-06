// Package schema provides an optional JSON Schema overlay for the editor. It is
// a *read-only* layer: it never mutates the document tree. Instead it produces
// an Overlay that annotates nodes by their structural path — the same
// index-based path scheme the tree/table views use for expansion and the table
// array — so the UI can decorate rows during render (highlight invalid values,
// mark required fields, offer enum choices, show a field's description) without
// forking the model.
//
// Validation leans entirely on santhosh-tekuri/jsonschema (draft 2020-12 and
// earlier). Our own logic is limited to walking the *compiled* schema in
// parallel with the document to find the subschemas that apply to each node —
// the library has already resolved $ref, so we follow Schema.Ref. Combinators
// are handled to the extent that is unambiguous: allOf branches all apply, so
// they are flattened into the working set (properties, required, descriptions
// and types are drawn from every branch); anyOf/oneOf branches describe
// alternatives, so they contribute display information only (the alternative
// types, and enum values merged across branches) — we never descend into their
// properties, because which branch applies depends on the data.
package schema

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/kk4cnm/nanoj/internal/document"
)

// Checker holds a compiled schema and can annotate documents against it.
type Checker struct {
	sch  *jsonschema.Schema
	name string // schema file name, for display
}

// Annotation is what the overlay knows about a single node.
type Annotation struct {
	Invalid         bool     // the node's value fails validation
	Required        bool     // the node is a required property of its parent object
	Description     string   // the schema "description" for this node, if any
	ExpectedType    string   // schema type(s), e.g. "string" or "string or null"
	EnumValues      []any    // allowed values (raw), if the schema constrains an enum
	MissingRequired []string // for an object node: required keys that are absent
}

// HasInfo reports whether the annotation carries anything worth showing in the
// status line for a selected node.
func (a Annotation) HasInfo() bool {
	return a.Description != "" || a.ExpectedType != "" || len(a.EnumValues) > 0 ||
		len(a.MissingRequired) > 0 || a.Required
}

// EnumDisplay returns human-readable text for each enum value, in order.
func (a Annotation) EnumDisplay() []string {
	out := make([]string, len(a.EnumValues))
	for i, v := range a.EnumValues {
		out[i] = displayValue(v)
	}
	return out
}

// Overlay is the result of checking a document: per-path annotations plus a
// one-line summary of the document's validity.
type Overlay struct {
	byPath  map[string]Annotation
	Valid   bool
	Summary string
}

// At returns the annotation for the node at the given structural path.
func (o Overlay) At(path string) (Annotation, bool) {
	a, ok := o.byPath[path]
	return a, ok
}

// Name returns the schema's display name.
func (c *Checker) Name() string { return c.name }

// Load compiles the schema at path.
func Load(path string) (*Checker, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	const url = "nanoj://schema"
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &Checker{sch: sch, name: baseName(path)}, nil
}

// Annotate validates root against the schema and returns an Overlay keyed by
// structural path.
func (c *Checker) Annotate(root *document.Node) Overlay {
	o := Overlay{byPath: map[string]Annotation{}, Valid: true}

	// Collect the most-specific (leaf) instance locations that failed, keyed the
	// same way we key the walk, so we can flag exactly those nodes.
	invalid := map[string]bool{}
	if err := c.sch.Validate(root.ToInterface()); err != nil {
		o.Valid = false
		if ve, ok := err.(*jsonschema.ValidationError); ok {
			collectLeafLocations(ve, invalid)
		}
	}

	walk(root, "", expand(c.sch, nil), "", false, invalid, o.byPath)
	o.Summary = summarize(c.name, o.Valid, len(invalid))
	return o
}

// walk descends the document and the applicable schemas together, recording an
// annotation for each node. schemas is the expanded set that unconditionally
// applies to n (the subschema plus its flattened allOf branches). keyPath is
// the JSON-pointer-style key path used to match validation locations; idxPath
// is the index-based path used as the overlay key. required is set when the
// parent object lists this node's key in its "required".
func walk(n *document.Node, idxPath string, schemas []*jsonschema.Schema, keyPath string, required bool, invalid map[string]bool, out map[string]Annotation) {
	a := Annotation{Required: required}
	if invalid[keyPath] {
		a.Invalid = true
	}
	for _, s := range schemas {
		if a.ExpectedType == "" && s.Types != nil {
			a.ExpectedType = strings.Join(s.Types.ToStrings(), " or ")
		}
		if a.Description == "" {
			a.Description = s.Description
		}
		if a.EnumValues == nil && s.Enum != nil {
			a.EnumValues = s.Enum.Values
		}
	}
	// If the schemas that directly apply don't pin these down, fall back to the
	// anyOf/oneOf alternatives.
	if a.ExpectedType == "" {
		a.ExpectedType = alternativeTypes(schemas)
	}
	if a.EnumValues == nil {
		a.EnumValues = alternativeEnums(schemas)
	}

	switch n.Kind {
	case document.KindObject:
		reqSet := map[string]bool{}
		present := map[string]bool{}
		for _, m := range n.Members {
			present[m.Key] = true
		}
		for _, s := range schemas {
			for _, req := range s.Required {
				if !reqSet[req] && !present[req] {
					a.MissingRequired = append(a.MissingRequired, req)
				}
				reqSet[req] = true
			}
		}
		for i, m := range n.Members {
			cp := childKeyPath(keyPath, m.Key)
			walk(m.Value, childIdx(idxPath, i), propertySchemas(schemas, m.Key), cp, reqSet[m.Key], invalid, out)
		}
	case document.KindArray:
		for i, item := range n.Items {
			cp := childKeyPath(keyPath, strconv.Itoa(i))
			walk(item, childIdx(idxPath, i), itemSchemas(schemas, i), cp, false, invalid, out)
		}
	}

	out[idxPath] = a
}

// maxExpand bounds the expanded schema set as a guard against reference cycles
// and pathological allOf nesting.
const maxExpand = 100

// expand appends to out the subschemas that all unconditionally apply to the
// same instance: s itself plus every allOf branch, recursively, with $ref
// resolved along the way.
func expand(s *jsonschema.Schema, out []*jsonschema.Schema) []*jsonschema.Schema {
	s = resolve(s)
	if s == nil || len(out) >= maxExpand {
		return out
	}
	out = append(out, s)
	for _, b := range s.AllOf {
		out = expand(b, out)
	}
	return out
}

// alternativeTypes returns a display string for the types allowed by the
// anyOf/oneOf branches of the given schemas, e.g. "string or number".
func alternativeTypes(schemas []*jsonschema.Schema) string {
	var types []string
	seen := map[string]bool{}
	for _, s := range schemas {
		for _, b := range append(append([]*jsonschema.Schema{}, s.AnyOf...), s.OneOf...) {
			for _, alt := range expand(b, nil) {
				if alt.Types == nil {
					continue
				}
				for _, t := range alt.Types.ToStrings() {
					if !seen[t] {
						seen[t] = true
						types = append(types, t)
					}
				}
			}
		}
	}
	return strings.Join(types, " or ")
}

// alternativeEnums merges the enum values found across anyOf/oneOf branches of
// the given schemas, deduplicated, in branch order. This makes the enum
// pick-list work for the common `oneOf: [{enum: [...]}, ...]` shape.
func alternativeEnums(schemas []*jsonschema.Schema) []any {
	var values []any
	seen := map[string]bool{}
	for _, s := range schemas {
		for _, b := range append(append([]*jsonschema.Schema{}, s.AnyOf...), s.OneOf...) {
			for _, alt := range expand(b, nil) {
				if alt.Enum == nil {
					continue
				}
				for _, v := range alt.Enum.Values {
					if key := displayValue(v); !seen[key] {
						seen[key] = true
						values = append(values, v)
					}
				}
			}
		}
	}
	return values
}

// propertySchemas returns the expanded set of subschemas that apply to object
// key k across every schema in the parent set.
func propertySchemas(parents []*jsonschema.Schema, k string) []*jsonschema.Schema {
	var out []*jsonschema.Schema
	for _, p := range parents {
		if p.Properties != nil {
			if s, ok := p.Properties[k]; ok {
				out = expand(s, out)
				continue
			}
		}
		if s, ok := p.AdditionalProperties.(*jsonschema.Schema); ok {
			out = expand(s, out)
		}
	}
	return out
}

// itemSchemas returns the expanded set of subschemas that apply to array index
// i across every schema in the parent set, handling both the draft 2020-12
// shape (PrefixItems + Items2020) and the older Items form (*Schema for all
// items, or []*Schema for tuples).
func itemSchemas(parents []*jsonschema.Schema, i int) []*jsonschema.Schema {
	var out []*jsonschema.Schema
	for _, p := range parents {
		if i < len(p.PrefixItems) {
			out = expand(p.PrefixItems[i], out)
			continue
		}
		if p.Items2020 != nil {
			out = expand(p.Items2020, out)
			continue
		}
		switch it := p.Items.(type) {
		case *jsonschema.Schema:
			out = expand(it, out)
		case []*jsonschema.Schema:
			if i < len(it) {
				out = expand(it[i], out)
			}
		}
	}
	return out
}

// resolve follows a chain of $ref links to the schema that actually applies,
// guarding against reference cycles.
func resolve(s *jsonschema.Schema) *jsonschema.Schema {
	for i := 0; s != nil && s.Ref != nil && i < 100; i++ {
		s = s.Ref
	}
	return s
}

func childIdx(parent string, index int) string {
	return parent + "/" + strconv.Itoa(index)
}

// childKeyPath appends a token to a JSON-pointer-style key path, matching how
// the validator reports InstanceLocation (unescaped tokens joined by NUL here
// for unambiguous comparison).
func childKeyPath(parent, token string) string {
	return parent + "\x00" + token
}

// collectLeafLocations gathers the instance locations of the deepest errors so
// only the specific failing nodes are flagged (not every ancestor).
func collectLeafLocations(ve *jsonschema.ValidationError, out map[string]bool) {
	if len(ve.Causes) == 0 {
		key := ""
		for _, tok := range ve.InstanceLocation {
			key = childKeyPath(key, tok)
		}
		out[key] = true
		return
	}
	for _, c := range ve.Causes {
		collectLeafLocations(c, out)
	}
}

func summarize(name string, valid bool, nInvalid int) string {
	if valid {
		return "schema " + name + ": valid ✓"
	}
	noun := "problems"
	if nInvalid == 1 {
		noun = "problem"
	}
	return fmt.Sprintf("schema %s: %d %s ✗", name, nInvalid, noun)
}

func displayValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(x)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

func baseName(path string) string {
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		return path[i+1:]
	}
	return path
}
