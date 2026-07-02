package document

import (
	"encoding/json"
	"strconv"
)

// ToInterface converts the node into the generic Go value shapes that
// encoding/json and JSON Schema validators expect: nil, bool, json.Number,
// string, []any, and map[string]any. Numbers are kept as json.Number so their
// exact text is preserved for validation (e.g. integer vs. number checks).
//
// Objects with duplicate keys collapse to the last value, as a Go map cannot
// hold duplicates; this matches how a standard decoder would see them.
func (n *Node) ToInterface() any {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case KindNull:
		return nil
	case KindBool:
		return n.Bool
	case KindNumber:
		return n.Num
	case KindString:
		return n.Str
	case KindArray:
		out := make([]any, len(n.Items))
		for i, item := range n.Items {
			out[i] = item.ToInterface()
		}
		return out
	case KindObject:
		out := make(map[string]any, len(n.Members))
		for _, m := range n.Members {
			out[m.Key] = m.Value.ToInterface()
		}
		return out
	default:
		return nil
	}
}

// FromGo builds a Node from the generic Go value shapes produced by a JSON
// decoder (nil, bool, json.Number, float64/int, string, []any, map[string]any).
// It is the inverse of ToInterface and is used to turn schema-supplied values
// (such as enum entries) into tree nodes. Unknown types become null.
func FromGo(v any) *Node {
	switch x := v.(type) {
	case nil:
		return NewNull()
	case bool:
		return NewBool(x)
	case json.Number:
		return NewNumber(x)
	case string:
		return NewString(x)
	case float64:
		return NewNumber(json.Number(strconv.FormatFloat(x, 'g', -1, 64)))
	case int:
		return NewNumber(json.Number(strconv.Itoa(x)))
	case []any:
		arr := NewArray()
		for _, item := range x {
			arr.Items = append(arr.Items, FromGo(item))
		}
		return arr
	case map[string]any:
		obj := NewObject()
		for k, val := range x {
			obj.Members = append(obj.Members, Member{Key: k, Value: FromGo(val)})
		}
		return obj
	default:
		return NewNull()
	}
}
