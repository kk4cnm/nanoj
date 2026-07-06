package convert

import (
	"strings"
	"testing"

	"github.com/kk4cnm/nanoj/internal/document"
)

// marshal renders a tree compactly so tests can assert on exact JSON output —
// which also proves the conversion produced a serializable, valid tree.
func marshal(t *testing.T, n *document.Node) string {
	t.Helper()
	out, err := n.Marshal("")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return strings.Join(strings.Fields(string(out)), "")
}

// --- YAML ---

func TestYAMLBasicTypes(t *testing.T) {
	n, err := FromYAML(strings.NewReader(`
name: Ada
age: 36
ratio: 0.5
admin: true
nick: null
tags: [x, y]
`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"Ada","age":36,"ratio":0.5,"admin":true,"nick":null,"tags":["x","y"]}`
	if got := marshal(t, n); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestYAMLPreservesKeyOrder(t *testing.T) {
	n, err := FromYAML(strings.NewReader("zebra: 1\napple: 2\nmango: 3\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := marshal(t, n); got != `{"zebra":1,"apple":2,"mango":3}` {
		t.Errorf("order not preserved: %s", got)
	}
}

func TestYAMLNumberSpellings(t *testing.T) {
	// Hex/octal/underscore spellings are YAML-only; they must come out as
	// canonical JSON numbers.
	n, err := FromYAML(strings.NewReader("hex: 0x1A\noct: 0o17\nbig: 12345678901234567890\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"hex":26,"oct":15,"big":12345678901234567890}`
	if got := marshal(t, n); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestYAMLAnchorsExpand(t *testing.T) {
	n, err := FromYAML(strings.NewReader(`
base: &b
  retries: 3
prod: *b
`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"base":{"retries":3},"prod":{"retries":3}}`
	if got := marshal(t, n); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestYAMLTimestampAndTagsBecomeStrings(t *testing.T) {
	n, err := FromYAML(strings.NewReader("when: 2026-07-06T10:00:00Z\nquoted: \"123\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"when":"2026-07-06T10:00:00Z","quoted":"123"}`
	if got := marshal(t, n); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestYAMLInfinityRejected(t *testing.T) {
	if _, err := FromYAML(strings.NewReader("bad: .inf\n")); err == nil {
		t.Error("expected an error for .inf, got none")
	}
}

func TestYAMLNonScalarKeyRejected(t *testing.T) {
	if _, err := FromYAML(strings.NewReader("? [a, b]\n: 1\n")); err == nil {
		t.Error("expected an error for a sequence key, got none")
	}
}

func TestYAMLEmptyDocument(t *testing.T) {
	n, err := FromYAML(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != document.KindNull {
		t.Errorf("empty document should be null, got kind %v", n.Kind)
	}
}

// --- TOML ---

func TestTOMLBasicTypes(t *testing.T) {
	n, err := FromTOML(strings.NewReader(`
name = "Ada"
age = 36
ratio = 0.5
admin = true
tags = ["x", "y"]
`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"Ada","age":36,"ratio":0.5,"admin":true,"tags":["x","y"]}`
	if got := marshal(t, n); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestTOMLPreservesKeyOrder(t *testing.T) {
	n, err := FromTOML(strings.NewReader(`
zebra = 1
apple = 2

[server]
port = 8080
host = "localhost"
`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"zebra":1,"apple":2,"server":{"port":8080,"host":"localhost"}}`
	if got := marshal(t, n); got != want {
		t.Errorf("order not preserved: %s", got)
	}
}

func TestTOMLArrayOfTables(t *testing.T) {
	n, err := FromTOML(strings.NewReader(`
[[servers]]
name = "a"
port = 80

[[servers]]
name = "b"
port = 81
`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"servers":[{"name":"a","port":80},{"name":"b","port":81}]}`
	if got := marshal(t, n); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestTOMLDatetimeBecomesString(t *testing.T) {
	n, err := FromTOML(strings.NewReader("when = 2026-07-06T10:00:00Z\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := marshal(t, n); got != `{"when":"2026-07-06T10:00:00Z"}` {
		t.Errorf("got %s", got)
	}
}

func TestTOMLInfinityRejected(t *testing.T) {
	if _, err := FromTOML(strings.NewReader("bad = inf\n")); err == nil {
		t.Error("expected an error for inf, got none")
	}
}

func TestTOMLInlineTable(t *testing.T) {
	n, err := FromTOML(strings.NewReader(`point = { y = 1, x = 2 }` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := marshal(t, n); got != `{"point":{"y":1,"x":2}}` {
		t.Errorf("got %s", got)
	}
}
