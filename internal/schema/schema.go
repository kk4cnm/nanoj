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
// parallel with the document to find the subschema that applies to each node —
// which the library has already resolved $ref for us, so we follow Schema.Ref
// but otherwise keep this deliberately small (allOf/anyOf/oneOf combinators are
// not deeply resolved here).
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

	walk(root, "", resolve(c.sch), "", false, invalid, o.byPath)
	o.Summary = summarize(c.name, o.Valid, len(invalid))
	return o
}

// walk descends the document and the (already $ref-resolved) schema together,
// recording an annotation for each node. keyPath is the JSON-pointer-style key
// path used to match validation locations; idxPath is the index-based path used
// as the overlay key. required is set when the parent object lists this node's
// key in its "required".
func walk(n *document.Node, idxPath string, sch *jsonschema.Schema, keyPath string, required bool, invalid map[string]bool, out map[string]Annotation) {
	a := Annotation{Required: required}
	if invalid[keyPath] {
		a.Invalid = true
	}
	if sch != nil {
		if sch.Types != nil {
			a.ExpectedType = strings.Join(sch.Types.ToStrings(), " or ")
		}
		a.Description = sch.Description
		if sch.Enum != nil {
			a.EnumValues = sch.Enum.Values
		}
	}

	switch n.Kind {
	case document.KindObject:
		reqSet := map[string]bool{}
		present := map[string]bool{}
		for _, m := range n.Members {
			present[m.Key] = true
		}
		if sch != nil {
			for _, req := range sch.Required {
				reqSet[req] = true
				if !present[req] {
					a.MissingRequired = append(a.MissingRequired, req)
				}
			}
		}
		for i, m := range n.Members {
			childSch := propertySchema(sch, m.Key)
			cp := childKeyPath(keyPath, m.Key)
			walk(m.Value, childIdx(idxPath, i), resolve(childSch), cp, reqSet[m.Key], invalid, out)
		}
	case document.KindArray:
		for i, item := range n.Items {
			childSch := itemSchema(sch, i)
			cp := childKeyPath(keyPath, strconv.Itoa(i))
			walk(item, childIdx(idxPath, i), resolve(childSch), cp, false, invalid, out)
		}
	}

	out[idxPath] = a
}

// propertySchema returns the subschema for object key k, or nil.
func propertySchema(parent *jsonschema.Schema, k string) *jsonschema.Schema {
	if parent == nil {
		return nil
	}
	if parent.Properties != nil {
		if s, ok := parent.Properties[k]; ok {
			return s
		}
	}
	if s, ok := parent.AdditionalProperties.(*jsonschema.Schema); ok {
		return s
	}
	return nil
}

// itemSchema returns the subschema for array index i, handling both the
// draft 2020-12 shape (PrefixItems + Items2020) and the older Items form
// (*Schema for all items, or []*Schema for tuples).
func itemSchema(parent *jsonschema.Schema, i int) *jsonschema.Schema {
	if parent == nil {
		return nil
	}
	if i < len(parent.PrefixItems) {
		return parent.PrefixItems[i]
	}
	if parent.Items2020 != nil {
		return parent.Items2020
	}
	switch it := parent.Items.(type) {
	case *jsonschema.Schema:
		return it
	case []*jsonschema.Schema:
		if i < len(it) {
			return it[i]
		}
	}
	return nil
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
