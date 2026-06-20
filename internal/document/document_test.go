package document

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRoundTrip verifies that parsing a document and serializing it again
// produces JSON that is semantically identical to the input. We compare via
// encoding/json's generic decoding (which ignores formatting and key order)
// so the test checks meaning, not byte-for-byte layout.
func TestRoundTrip(t *testing.T) {
	cases := []string{
		`null`,
		`true`,
		`false`,
		`42`,
		`-17.5`,
		`12345678901234567890`, // larger than float64 can hold exactly
		`"hello"`,
		`""`,
		`"with \"quotes\" and \n newline"`,
		`"unicode: é 中"`,
		`[]`,
		`{}`,
		`[1, 2, 3]`,
		`{"a": 1, "b": [true, null, "x"], "c": {"nested": {}}}`,
		`{"emptyArr": [], "emptyObj": {}}`,
		`[{"id": 1, "name": "Ada"}, {"id": 2, "name": "Linus"}]`,
	}

	for _, in := range cases {
		node, err := Parse(strings.NewReader(in))
		if err != nil {
			t.Errorf("Parse(%s) failed: %v", in, err)
			continue
		}
		out, err := node.Marshal("  ")
		if err != nil {
			t.Errorf("Marshal of %s failed: %v", in, err)
			continue
		}
		if !semanticallyEqual(t, in, string(out)) {
			t.Errorf("round-trip mismatch\n  in:  %s\n  out: %s", in, out)
		}
	}
}

// TestKeyOrderPreserved confirms objects keep their original key order, which
// matters for keeping the user's files stable across edits.
func TestKeyOrderPreserved(t *testing.T) {
	in := `{"zebra": 1, "apple": 2, "mango": 3}`
	node, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	out, err := node.Marshal("  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	zi := strings.Index(string(out), "zebra")
	ai := strings.Index(string(out), "apple")
	mi := strings.Index(string(out), "mango")
	if !(zi < ai && ai < mi) {
		t.Errorf("key order not preserved: %s", out)
	}
}

// TestNumberPrecision confirms big integers survive without float64 rounding.
func TestNumberPrecision(t *testing.T) {
	in := `12345678901234567890`
	node, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if got := node.Num.String(); got != in {
		t.Errorf("number precision lost: got %s want %s", got, in)
	}
}

// TestRejectMalformed confirms that invalid JSON is refused at parse time, so
// nothing malformed ever enters the model.
func TestRejectMalformed(t *testing.T) {
	bad := []string{
		`{`,
		`[1, 2,]`,
		`{"a": }`,
		`{"a" 1}`,
		`tru`,
		`{"a": 1} extra`,
		`"unterminated`,
		``,
	}
	for _, in := range bad {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("expected Parse(%q) to fail, but it succeeded", in)
		}
	}
}

// TestHTMLCharsNotEscaped confirms <, > and & are written literally.
func TestHTMLCharsNotEscaped(t *testing.T) {
	node := NewString("a < b && c > d")
	out, err := node.Marshal("  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// If HTML escaping were active, '<' '>' and '&' would be rewritten as
	// \uXXXX sequences and the literal characters would be absent. Their
	// presence therefore proves escaping is disabled.
	for _, lit := range []string{"<", ">", "&"} {
		if !strings.Contains(string(out), lit) {
			t.Errorf("expected literal %q in output (HTML escaping not disabled?): %s", lit, out)
		}
	}
}

// semanticallyEqual reports whether two JSON texts decode to equal values,
// ignoring formatting and object key order.
func semanticallyEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal([]byte(a), &av); err != nil {
		t.Fatalf("test input not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(b), &bv); err != nil {
		t.Fatalf("our output not valid JSON: %v (%s)", err, b)
	}
	return jsonEqual(av, bv)
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
