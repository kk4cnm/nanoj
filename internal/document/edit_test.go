package document

import (
	"strings"
	"testing"
)

func TestParseNumber(t *testing.T) {
	ok := []string{"0", "42", "-17", "3.14", "1e10", "-2.5E-3", "12345678901234567890", "  7  "}
	for _, s := range ok {
		if _, valid := ParseNumber(s); !valid {
			t.Errorf("ParseNumber(%q) should be valid", s)
		}
	}
	bad := []string{"", "abc", "1.2.3", "1,000", "0x1F", "1 2", "true", "--1", "."}
	for _, s := range bad {
		if _, valid := ParseNumber(s); valid {
			t.Errorf("ParseNumber(%q) should be invalid", s)
		}
	}
}

func TestSetNumberRejectsGarbage(t *testing.T) {
	n := NewString("x")
	if err := n.SetNumber("not a number"); err == nil {
		t.Fatal("expected error for non-numeric input")
	}
	if n.Kind != KindString {
		t.Errorf("node should be unchanged on invalid input, got %s", n.Kind)
	}
	if err := n.SetNumber("99"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Kind != KindNumber || n.Num.String() != "99" {
		t.Errorf("expected number 99, got %s %s", n.Kind, n.Num)
	}
}

func TestScalarFromInput(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
	}{
		{"", KindNull},
		{"null", KindNull},
		{"true", KindBool},
		{"false", KindBool},
		{"42", KindNumber},
		{"-3.14", KindNumber},
		{"hello", KindString},
		{"123abc", KindString},
		{"  ", KindNull}, // whitespace-only trims to empty
	}
	for _, c := range cases {
		if got := ScalarFromInput(c.in).Kind; got != c.kind {
			t.Errorf("ScalarFromInput(%q).Kind = %s, want %s", c.in, got, c.kind)
		}
	}
	if n := ScalarFromInput("true"); !n.Bool {
		t.Error("ScalarFromInput(\"true\") should be bool true")
	}
	if n := ScalarFromInput("  spaced  "); n.Str != "  spaced  " {
		t.Errorf("string should preserve original text, got %q", n.Str)
	}
}

func TestSetScalarInPlace(t *testing.T) {
	n := NewNull()
	n.SetScalar("99")
	if n.Kind != KindNumber || n.Num.String() != "99" {
		t.Errorf("expected number 99, got %s %s", n.Kind, n.Num)
	}
	n.SetScalar("hi")
	if n.Kind != KindString || n.Str != "hi" {
		t.Errorf("expected string hi, got %s %q", n.Kind, n.Str)
	}
}

func TestConvertScalarCarryOver(t *testing.T) {
	// number -> string keeps the digits
	n := NewNumber("42")
	if err := n.ConvertTo(KindString); err != nil {
		t.Fatal(err)
	}
	if n.Kind != KindString || n.Str != "42" {
		t.Errorf("expected string \"42\", got %s %q", n.Kind, n.Str)
	}

	// non-numeric string -> number falls back to 0
	s := NewString("hello")
	if err := s.ConvertTo(KindNumber); err != nil {
		t.Fatal(err)
	}
	if s.Num.String() != "0" {
		t.Errorf("expected fallback 0, got %s", s.Num)
	}

	// "true" string -> bool true
	b := NewString("true")
	if err := b.ConvertTo(KindBool); err != nil {
		t.Fatal(err)
	}
	if !b.Bool {
		t.Errorf("expected bool true")
	}
}

func TestConvertRefusesNonEmptyContainer(t *testing.T) {
	root, err := Parse(strings.NewReader(`{"a": 1}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := root.ConvertTo(KindString); err == nil {
		t.Error("expected refusal to convert a non-empty object")
	}
	// An empty object may be converted.
	empty := NewObject()
	if err := empty.ConvertTo(KindArray); err != nil {
		t.Errorf("empty container conversion should succeed: %v", err)
	}
}

func TestAddAndRemoveChildren(t *testing.T) {
	obj := NewObject()
	if err := obj.AddMember("x", NewNull()); err != nil {
		t.Fatal(err)
	}
	if obj.ChildCount() != 1 || obj.Members[0].Key != "x" {
		t.Errorf("AddMember failed: %+v", obj.Members)
	}
	if err := obj.AppendItem(NewNull()); err == nil {
		t.Error("AppendItem on object should fail")
	}

	arr := NewArray()
	arr.AppendItem(NewBool(true))
	arr.AppendItem(NewBool(false))
	if err := arr.RemoveChild(0); err != nil {
		t.Fatal(err)
	}
	if arr.ChildCount() != 1 || arr.Items[0].Bool != false {
		t.Errorf("RemoveChild failed: %+v", arr.Items)
	}
	if err := arr.RemoveChild(5); err == nil {
		t.Error("out-of-range RemoveChild should fail")
	}
}
