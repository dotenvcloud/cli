package hierarchy

import (
	"testing"
)


func TestNode_Walk(t *testing.T) {
	// Create a test hierarchy
	root := &Node{
		Type: NodeTypeOrganization,
		Name: "org",
		Path: "/",
		Children: []*Node{
			{
				Type: NodeTypeProject,
				Name: "project1",
				Path: "project1",
				Children: []*Node{
					{
						Type: NodeTypeTarget,
						Name: "target1",
						Path: "project1/target1",
						Children: []*Node{
							{
								Type:     NodeTypeEnvironment,
								Name:     "env1",
								Path:     "project1/target1/env1",
								Children: []*Node{},
							},
						},
					},
				},
			},
		},
	}

	// Test walking the tree
	var visited []string
	err := root.Walk(func(n *Node) error {
		visited = append(visited, n.Path)
		return nil
	})

	if err != nil {
		t.Errorf("Walk() error = %v", err)
	}

	expected := []string{"/", "project1", "project1/target1", "project1/target1/env1"}
	if len(visited) != len(expected) {
		t.Errorf("Walk() visited %d nodes, expected %d", len(visited), len(expected))
	}

	for i, path := range expected {
		if i >= len(visited) || visited[i] != path {
			t.Errorf("Walk() visited[%d] = %v, want %v", i, visited[i], path)
		}
	}
}

func TestNode_Find(t *testing.T) {
	// Create a test hierarchy
	env := &Node{
		Type:     NodeTypeEnvironment,
		Name:     "env1",
		Path:     "project1/target1/env1",
		Children: []*Node{},
	}

	target := &Node{
		Type:     NodeTypeTarget,
		Name:     "target1",
		Path:     "project1/target1",
		Children: []*Node{env},
	}

	project := &Node{
		Type:     NodeTypeProject,
		Name:     "project1",
		Path:     "project1",
		Children: []*Node{target},
	}

	root := &Node{
		Type:     NodeTypeOrganization,
		Name:     "org",
		Path:     "/",
		Children: []*Node{project},
	}

	tests := []struct {
		name     string
		path     string
		wantNil  bool
		wantType NodeType
	}{
		{
			name:     "find root",
			path:     "/",
			wantType: NodeTypeOrganization,
		},
		{
			name:     "find project",
			path:     "project1",
			wantType: NodeTypeProject,
		},
		{
			name:     "find target",
			path:     "project1/target1",
			wantType: NodeTypeTarget,
		},
		{
			name:     "find environment",
			path:     "project1/target1/env1",
			wantType: NodeTypeEnvironment,
		},
		{
			name:    "find non-existent",
			path:    "project2",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := root.Find(tt.path)
			if tt.wantNil {
				if found != nil {
					t.Errorf("Find() = %v, want nil", found)
				}
			} else {
				if found == nil {
					t.Errorf("Find() = nil, want node")
				} else if found.Type != tt.wantType {
					t.Errorf("Find() type = %v, want %v", found.Type, tt.wantType)
				}
			}
		})
	}
}

