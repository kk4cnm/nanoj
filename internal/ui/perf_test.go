package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kk4cnm/nanoj/internal/document"
)

// genRows builds the JSON text for an array of `rows` objects each with `cols`
// string columns. Used to exercise large-table behavior without a fixture file.
func genRows(rows, cols int) string {
	var b strings.Builder
	b.WriteByte('[')
	for r := 0; r < rows; r++ {
		if r > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('{')
		for c := 0; c < cols; c++ {
			if c > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`"col`)
			b.WriteString(itoa(c))
			b.WriteString(`":"r`)
			b.WriteString(itoa(r))
			b.WriteString(`c`)
			b.WriteString(itoa(c))
			b.WriteString(`"`)
		}
		b.WriteByte('}')
	}
	b.WriteByte(']')
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func mustParse(tb testing.TB, s string) *document.Node {
	tb.Helper()
	n, err := document.Parse(strings.NewReader(s))
	if err != nil {
		tb.Fatal(err)
	}
	return n
}

// TestLazyRowsInTableView confirms that opening a large array of objects in the
// table view does not eagerly flatten the whole tree, and that switching to the
// tree view realizes the rows on demand.
func TestLazyRowsInTableView(t *testing.T) {
	m := New(mustParse(t, genRows(2000, 4)), "big.json")
	if m.view != viewTable {
		t.Fatal("expected table view for an array of objects")
	}
	if !m.rowsStale || len(m.rows) != 0 {
		t.Errorf("tree rows should be deferred in table view, got stale=%v len=%d", m.rowsStale, len(m.rows))
	}
	// Switching to the tree view builds them.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.rowsStale || len(m.rows) == 0 {
		t.Errorf("exiting to tree should realize rows, got stale=%v len=%d", m.rowsStale, len(m.rows))
	}
}

// BenchmarkTableNavigate is a baseline for per-keystroke cost in a large table.
// Run with: go test ./internal/ui -bench BenchmarkTableNavigate -run x
func BenchmarkTableNavigate(b *testing.B) {
	m := New(mustParse(b, genRows(50000, 8)), "big.json")
	m = sized(m, 100, 40)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = u.(Model)
		_ = m.View()
	}
}
