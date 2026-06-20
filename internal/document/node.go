// Package document defines nanoj's in-memory representation of a JSON value.
//
// The design principle here is "valid by construction": the user never edits
// raw JSON text. Instead they manipulate this typed tree, and the only path
// back out to disk is through Marshal, which always emits well-formed JSON.
// That makes it structurally impossible for nanoj to save a malformed file.
//
// The tree is the single source of truth. Every view (the tree view today, a
// table view later) is just a way of rendering this same model.
package document

import "encoding/json"

// Kind enumerates the six JSON value types. Each Node carries exactly one
// Kind, and that Kind determines which of the Node's fields are meaningful.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindNumber
	KindString
	KindArray
	KindObject
)

// String returns a short human-readable name for the Kind. It is used in the
// UI (e.g. when changing a value's type) and in error messages.
func (k Kind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindBool:
		return "bool"
	case KindNumber:
		return "number"
	case KindString:
		return "string"
	case KindArray:
		return "array"
	case KindObject:
		return "object"
	default:
		return "invalid"
	}
}

// Member is a single key/value pair within an object. Objects store their
// members as an ordered slice rather than a Go map so that key order is
// preserved across a load/save round-trip. (A Go map randomizes iteration
// order, which would needlessly churn the user's files.)
type Member struct {
	Key   string
	Value *Node
}

// Node is one value in the JSON tree.
//
// Only the fields relevant to the Node's Kind are populated:
//
//	KindNull    - (no payload)
//	KindBool    - Bool
//	KindNumber  - Num (json.Number preserves the original numeric text, so
//	              large integers and high-precision floats round-trip exactly
//	              instead of being forced through float64)
//	KindString  - Str
//	KindArray   - Items
//	KindObject  - Members (ordered)
type Node struct {
	Kind    Kind
	Bool    bool
	Num     json.Number
	Str     string
	Items   []*Node
	Members []Member
}

// NewNull, NewBool, NewString, NewNumber, NewArray and NewObject are small
// constructors used by the editor when the user adds or retypes a value.

func NewNull() *Node                { return &Node{Kind: KindNull} }
func NewBool(b bool) *Node          { return &Node{Kind: KindBool, Bool: b} }
func NewString(s string) *Node      { return &Node{Kind: KindString, Str: s} }
func NewArray() *Node               { return &Node{Kind: KindArray} }
func NewObject() *Node              { return &Node{Kind: KindObject} }
func NewNumber(n json.Number) *Node { return &Node{Kind: KindNumber, Num: n} }

// IsContainer reports whether the node can hold children (object or array).
func (n *Node) IsContainer() bool {
	return n.Kind == KindObject || n.Kind == KindArray
}

// ChildCount returns the number of direct children for containers, 0 otherwise.
func (n *Node) ChildCount() int {
	switch n.Kind {
	case KindArray:
		return len(n.Items)
	case KindObject:
		return len(n.Members)
	default:
		return 0
	}
}
