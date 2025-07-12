package hierarchy

import (
	"testing"
)

func TestNode_SortChildren_NilSafety(t *testing.T) {
	tests := []struct {
		name string
		node *Node
	}{
		{
			name: "nil node",
			node: nil,
		},
		{
			name: "node with nil children",
			node: &Node{
				Name:     "test",
				Children: nil,
			},
		},
		{
			name: "node with empty children",
			node: &Node{
				Name:     "test",
				Children: []*Node{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			tt.node.SortChildren()
		})
	}
}
