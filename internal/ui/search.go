package ui

import (
	"strings"

	"github.com/kk4cnm/nanoj/internal/document"
)

// beginSearch opens nano's "Where Is" prompt, prefilled with the previous
// query so pressing Enter repeats the last search.
func (m *Model) beginSearch() {
	m.beginInput(actSearch, "Search: ", m.lastSearch)
}

// doSearch finds the next node (forward from the cursor, wrapping) whose key or
// scalar value contains query, case-insensitively. Matches inside collapsed
// containers are revealed by expanding their ancestors. The whole tree is
// searched, not just the visible rows.
func (m *Model) doSearch(query string) {
	if strings.TrimSpace(query) == "" {
		return
	}
	m.lastSearch = query
	needle := strings.ToLower(query)

	// Flatten with no collapsing so every node is searchable.
	all := Flatten(m.root, nil)

	// Locate the current selection within the full ordering.
	start := 0
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		curPath := m.rows[m.cursor].Path
		for i, r := range all {
			if r.Path == curPath {
				start = i
				break
			}
		}
	}

	n := len(all)
	for off := 1; off <= n; off++ {
		r := all[(start+off)%n]
		if rowMatches(r, needle) {
			m.revealPath(r.Path)
			m.rebuild()
			m.selectByPath(r.Path)
			m.status = "found: " + query
			return
		}
	}
	m.status = "not found: " + query
}

// doTableSearch finds the next cell (scanning row-major, forward from the
// selected cell and wrapping) whose scalar value contains query,
// case-insensitively, and moves the table cursor to it — scrolling as needed so
// the match becomes visible.
func (m *Model) doTableSearch(query string) {
	if strings.TrimSpace(query) == "" {
		return
	}
	m.lastSearch = query
	needle := strings.ToLower(query)

	rows := len(m.table.array.Items)
	cols := len(m.table.columns)
	total := rows * cols
	if total == 0 {
		return
	}

	start := m.tableRow*cols + m.tableCol
	for off := 1; off <= total; off++ {
		idx := (start + off) % total
		r, c := idx/cols, idx%cols
		if cellMatches(m.table.cellNode(r, c), needle) {
			m.tableRow, m.tableCol = r, c
			m.clampTableScroll()
			m.status = "found: " + query
			return
		}
	}
	m.status = "not found: " + query
}

// cellMatches reports whether a scalar cell's value contains needle (already
// lower-cased). Container cells (shown as placeholders) are not searched.
func cellMatches(n *document.Node, needle string) bool {
	if n == nil || n.IsContainer() {
		return false
	}
	return strings.Contains(strings.ToLower(scalarText(n)), needle)
}

// rowMatches reports whether the row's key or scalar value contains needle
// (which must already be lower-cased).
func rowMatches(r Row, needle string) bool {
	if r.HasKey && strings.Contains(strings.ToLower(r.Key), needle) {
		return true
	}
	if !r.Node.IsContainer() {
		if strings.Contains(strings.ToLower(scalarText(r.Node)), needle) {
			return true
		}
	}
	return false
}

// revealPath expands every ancestor of path so the target row becomes visible.
func (m *Model) revealPath(path string) {
	for c := range m.collapsed {
		if c == path || strings.HasPrefix(path, c+"/") {
			delete(m.collapsed, c)
		}
	}
}

// selectByPath moves the cursor to the visible row with the given path.
func (m *Model) selectByPath(path string) {
	for i, r := range m.rows {
		if r.Path == path {
			m.cursor = i
			m.clampScroll()
			return
		}
	}
}
