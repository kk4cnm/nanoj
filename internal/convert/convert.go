// Package convert imports YAML and TOML documents into nanoj's JSON tree.
//
// This is deliberately a one-way *conversion*, not round-trip editing: the
// valid-by-construction promise is a JSON promise, and YAML/TOML features that
// JSON cannot express (comments, anchors, tags, datetimes) do not survive. The
// caller opens the result as a JSON working copy and the only path back to
// disk remains the strict JSON serializer. Anything JSON genuinely cannot
// represent (infinities, NaN) is a conversion error rather than a silent
// mangling.
//
// Both importers preserve key order as written in the source file, matching
// how the JSON parser preserves object order.
package convert

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	yaml "gopkg.in/yaml.v3"

	"github.com/kk4cnm/nanoj/internal/document"
)

// FromYAML decodes a YAML document into a JSON tree. Mapping order is
// preserved; anchors/aliases are expanded; scalar tags map onto the six JSON
// types (timestamps and unrecognized tags become strings).
func FromYAML(r io.Reader) (*document.Node, error) {
	var root yaml.Node
	if err := yaml.NewDecoder(r).Decode(&root); err != nil {
		if err == io.EOF {
			return document.NewNull(), nil // an empty document is null
		}
		return nil, err
	}
	return yamlNode(&root, 0)
}

// yamlMaxDepth bounds recursion as a guard against alias bombs that slip past
// the yaml package's own expansion limits.
const yamlMaxDepth = 10000

func yamlNode(n *yaml.Node, depth int) (*document.Node, error) {
	if depth > yamlMaxDepth {
		return nil, fmt.Errorf("yaml: document nested too deeply")
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return document.NewNull(), nil
		}
		return yamlNode(n.Content[0], depth+1)

	case yaml.AliasNode:
		return yamlNode(n.Alias, depth+1)

	case yaml.MappingNode:
		obj := document.NewObject()
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("yaml line %d: JSON object keys must be scalars", k.Line)
			}
			val, err := yamlNode(v, depth+1)
			if err != nil {
				return nil, err
			}
			if err := obj.AddMember(k.Value, val); err != nil {
				return nil, fmt.Errorf("yaml line %d: %w", k.Line, err)
			}
		}
		return obj, nil

	case yaml.SequenceNode:
		arr := document.NewArray()
		for _, item := range n.Content {
			val, err := yamlNode(item, depth+1)
			if err != nil {
				return nil, err
			}
			_ = arr.AppendItem(val)
		}
		return arr, nil

	case yaml.ScalarNode:
		return yamlScalar(n)

	default:
		return nil, fmt.Errorf("yaml line %d: unsupported node kind", n.Line)
	}
}

// yamlScalar converts one YAML scalar by its resolved tag. Numbers keep their
// source text when it is already a valid JSON number; YAML-only spellings
// (hex, octal, underscores) are re-formatted canonically.
func yamlScalar(n *yaml.Node) (*document.Node, error) {
	switch n.Tag {
	case "!!null":
		return document.NewNull(), nil

	case "!!bool":
		var b bool
		if err := n.Decode(&b); err != nil {
			return nil, fmt.Errorf("yaml line %d: %w", n.Line, err)
		}
		return document.NewBool(b), nil

	case "!!int":
		if num, ok := document.ParseNumber(n.Value); ok {
			return document.NewNumber(num), nil
		}
		var i int64
		if err := n.Decode(&i); err == nil {
			return document.NewNumber(num64(strconv.FormatInt(i, 10))), nil
		}
		var u uint64
		if err := n.Decode(&u); err != nil {
			return nil, fmt.Errorf("yaml line %d: cannot represent %q as a JSON number", n.Line, n.Value)
		}
		return document.NewNumber(num64(strconv.FormatUint(u, 10))), nil

	case "!!float":
		if num, ok := document.ParseNumber(n.Value); ok {
			return document.NewNumber(num), nil
		}
		var f float64
		if err := n.Decode(&f); err != nil {
			return nil, fmt.Errorf("yaml line %d: %w", n.Line, err)
		}
		return floatNode(f, n.Value)

	default:
		// Strings, timestamps, binary, and any custom tag: keep the text. A
		// conversion is lossy by declaration, and a string is the faithful
		// JSON spelling of everything else.
		return document.NewString(n.Value), nil
	}
}

