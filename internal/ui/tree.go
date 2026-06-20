// Package ui implements nanoj's interactive terminal interface using the
// Bubble Tea framework. It renders the document.Node tree as a navigable,
// collapsible outline (the Phase 1 "tree view") and translates keystrokes
// into edits of the model.
//
// The rendering pipeline has two stages kept deliberately separate so the
// hard part stays unit-testable without a real terminal:
//
//  1. Flatten: turn the tree (plus a set of collapsed paths) into a flat slice
//     of visible Rows. Pure data, no styling.
//  2. View: turn Rows into a styled string for the current viewport.
//
// Expansion state is tracked by structural path (e.g. "/1/0") rather than by
// node pointer. Paths survive the deep-cloning used for undo/redo, and they
// belong to the view rather than the shared document model — keeping the door
// open for a future table view that ignores expansion entirely.
package ui

import (
	"strconv"

	"github.com/kk4cnm/nanoj/internal/document"
)

// Row is one visible line in the tree view: a single node together with the
// context needed to render and act on it (its parent, its position within that
// parent, its key or array index, its structural path, and its depth).
type Row struct {
	Node   *document.Node
	Parent *document.Node // nil for the root
	HasKey bool           // true when this node is a member of an object
	Key    string         // object key (valid when HasKey)
	Index  int            // position within the parent (member index or array index); 0 for root
	Path   string         // structural path, e.g. "" for root, "/2/0" for a nested child
	Depth  int            // nesting depth; root is 0
}

// Flatten walks the tree and returns the rows that are currently visible. A
// container whose path is in collapsed is shown but its children are hidden.
// A nil collapsed map means everything is expanded (used for whole-tree search).
func Flatten(root *document.Node, collapsed map[string]bool) []Row {
	var rows []Row
	var walk func(n, parent *document.Node, hasKey bool, key string, index int, path string, depth int)
	walk = func(n, parent *document.Node, hasKey bool, key string, index int, path string, depth int) {
		rows = append(rows, Row{
			Node:   n,
			Parent: parent,
			HasKey: hasKey,
			Key:    key,
			Index:  index,
			Path:   path,
			Depth:  depth,
		})
		if !n.IsContainer() || collapsed[path] {
			return
		}
		switch n.Kind {
		case document.KindObject:
			for i, m := range n.Members {
				walk(m.Value, n, true, m.Key, i, childPath(path, i), depth+1)
			}
		case document.KindArray:
			for i, item := range n.Items {
				walk(item, n, false, "", i, childPath(path, i), depth+1)
			}
		}
	}
	walk(root, nil, false, "", 0, "", 0)
	return rows
}

// childPath builds the path of the index-th child of the node at parentPath.
// Using the child index (not the key) keeps paths unambiguous even when object
// keys contain slashes or are duplicated.
func childPath(parentPath string, index int) string {
	return parentPath + "/" + strconv.Itoa(index)
}
