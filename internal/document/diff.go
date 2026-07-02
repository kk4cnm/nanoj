package document

import "strconv"

// This file implements a structural diff between two document trees. It powers
// the editor's read-only diff overlay: the result is keyed by the same
// index-based structural path the views use (e.g. "/0/2"), so the UI can mark
// rows without mutating the tree.
//
// Objects are compared by key; arrays are compared positionally (index by
// index). Array move/LCS detection is deliberately out of scope — a reordered
// array reads as a run of changes, which is honest and predictable for v1.

// DiffKind classifies a node in the working tree relative to the baseline.
type DiffKind int

const (
	DiffNone    DiffKind = iota // unchanged
	DiffAdded                   // present in working, absent in baseline
	DiffChanged                 // present in both but the scalar value or type differs
)

// DiffResult holds the per-path classification plus summary counts. Added and
// Changed count subtree roots in the working tree; Removed counts subtree roots
// that exist only in the baseline (these have no path in the working tree, so
// they cannot be marked inline — the count surfaces them instead).
type DiffResult struct {
	ByPath  map[string]DiffKind
	Added   int
	Changed int
	Removed int
}

// Kind returns the classification for a path (DiffNone if absent).
func (d DiffResult) Kind(path string) DiffKind { return d.ByPath[path] }

// Diff compares working against baseline and returns the classification.
func Diff(baseline, working *Node) DiffResult {
	d := DiffResult{ByPath: map[string]DiffKind{}}
	d.diff(baseline, working, "")
	return d
}

func (d *DiffResult) diff(old, new *Node, path string) {
	switch {
	case old == nil && new == nil:
		return
	case old == nil:
		// Newly present subtree.
		d.Added++
		d.markAll(new, path, DiffAdded)
		return
	case new == nil:
		// Present only in the baseline; no working path to mark.
		d.Removed++
		return
	}

	if old.Kind != new.Kind {
		d.ByPath[path] = DiffChanged
		d.Changed++
		return
	}

	switch new.Kind {
	case KindObject:
		d.diffObject(old, new, path)
	case KindArray:
		d.diffArray(old, new, path)
	default:
		if !scalarEqual(old, new) {
			d.ByPath[path] = DiffChanged
			d.Changed++
		}
	}
}

func (d *DiffResult) diffObject(old, new *Node, path string) {
	// Index baseline members by key (last one wins, matching map semantics).
	oldByKey := map[string]*Node{}
	for _, m := range old.Members {
		oldByKey[m.Key] = m.Value
	}
	newKeys := map[string]bool{}
	for i, m := range new.Members {
		newKeys[m.Key] = true
		cp := childPath(path, i)
		if ov, ok := oldByKey[m.Key]; ok {
			d.diff(ov, m.Value, cp)
		} else {
			d.Added++
			d.markAll(m.Value, cp, DiffAdded)
		}
	}
	for _, m := range old.Members {
		if !newKeys[m.Key] {
			d.Removed++
		}
	}
}

func (d *DiffResult) diffArray(old, new *Node, path string) {
	for i := 0; i < len(new.Items); i++ {
		cp := childPath(path, i)
		if i < len(old.Items) {
			d.diff(old.Items[i], new.Items[i], cp)
		} else {
			d.Added++
			d.markAll(new.Items[i], cp, DiffAdded)
		}
	}
	if len(old.Items) > len(new.Items) {
		d.Removed += len(old.Items) - len(new.Items)
	}
}

// markAll tags a node and all its descendants with kind, so every row of an
// added subtree carries the marker.
func (d *DiffResult) markAll(n *Node, path string, kind DiffKind) {
	d.ByPath[path] = kind
	switch n.Kind {
	case KindObject:
		for i, m := range n.Members {
			d.markAll(m.Value, childPath(path, i), kind)
		}
	case KindArray:
		for i, item := range n.Items {
			d.markAll(item, childPath(path, i), kind)
		}
	}
}

func childPath(parent string, index int) string {
	return parent + "/" + strconv.Itoa(index)
}

// scalarEqual reports whether two non-container nodes have the same value.
// Numbers compare by their preserved text, so 1 and 1.0 read as changed —
// intentional, since the file bytes differ.
func scalarEqual(a, b *Node) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindNull:
		return true
	case KindBool:
		return a.Bool == b.Bool
	case KindNumber:
		return a.Num.String() == b.Num.String()
	case KindString:
		return a.Str == b.Str
	default:
		return false
	}
}