func TestNode_CountDescendants(t *testing.T) {
	// Create a test hierarchy
	root := &Node{
		Type: NodeTypeOrganization,
		Name: "org",
		Children: []*Node{
			{
				Type: NodeTypeProject,
				Name: "project1",
				Children: []*Node{
					{
						Type: NodeTypeTarget,
						Name: "target1",
						Children: []*Node{
							{Type: NodeTypeEnvironment, Name: "env1", Children: []*Node{}},
							{Type: NodeTypeEnvironment, Name: "env2", Children: []*Node{}},
						},
					},
					{
						Type: NodeTypeTarget,
						Name: "target2",
						Children: []*Node{
							{Type: NodeTypeEnvironment, Name: "env3", Children: []*Node{}},
						},
					},
				},
			},
			{
				Type:     NodeTypeProject,
				Name:     "project2",
				Children: []*Node{},
			},
		},
	}

	tests := []struct {
		name     string
		node     *Node
		expected int
	}{
		{
			name:     "root count",
			node:     root,
			expected: 7, // 2 projects + 2 targets + 3 envs
		},
		{
			name:     "project1 count",
			node:     root.Children[0],
			expected: 5, // 2 targets + 3 envs
		},
		{
			name:     "target1 count",
			node:     root.Children[0].Children[0],
			expected: 2, // 2 envs
		},
		{
			name:     "env1 count",
			node:     root.Children[0].Children[0].Children[0],
			expected: 0, // no children
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.CountDescendants(); got != tt.expected {
				t.Errorf("CountDescendants() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNode_GetLeaves(t *testing.T) {
	// Create a test hierarchy
	env1 := &Node{Type: NodeTypeEnvironment, Name: "env1", Path: "p1/t1/env1", Children: []*Node{}}
	env2 := &Node{Type: NodeTypeEnvironment, Name: "env2", Path: "p1/t1/env2", Children: []*Node{}}
	env3 := &Node{Type: NodeTypeEnvironment, Name: "env3", Path: "p1/t2/env3", Children: []*Node{}}

	root := &Node{
		Type: NodeTypeOrganization,
		Name: "org",
		Children: []*Node{
			{
				Type: NodeTypeProject,
				Name: "project1",
				Children: []*Node{
					{
						Type:     NodeTypeTarget,
						Name:     "target1",
						Children: []*Node{env1, env2},
					},
					{
						Type:     NodeTypeTarget,
						Name:     "target2",
						Children: []*Node{env3},
					},
				},
			},
			{
				Type:     NodeTypeProject,
				Name:     "project2",
				Children: []*Node{}, // Empty project is a leaf
			},
		},
	}

	leaves := root.GetLeaves()

	// Should have 4 leaves: 3 environments + 1 empty project
	if len(leaves) != 4 {
		t.Errorf("GetLeaves() returned %d leaves, want 4", len(leaves))
	}

	// Check that all leaves have no children
	for _, leaf := range leaves {
		if len(leaf.Children) > 0 {
			t.Errorf("GetLeaves() returned node %s with %d children", leaf.Name, len(leaf.Children))
		}
	}
}

func TestNode_SortChildren(t *testing.T) {
	// Create unsorted hierarchy
	root := &Node{
		Type: NodeTypeOrganization,
		Name: "org",
		Children: []*Node{
			{Name: "zebra", Children: []*Node{
				{Name: "charlie", Children: []*Node{}},
				{Name: "alpha", Children: []*Node{}},
				{Name: "bravo", Children: []*Node{}},
			}},
			{Name: "alpha", Children: []*Node{}},
			{Name: "charlie", Children: []*Node{}},
			{Name: "bravo", Children: []*Node{}},
		},
	}

	root.SortChildren()

	// Check root level sorting
	expectedOrder := []string{"alpha", "bravo", "charlie", "zebra"}
	for i, expected := range expectedOrder {
		if root.Children[i].Name != expected {
			t.Errorf("SortChildren() root.Children[%d].Name = %v, want %v", i, root.Children[i].Name, expected)
		}
	}

	// Check nested sorting
	nestedOrder := []string{"alpha", "bravo", "charlie"}
	zebra := root.Children[3] // zebra is now last after sorting
	for i, expected := range nestedOrder {
		if zebra.Children[i].Name != expected {
			t.Errorf("SortChildren() zebra.Children[%d].Name = %v, want %v", i, zebra.Children[i].Name, expected)
		}
	}
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "simple path",
			path: "project/target/env",
			want: []string{"project", "target", "env"},
		},
		{
			name: "with leading slash",
			path: "/project/target",
			want: []string{"project", "target"},
		},
		{
			name: "with trailing slash",
			path: "project/target/",
			want: []string{"project", "target"},
		},
		{
			name: "with multiple slashes",
			path: "project//target///env",
			want: []string{"project", "target", "env"},
		},
		{
			name: "single component",
			path: "project",
			want: []string{"project"},
		},
		{
			name: "empty path",
			path: "",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePath(tt.path)
			if len(got) != len(tt.want) {
				t.Errorf("parsePath() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parsePath()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}