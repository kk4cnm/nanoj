package document

// Clone returns a deep copy of the node and all of its descendants. It is used
// for undo/redo snapshots, where each history entry must be fully independent
// of the live tree so later edits don't mutate the saved state.
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	c := &Node{
		Kind: n.Kind,
		Bool: n.Bool,
		Num:  n.Num,
		Str:  n.Str,
	}
	switch n.Kind {
	case KindArray:
		c.Items = make([]*Node, len(n.Items))
		for i, item := range n.Items {
			c.Items[i] = item.Clone()
		}
	case KindObject:
		c.Members = make([]Member, len(n.Members))
		for i, m := range n.Members {
			c.Members[i] = Member{Key: m.Key, Value: m.Value.Clone()}
		}
	}
	return c
}
