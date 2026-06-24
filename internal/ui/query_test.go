package ui

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
)

func num(s string) *document.Node { return document.NewNumber(json.Number(s)) }

func TestParseQueryPlainFallback(t *testing.T) {
	// No structured term: the whole string (spaces included) is one substring.
	q := parseQuery("hello world")
	if len(q.preds) != 1 || q.preds[0].kind != predPlain || q.preds[0].a != "hello world" {
		t.Fatalf("expected one plain predicate %q, got %+v", "hello world", q.preds)
	}
}

func TestParseQueryStructuredSplits(t *testing.T) {
	q := parseQuery("key:name type:string extra")
	if len(q.preds) != 3 {
		t.Fatalf("expected 3 predicates, got %d: %+v", len(q.preds), q.preds)
	}
	if q.preds[0].kind != predKey || q.preds[0].a != "name" {
		t.Errorf("pred[0] = %+v", q.preds[0])
	}
	if q.preds[1].kind != predType || q.preds[1].a != "string" {
		t.Errorf("pred[1] = %+v", q.preds[1])
	}
	if q.preds[2].kind != predPlain || q.preds[2].a != "extra" {
		t.Errorf("pred[2] = %+v", q.preds[2])
	}
}

func TestMatchNodePredicates(t *testing.T) {
	cases := []struct {
		query   string
		key     string
		hasKey  bool
		node    *document.Node
		want    bool
		comment string
	}{
		{"key:nam", "name", true, document.NewString("Ada"), true, "key substring"},
		{"key:zzz", "name", true, document.NewString("Ada"), false, "key no match"},
		{"key:name", "name", false, document.NewString("Ada"), false, "key pred needs a key"},
		{"type:string", "name", true, document.NewString("Ada"), true, "type string"},
		{"type:number", "n", true, num("42"), true, "type number"},
		{"type:int", "n", true, num("42"), true, "type alias int->number"},
		{"type:boolean", "b", true, document.NewBool(true), true, "type alias boolean->bool"},
		{"type:object", "o", true, document.NewObject(), true, "type object container"},
		{"type:number", "name", true, document.NewString("Ada"), false, "type mismatch"},
		{"value:42", "n", true, num("420"), true, "value substring on number"},
		{"value:da", "name", true, document.NewString("Ada"), true, "value substring on string (unquoted)"},
		{"value:true", "b", true, document.NewBool(true), true, "value bool literal"},
		{"name=ada", "name", true, document.NewString("Ada"), true, "key=value exact, case-insensitive"},
		{"name=ad", "name", true, document.NewString("Ada"), false, "key=value not a substring"},
		{"age=36", "age", true, num("36"), true, "key=value on number"},
		{"key:name type:string", "name", true, document.NewString("Ada"), true, "AND both satisfied"},
		{"key:name type:number", "name", true, document.NewString("Ada"), false, "AND one fails"},
		{"Ada", "name", true, document.NewString("Ada"), true, "plain matches value"},
		{"name", "name", true, num("1"), true, "plain matches key"},
	}
	for _, c := range cases {
		q := parseQuery(c.query)
		if got := q.matchNode(c.key, c.hasKey, c.node); got != c.want {
			t.Errorf("%s: query %q on key=%q node=%v => %v, want %v",
				c.comment, c.query, c.key, c.node.Kind, got, c.want)
		}
	}
}

func TestMatchNodeNilCell(t *testing.T) {
	q := parseQuery("value:x")
	if q.matchNode("col", true, nil) {
		t.Error("nil cell should not match a value predicate")
	}
	if parseQuery("type:null").matchNode("col", true, nil) {
		t.Error("nil cell has no type and should not match type:null")
	}
}

func TestDoSearchTypePredicate(t *testing.T) {
	m := New(parse(t, `{"a": "x", "n": 7, "b": true}`), "x.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "type:number")
	m = sendKey(m, "enter")
	if got := m.rows[m.cursor]; !got.HasKey || got.Key != "n" {
		t.Errorf("type:number should land on key \"n\", got %+v", got)
	}
}

func TestDoSearchKeyValuePredicate(t *testing.T) {
	m := New(parse(t, `{"a": "x", "name": "Ada", "other": "Ada"}`), "x.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	m = typeText(m, "name=ada")
	m = sendKey(m, "enter")
	if got := m.rows[m.cursor]; got.Key != "name" {
		t.Errorf("name=ada should land on key \"name\", got %+v", got)
	}
	if m.status != "found: name=ada" {
		t.Errorf("expected found status, got %q", m.status)
	}
}

func TestDoTableSearchKeyPredicate(t *testing.T) {
	m := New(parse(t, `[{"a":"apple","b":"red"},{"a":"banana","b":"yellow"}]`), "f.json")
	m = sized(m, 80, 24)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	// Match on the column name "b" with value containing "yel".
	m = typeText(m, "key:b value:yel")
	m = sendKey(m, "enter")
	if m.tableRow != 1 || m.tableCol != 1 {
		t.Errorf("expected cell (1,1), got (%d,%d)", m.tableRow, m.tableCol)
	}
}