// FromTOML decodes a TOML document into a JSON tree. Key order is recovered
// from the decoder's metadata so tables keep their order as written;
// datetimes become RFC 3339 strings.
func FromTOML(r io.Reader) (*document.Node, error) {
	var v map[string]any
	meta, err := toml.NewDecoder(r).Decode(&v)
	if err != nil {
		return nil, err
	}
	return tomlValue(v, "", keyOrder(meta))
}

// keyOrder builds, for every table path, its child keys in order of appearance.
// Paths are NUL-joined so dotted keys can't collide. Array-of-table elements
// share their parent's path, so their key order is the union across elements —
// each element then filters it down to the keys it actually has.
func keyOrder(meta toml.MetaData) map[string][]string {
	order := map[string][]string{}
	seen := map[string]bool{}
	for _, key := range meta.Keys() {
		parent := strings.Join(key[:len(key)-1], "\x00")
		leaf := key[len(key)-1]
		id := parent + "\x00\x00" + leaf
		if !seen[id] {
			seen[id] = true
			order[parent] = append(order[parent], leaf)
		}
	}
	return order
}

func tomlValue(v any, path string, order map[string][]string) (*document.Node, error) {
	switch x := v.(type) {
	case nil:
		return document.NewNull(), nil
	case bool:
		return document.NewBool(x), nil
	case string:
		return document.NewString(x), nil
	case int64:
		return document.NewNumber(num64(strconv.FormatInt(x, 10))), nil
	case float64:
		return floatNode(x, strconv.FormatFloat(x, 'g', -1, 64))
	case time.Time:
		return document.NewString(x.Format(time.RFC3339Nano)), nil

	case map[string]any:
		obj := document.NewObject()
		for _, k := range tableKeys(x, path, order) {
			child, err := tomlValue(x[k], childTablePath(path, k), order)
			if err != nil {
				return nil, err
			}
			if err := obj.AddMember(k, child); err != nil {
				return nil, err
			}
		}
		return obj, nil

	case []map[string]any: // array of tables
		arr := document.NewArray()
		for _, elem := range x {
			child, err := tomlValue(elem, path, order)
			if err != nil {
				return nil, err
			}
			_ = arr.AppendItem(child)
		}
		return arr, nil

	case []any:
		arr := document.NewArray()
		for _, elem := range x {
			child, err := tomlValue(elem, path, order)
			if err != nil {
				return nil, err
			}
			_ = arr.AppendItem(child)
		}
		return arr, nil

	default:
		return nil, fmt.Errorf("toml: unsupported value type %T", v)
	}
}

// tableKeys returns the keys of table m in source order (per the metadata),
// followed by any leftovers in Go's map order — leftovers shouldn't happen,
// but a key must never be dropped.
func tableKeys(m map[string]any, path string, order map[string][]string) []string {
	keys := make([]string, 0, len(m))
	used := map[string]bool{}
	for _, k := range order[path] {
		if _, ok := m[k]; ok && !used[k] {
			used[k] = true
			keys = append(keys, k)
		}
	}
	for k := range m {
		if !used[k] {
			keys = append(keys, k)
		}
	}
	return keys
}

func childTablePath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "\x00" + key
}

// floatNode builds a number node for f, refusing values JSON cannot represent.
// srcText is used in the error message.
func floatNode(f float64, srcText string) (*document.Node, error) {
	text := strconv.FormatFloat(f, 'g', -1, 64)
	num, ok := document.ParseNumber(text)
	if !ok {
		return nil, fmt.Errorf("cannot represent %s as a JSON number", srcText)
	}
	return document.NewNumber(num), nil
}

// num64 wraps text already known to be a valid JSON number.
func num64(text string) json.Number { return json.Number(text) }
