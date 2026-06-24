package ui

import (
	"strings"
)

// beginSearch opens nano's "Where Is" prompt, prefilled with the previous
// query so pressing Enter repeats the last search.
func (m *Model) beginSearch() {
	m.beginInput(actSearch, "Search: ", m.lastSearch)
}

// doSearch finds the next node (forward from the cursor, wrapping) that matches
// query. A bare query is a key-or-value substring; a query using the predicate
// grammar (key:/type:/value:/key=value, see parseQuery) matches structurally.
// Matches inside collapsed containers are revealed by expanding their
// ancestors. The whole tree is searched, not just the visible rows.
func (m *Model) doSearch(query string) {
	if strings.TrimSpace(query) == "" {
		return
	}
	m.lastSearch = query
	q := parseQuery(query)
	if q.empty() {
		return
	}

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
		if q.matchNode(r.Key, r.HasKey, r.Node) {
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
// selected cell and wrapping) that matches query and moves the table cursor to
// it — scrolling as needed so the match becomes visible. A cell's "key" for
// predicate matching is its column name, so key:/key=value terms work too.
func (m *Model) doTableSearch(query string) {
	if strings.TrimSpace(query) == "" {
		return
	}
	m.lastSearch = query
	q := parseQuery(query)
	if q.empty() {
		return
	}

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
		if q.matchNode(m.table.columns[c], true, m.table.cellNode(r, c)) {
			m.tableRow, m.tableCol = r, c
			m.clampTableScroll()
			m.status = "found: " + query
			return
		}
	}
	m.status = "not found: " + query
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
