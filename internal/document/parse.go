package document

import (
	"encoding/json"
	"fmt"
	"io"
)

// Parse reads a single JSON value from r and builds the typed Node tree.
//
// It uses the standard library's streaming token decoder rather than a custom
// parser. We deliberately do not write our own JSON parser: encoding/json is
// battle-tested, and leaning on it means nanoj inherits correct handling of
// escaping, Unicode, and number formats for free.
//
// UseNumber keeps numeric tokens as json.Number (their original text) instead
// of converting them to float64, so values round-trip without precision loss.
func Parse(r io.Reader) (*Node, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	node, err := parseValue(dec)
	if err != nil {
		return nil, err
	}

	// A valid document is exactly one JSON value. Anything after it (other
	// than EOF) means the input was malformed or contained trailing junk.
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing data after JSON value")
	}
	return node, nil
}

// parseValue reads one complete JSON value (which may be a whole subtree).
func parseValue(dec *json.Decoder) (*Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return parseFromToken(tok, dec)
}

func parseFromToken(tok json.Token, dec *json.Decoder) (*Node, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return parseObject(dec)
		case '[':
			return parseArray(dec)
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", t)
		}
	case string:
		return NewString(t), nil
	case json.Number:
		return NewNumber(t), nil
	case bool:
		return NewBool(t), nil
	case nil:
		return NewNull(), nil
	default:
		return nil, fmt.Errorf("unexpected token %v", tok)
	}
}

func parseObject(dec *json.Decoder) (*Node, error) {
	n := NewObject()
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected object key string, got %v", keyTok)
		}
		val, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		n.Members = append(n.Members, Member{Key: key, Value: val})
	}
	// Consume the closing '}'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return n, nil
}

func parseArray(dec *json.Decoder) (*Node, error) {
	n := NewArray()
	for dec.More() {
		val, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		n.Items = append(n.Items, val)
	}
	// Consume the closing ']'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return n, nil
}
