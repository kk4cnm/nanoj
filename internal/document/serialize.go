package document

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Marshal renders the tree as pretty-printed JSON using the given indent unit
// (for example "  " for two spaces or "\t" for a tab). The output ends with a
// trailing newline, matching the convention of most editors and formatters.
//
// Because Marshal walks the typed tree and emits each piece itself, its output
// is well-formed JSON by construction: there is no code path that can produce
// an unbalanced brace or a stray comma.
func (n *Node) Marshal(indent string) ([]byte, error) {
	var buf bytes.Buffer
	if err := n.write(&buf, indent, 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func (n *Node) write(buf *bytes.Buffer, indent string, depth int) error {
	switch n.Kind {
	case KindNull:
		buf.WriteString("null")
	case KindBool:
		if n.Bool {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case KindNumber:
		s := n.Num.String()
		if s == "" {
			return fmt.Errorf("number node has empty value")
		}
		buf.WriteString(s)
	case KindString:
		if err := writeJSONString(buf, n.Str); err != nil {
			return err
		}
	case KindArray:
		return n.writeArray(buf, indent, depth)
	case KindObject:
		return n.writeObject(buf, indent, depth)
	default:
		return fmt.Errorf("cannot serialize unknown node kind %d", n.Kind)
	}
	return nil
}

func (n *Node) writeArray(buf *bytes.Buffer, indent string, depth int) error {
	if len(n.Items) == 0 {
		buf.WriteString("[]")
		return nil
	}
	buf.WriteString("[\n")
	for i, item := range n.Items {
		writeIndent(buf, indent, depth+1)
		if err := item.write(buf, indent, depth+1); err != nil {
			return err
		}
		if i < len(n.Items)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, indent, depth)
	buf.WriteByte(']')
	return nil
}

func (n *Node) writeObject(buf *bytes.Buffer, indent string, depth int) error {
	if len(n.Members) == 0 {
		buf.WriteString("{}")
		return nil
	}
	buf.WriteString("{\n")
	for i, m := range n.Members {
		writeIndent(buf, indent, depth+1)
		if err := writeJSONString(buf, m.Key); err != nil {
			return err
		}
		buf.WriteString(": ")
		if err := m.Value.write(buf, indent, depth+1); err != nil {
			return err
		}
		if i < len(n.Members)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeIndent(buf, indent, depth)
	buf.WriteByte('}')
	return nil
}

func writeIndent(buf *bytes.Buffer, indent string, depth int) {
	for i := 0; i < depth; i++ {
		buf.WriteString(indent)
	}
}

// writeJSONString emits a properly escaped JSON string. We route through an
// encoding/json Encoder (rather than building the escapes by hand) so the
// escaping is exactly correct. HTML escaping is disabled so that characters
// like <, > and & are written literally, which keeps human-facing JSON
// readable instead of full of < sequences.
func writeJSONString(buf *bytes.Buffer, s string) error {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	// Encoder.Encode appends a newline; strip it.
	buf.Write(bytes.TrimRight(tmp.Bytes(), "\n"))
	return nil
}
