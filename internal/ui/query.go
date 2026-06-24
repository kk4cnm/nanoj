package ui

import (
	"strings"

	"github.com/kk4cnm/nanoj/internal/document"
)

// A search query is a small grammar layered over the original substring search.
// It parses into a list of predicates that are ANDed together. Supported terms:
//
//	key:name     key contains "name" (case-insensitive)
//	type:number  the node is of that JSON type (null/bool/number/string/array/object,
//	             plus common aliases like boolean/obj/arr/str/num)
//	value:123    the scalar value contains "123" (case-insensitive)
//	key=value    the key equals "key" AND the scalar value equals "value" (exact,
//	             case-insensitive) — a precise "this field has this value" probe
//	plain        any other word matches key OR value as a substring (the original
//	             behavior)
//
// A query with no structured term (no key:/type:/value:/= form) is treated as a
// single plain substring, exactly as before — so existing searches, including
// ones containing spaces, keep working unchanged. Once any structured term is
// present, the query is split on whitespace into ANDed terms.

type predKind int

const (
	predPlain predKind = iota // substring on key OR value
	predKey                   // substring on key
	predType                  // exact type-name match
	predValue                 // substring on value
	predKeyEq                 // key exact AND value exact
)

type predicate struct {
	kind predKind
	a    string // primary needle (lower-cased): substring, type name, or key
	b    string // secondary needle (lower-cased): value, for predKeyEq
}

type searchQuery struct {
	preds []predicate
}

// empty reports whether the query has no predicates to match.
func (q searchQuery) empty() bool { return len(q.preds) == 0 }

// parseQuery turns a raw query string into a searchQuery. It never errors: an
// unrecognizable term falls back to a plain substring match.
func parseQuery(s string) searchQuery {
	s = strings.TrimSpace(s)
	if s == "" {
		return searchQuery{}
	}

	fields := strings.Fields(s)
	// Decide whether the query opts into the structured grammar. If not, the
	// whole string (spaces and all) is one plain substring, preserving the
	// original behavior.
	structured := false
	for _, f := range fields {
		if _, ok := parsePredicate(f); ok {
			structured = true
			break
		}
	}
	if !structured {
		return searchQuery{preds: []predicate{{kind: predPlain, a: strings.ToLower(s)}}}
	}

	var preds []predicate
	for _, f := range fields {
		if p, ok := parsePredicate(f); ok {
			preds = append(preds, p)
		} else {
			preds = append(preds, predicate{kind: predPlain, a: strings.ToLower(f)})
		}
	}
	return searchQuery{preds: preds}
}

// parsePredicate parses a single whitespace-delimited term into a structured
// predicate. It returns ok=false when the term has no recognized form (the
// caller then treats it as a plain substring).
func parsePredicate(f string) (predicate, bool) {
	lower := strings.ToLower(f)
	switch {
	case strings.HasPrefix(lower, "key:"):
		return predicate{kind: predKey, a: lower[len("key:"):]}, true
	case strings.HasPrefix(lower, "type:"):
		return predicate{kind: predType, a: normalizeType(lower[len("type:"):])}, true
	case strings.HasPrefix(lower, "value:"):
		return predicate{kind: predValue, a: lower[len("value:"):]}, true
	}
	// key=value: an '=' with a non-empty key to its left.
	if i := strings.Index(f, "="); i > 0 {
		return predicate{
			kind: predKeyEq,
			a:    strings.ToLower(f[:i]),
			b:    strings.ToLower(f[i+1:]),
		}, true
	}
	return predicate{}, false
}

// normalizeType maps common type aliases onto the canonical Kind names so that
// e.g. "type:boolean" and "type:bool" both work.
func normalizeType(t string) string {
	switch t {
	case "boolean":
		return "bool"
	case "obj":
		return "object"
	case "arr", "list":
		return "array"
	case "str":
		return "string"
	case "num", "int", "integer", "float":
		return "number"
	}
	return t
}

// matchNode reports whether a value with the given key satisfies the query.
// hasKey is false for array elements and the root, where key predicates can
// never match. n may be nil (a missing table cell), which matches nothing.
func (q searchQuery) matchNode(key string, hasKey bool, n *document.Node) bool {
	if q.empty() {
		return false
	}
	for _, p := range q.preds {
		if !p.match(key, hasKey, n) {
			return false
		}
	}
	return true
}

func (p predicate) match(key string, hasKey bool, n *document.Node) bool {
	keyLower := strings.ToLower(key)
	switch p.kind {
	case predPlain:
		if hasKey && strings.Contains(keyLower, p.a) {
			return true
		}
		return strings.Contains(valueText(n), p.a)
	case predKey:
		return hasKey && strings.Contains(keyLower, p.a)
	case predType:
		return n != nil && strings.EqualFold(n.Kind.String(), p.a)
	case predValue:
		return strings.Contains(valueText(n), p.a)
	case predKeyEq:
		return hasKey && keyLower == p.a && valueText(n) == p.b
	}
	return false
}

// valueText returns a scalar node's value as lower-cased, unquoted text for
// matching. Containers and nil have no value text.
func valueText(n *document.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case document.KindString:
		return strings.ToLower(n.Str)
	case document.KindNumber:
		return strings.ToLower(n.Num.String())
	case document.KindBool:
		if n.Bool {
			return "true"
		}
		return "false"
	case document.KindNull:
		return "null"
	default:
		return "" // containers
	}
}
