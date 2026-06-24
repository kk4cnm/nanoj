package document

import (
	"strings"
	"testing"
)

func TestStripJSONCComments(t *testing.T) {
	in := `{
		// a line comment
		"a": 1, /* inline block */ "b": 2,
		/* multi
		   line */
		"c": 3
	}`
	cleaned, had := StripJSONC([]byte(in))
	if !had {
		t.Error("expected hadComments to be true")
	}
	node, err := Parse(strings.NewReader(string(cleaned)))
	if err != nil {
		t.Fatalf("cleaned JSON should parse: %v\n%s", err, cleaned)
	}
	if node.ChildCount() != 3 {
		t.Errorf("expected 3 members, got %d", node.ChildCount())
	}
}

func TestStripJSONCTrailingCommas(t *testing.T) {
	cases := []string{
		`[1, 2, 3,]`,
		`{"a": 1, "b": 2,}`,
		`{"a": [1, 2,], "b": {"c": 3,},}`,
		"[1, 2,\n]",         // comma, whitespace, then close
		"{\"a\":1, // c\n}", // trailing comma revealed after comment removal
	}
	for _, in := range cases {
		cleaned, _ := StripJSONC([]byte(in))
		if _, err := Parse(strings.NewReader(string(cleaned))); err != nil {
			t.Errorf("expected %q to parse after cleaning, got %v (cleaned: %q)", in, err, cleaned)
		}
	}
}

// TestStripJSONCPreservesStrings is the important safety test: comment markers
// and commas inside string values must NOT be touched.
func TestStripJSONCPreservesStrings(t *testing.T) {
	in := `{"url": "http://example.com/path", "note": "not /* a */ comment, really", "trailing": "a,"}`
	cleaned, had := StripJSONC([]byte(in))
	if had {
		t.Error("there are no real comments here; hadComments should be false")
	}
	node, err := Parse(strings.NewReader(string(cleaned)))
	if err != nil {
		t.Fatalf("parse failed: %v\n%s", err, cleaned)
	}
	if got := node.Members[0].Value.Str; got != "http://example.com/path" {
		t.Errorf("URL string was corrupted: %q", got)
	}
	if got := node.Members[1].Value.Str; got != "not /* a */ comment, really" {
		t.Errorf("comment-like string content was corrupted: %q", got)
	}
	if got := node.Members[2].Value.Str; got != "a," {
		t.Errorf("trailing-comma-in-string was corrupted: %q", got)
	}
}

func TestStripJSONCEscapedQuotes(t *testing.T) {
	// An escaped quote must not end the string early (which would expose the
	// following // to comment-stripping).
	in := `{"s": "he said \"// hi\"", "n": 2}`
	cleaned, had := StripJSONC([]byte(in))
	if had {
		t.Error("no real comments; hadComments should be false")
	}
	node, err := Parse(strings.NewReader(string(cleaned)))
	if err != nil {
		t.Fatalf("parse failed: %v\n%s", err, cleaned)
	}
	if node.Members[0].Value.Str != `he said "// hi"` {
		t.Errorf("escaped-quote string corrupted: %q", node.Members[0].Value.Str)
	}
}

func TestStripJSONCNoChangeForStrictJSON(t *testing.T) {
	in := `{"a":1,"b":[true,null,"x"],"c":{"d":2}}`
	cleaned, had := StripJSONC([]byte(in))
	if had {
		t.Error("strict JSON has no comments")
	}
	if string(cleaned) != in {
		t.Errorf("strict JSON should be unchanged\n in: %s\nout: %s", in, cleaned)
	}
}

func TestParseLenientReportsComments(t *testing.T) {
	withComments := `{
		"a": 1 // the value of a
	}`
	node, had, err := ParseLenient(strings.NewReader(withComments))
	if err != nil {
		t.Fatal(err)
	}
	if !had {
		t.Error("expected hadComments true")
	}
	if node.Members[0].Value.Num.String() != "1" {
		t.Errorf("value not parsed: %v", node.Members[0].Value)
	}

	_, had2, err := ParseLenient(strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if had2 {
		t.Error("plain JSON should report hadComments false")
	}
}

// TestParseLenientStillStrictOutput confirms a leniently-parsed doc serializes
// back to clean, comment-free, valid JSON.
func TestParseLenientStillStrictOutput(t *testing.T) {
	node, _, err := ParseLenient(strings.NewReader(`{
		"name": "Ada", // first name
		"tags": ["x", "y",], /* trailing comma above */
	}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := node.Marshal("  ")
	if err != nil {
		t.Fatal(err)
	}
	// Re-parse strictly: the output must be valid JSON with no comments.
	if _, err := Parse(strings.NewReader(string(out))); err != nil {
		t.Errorf("lenient output is not strict JSON: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "//") || strings.Contains(string(out), "/*") {
		t.Errorf("output still contains comment markers:\n%s", out)
	}
}
