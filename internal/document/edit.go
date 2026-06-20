package document

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// This file holds the mutation primitives the editor uses. They live in the
// document package (rather than the UI) so the rules that keep the tree valid
// are testable on their own and cannot be bypassed by a view.

// ParseNumber validates that s is a single well-formed JSON number and returns
// it as a json.Number preserving the original text. It is used when the user
// types a numeric value, so an invalid entry is rejected rather than silently
// corrupting the document.
func ParseNumber(s string) (json.Number, bool) {
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(s)))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return "", false
	}
	num, ok := tok.(json.Number)
	if !ok {
		return "", false
	}
	// Reject trailing junk like "1 2" or "3,".
	if _, err := dec.Token(); err != io.EOF {
		return "", false
	}
	return num, true
}

// SetString replaces the node with a string value. Any input is valid as a
// string, so this cannot fail.
func (n *Node) SetString(s string) {
	n.reset(KindString)
	n.Str = s
}

// ScalarFromInput builds a scalar node from user text, inferring its type the
// way a spreadsheet would: empty or "null" become null, "true"/"false" become
// booleans, anything that parses as a JSON number becomes a number, and
// everything else is a string. This is used when filling a blank or null cell,
// where there is no existing type to preserve.
func ScalarFromInput(input string) *Node {
	switch strings.TrimSpace(input) {
	case "", "null":
		return NewNull()
	case "true":
		return NewBool(true)
	case "false":
		return NewBool(false)
	}
	if num, ok := ParseNumber(input); ok {
		return NewNumber(num)
	}
	return NewString(input)
}

// SetScalar replaces the node in place with a scalar whose type is inferred
// from input (see ScalarFromInput). The node keeps its identity, so a parent's
// reference to it stays valid.
func (n *Node) SetScalar(input string) {
	v := ScalarFromInput(input)
	n.reset(v.Kind)
	n.Bool = v.Bool
	n.Num = v.Num
	n.Str = v.Str
}

// SetNumber replaces the node with a number value, validating the text first.
func (n *Node) SetNumber(s string) error {
	num, ok := ParseNumber(s)
	if !ok {
		return fmt.Errorf("not a valid JSON number: %q", s)
	}
	n.reset(KindNumber)
	n.Num = num
	return nil
}

// ConvertTo changes the node's kind in place, carrying the old value over on a
// best-effort basis (e.g. a number becomes the string of its digits). Changing
// the type of a non-empty container is refused, because there is no meaningful
// carry-over and it would silently discard the children.
func (n *Node) ConvertTo(k Kind) error {
	if n.Kind == k {
		return nil
	}
	if n.IsContainer() && n.ChildCount() > 0 {
		return fmt.Errorf("cannot change type of a non-empty %s", n.Kind)
	}

	text := n.scalarText()
	n.reset(k)
	switch k {
	case KindString:
		n.Str = text
	case KindNumber:
		if num, ok := ParseNumber(text); ok {
			n.Num = num
		} else {
			n.Num = "0"
		}
	case KindBool:
		n.Bool = text == "true"
	case KindNull, KindObject, KindArray:
		// no payload
	}
	return nil
}

// scalarText returns the node's value as text for type-conversion carry-over.
// Containers return "" since their content does not carry to a scalar.
func (n *Node) scalarText() string {
	switch n.Kind {
	case KindString:
		return n.Str
	case KindNumber:
		return n.Num.String()
	case KindBool:
		if n.Bool {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// reset clears all payload fields and sets the node's kind. Callers populate
// the relevant field afterward.
func (n *Node) reset(k Kind) {
	n.Kind = k
	n.Str = ""
	n.Num = ""
	n.Bool = false
	n.Items = nil
	n.Members = nil
}

// AddMember appends a key/value pair to an object. It returns an error if the
// node is not an object.
func (n *Node) AddMember(key string, value *Node) error {
	if n.Kind != KindObject {
		return fmt.Errorf("cannot add a key to a %s", n.Kind)
	}
	n.Members = append(n.Members, Member{Key: key, Value: value})
	return nil
}

// AppendItem appends a value to an array. It returns an error if the node is
// not an array.
func (n *Node) AppendItem(value *Node) error {
	if n.Kind != KindArray {
		return fmt.Errorf("cannot append to a %s", n.Kind)
	}
	n.Items = append(n.Items, value)
	return nil
}

// RemoveChild removes the child at position i from a container. For objects i
// indexes Members; for arrays it indexes Items.
func (n *Node) RemoveChild(i int) error {
	switch n.Kind {
	case KindObject:
		if i < 0 || i >= len(n.Members) {
			return fmt.Errorf("member index %d out of range", i)
		}
		n.Members = append(n.Members[:i], n.Members[i+1:]...)
	case KindArray:
		if i < 0 || i >= len(n.Items) {
			return fmt.Errorf("array index %d out of range", i)
		}
		n.Items = append(n.Items[:i], n.Items[i+1:]...)
	default:
		return fmt.Errorf("cannot remove a child from a %s", n.Kind)
	}
	return nil
}
