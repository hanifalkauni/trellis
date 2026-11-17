package trellis

import (
	"io"
	"strings"
)

// Node represents a node within a hierarchical Tree.
type Node struct {
	Label    string
	Badge    string
	Children []*Node
}

// Tree renders a hierarchical structure of Nodes with Unicode branch lines.
type Tree struct {
	root *Node
}

// NewTree constructs a new Tree rooted at label.
func NewTree(label string) *Tree {
	return &Tree{
		root: &Node{Label: label},
	}
}

// SetBadge sets an optional status badge on the root node and returns the Tree
// for fluent chaining.
func (t *Tree) SetBadge(badge string) *Tree {
	t.root.Badge = badge
	return t
}

// AddChild appends a child node under the root and returns the new Node.
func (t *Tree) AddChild(label string) *Node {
	return t.root.AddChild(label)
}

// AddChildWithBadge appends a child node with a status badge under the root
// and returns the new Node.
func (t *Tree) AddChildWithBadge(label, badge string) *Node {
	return t.root.AddChildWithBadge(label, badge)
}

// Root returns the underlying root Node of the Tree.
func (t *Tree) Root() *Node {
	return t.root
}

// SetBadge sets an optional status badge on n and returns n for fluent chaining.
func (n *Node) SetBadge(badge string) *Node {
	n.Badge = badge
	return n
}

// AddChild appends a child node under n and returns the new Node.
func (n *Node) AddChild(label string) *Node {
	child := &Node{Label: label}
	n.Children = append(n.Children, child)
	return child
}

// AddChildWithBadge appends a child node with a status badge under n and
// returns the new Node.
func (n *Node) AddChildWithBadge(label, badge string) *Node {
	child := &Node{Label: label, Badge: badge}
	n.Children = append(n.Children, child)
	return child
}

// String renders the entire tree and returns the formatted string.
func (t *Tree) String() string {
	if t == nil || t.root == nil {
		return ""
	}
	var b strings.Builder
	writeNodeLabel(&b, t.root.Label, t.root.Badge)
	b.WriteByte('\n')
	renderChildren(&b, t.root.Children, "")
	return b.String()
}

// Render writes the formatted tree to w.
func (t *Tree) Render(w io.Writer) error {
	out := t.String()
	if out == "" {
		return nil
	}
	_, err := io.WriteString(w, out)
	return err
}

func writeNodeLabel(b *strings.Builder, label, badge string) {
	b.WriteString(label)
	if badge != "" {
		b.WriteByte(' ')
		b.WriteString(badge)
	}
}

func renderChildren(b *strings.Builder, children []*Node, prefix string) {
	for i, child := range children {
		isLast := i == len(children)-1
		b.WriteString(prefix)
		if isLast {
			b.WriteString("└── ")
		} else {
			b.WriteString("├── ")
		}

		var nextPrefix string
		if isLast {
			nextPrefix = prefix + "    "
		} else {
			nextPrefix = prefix + "│   "
		}

		if strings.ContainsRune(child.Label, '\n') {
			lines := strings.Split(strings.ReplaceAll(child.Label, "\r\n", "\n"), "\n")
			writeNodeLabel(b, lines[0], child.Badge)
			b.WriteByte('\n')
			for _, ln := range lines[1:] {
				b.WriteString(nextPrefix)
				b.WriteString(ln)
				b.WriteByte('\n')
			}
		} else {
			writeNodeLabel(b, child.Label, child.Badge)
			b.WriteByte('\n')
		}

		if len(child.Children) > 0 {
			renderChildren(b, child.Children, nextPrefix)
		}
	}
}
