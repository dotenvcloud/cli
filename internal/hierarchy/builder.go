package hierarchy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	dotenv "github.com/dotenv/sdk-go"
)

// NodeType represents the type of node in the hierarchy
type NodeType string

const (
	NodeTypeOrganization NodeType = "organization"
	NodeTypeProject      NodeType = "project"
	NodeTypeTarget       NodeType = "target"
	NodeTypeEnvironment  NodeType = "environment"
)

// Node represents a node in the resource hierarchy
type Node struct {
	Type     NodeType
	Name     string
	Slug     string
	Path     string
	Children []*Node
	Metadata interface{} // Can hold *dotenv.Project, *dotenv.Target, or *dotenv.Environment
}

// Builder builds resource hierarchies from the DotEnv API
type Builder struct {
	client *dotenv.Client
	cache  *hierarchyCache
}

// hierarchyCache provides simple caching for API results
type hierarchyCache struct {
	mu       sync.RWMutex
	projects map[string][]*dotenv.Project
	targets  map[string][]*dotenv.Target
	envs     map[string][]*dotenv.Environment
}

// NewBuilder creates a new hierarchy builder
func NewBuilder(client *dotenv.Client) *Builder {
	return &Builder{
		client: client,
		cache: &hierarchyCache{
			projects: make(map[string][]*dotenv.Project),
			targets:  make(map[string][]*dotenv.Target),
			envs:     make(map[string][]*dotenv.Environment),
		},
	}
}

// Build constructs the full hierarchy for an organization
func (b *Builder) Build(ctx context.Context, orgSlug string) (*Node, error) {
	if orgSlug == "" {
		return nil, fmt.Errorf("organization identifier cannot be empty")
	}

	root := &Node{
		Type:     NodeTypeOrganization,
		Name:     orgSlug,
		Slug:     orgSlug,
		Path:     "/",
		Children: []*Node{},
	}

	// Fetch all projects
	projects, err := b.fetchProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	// Build project nodes
	for _, project := range projects {
		projectNode := &Node{
			Type:     NodeTypeProject,
			Name:     project.Name,
			Slug:     project.Slug,
			Path:     project.Slug,
			Metadata: project,
			Children: []*Node{},
		}

		// Only fetch targets if there are any
		if project.TargetCount > 0 {
			if err := b.loadTargetsForProject(ctx, projectNode, project.Slug); err != nil {
				// Log warning but continue with other projects
				continue
			}
		}

		root.Children = append(root.Children, projectNode)
	}

	return root, nil
}

// BuildProject builds hierarchy for a specific project
func (b *Builder) BuildProject(ctx context.Context, projectSlug string) (*Node, error) {
	// First, get the project details
	project, _, err := b.client.Projects.Get(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	projectNode := &Node{
		Type:     NodeTypeProject,
		Name:     project.Name,
		Slug:     project.Slug,
		Path:     project.Slug,
		Metadata: project,
		Children: []*Node{},
	}

	// Load targets if any
	if project.TargetCount > 0 {
		if err := b.loadTargetsForProject(ctx, projectNode, project.Slug); err != nil {
			return nil, err
		}
	}

	return projectNode, nil
}

// BuildTarget builds hierarchy for a specific target
func (b *Builder) BuildTarget(ctx context.Context, projectSlug, targetSlug string) (*Node, error) {
	// Get target details
	target, _, err := b.client.Targets.Get(ctx, projectSlug, targetSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}

	targetNode := &Node{
		Type:     NodeTypeTarget,
		Name:     target.Name,
		Slug:     target.Slug,
		Path:     fmt.Sprintf("%s/%s", projectSlug, target.Slug),
		Metadata: target,
		Children: []*Node{},
	}

	// Load environments
	if err := b.loadEnvironmentsForTarget(ctx, targetNode, projectSlug, target.Slug); err != nil {
		return nil, err
	}

	return targetNode, nil
}

// LoadChildren loads children for a given node
func (b *Builder) LoadChildren(ctx context.Context, node *Node) error {
	switch node.Type {
	case NodeTypeProject:
		project, ok := node.Metadata.(*dotenv.Project)
		if !ok {
			return fmt.Errorf("invalid project metadata")
		}
		return b.loadTargetsForProject(ctx, node, project.Slug)

	case NodeTypeTarget:
		// Parse project slug from path
		path := node.Path
		parts := parsePath(path)
		if len(parts) < 2 {
			return fmt.Errorf("invalid target path: %s", path)
		}
		return b.loadEnvironmentsForTarget(ctx, node, parts[0], parts[1])

	default:
		// Organizations and environments don't have children to load dynamically
		return nil
	}
}

// Private helper methods

func (b *Builder) fetchProjects(ctx context.Context) ([]*dotenv.Project, error) {
	// Note: organization context is set in the client, so we don't need orgSlug parameter

	// For now, we'll use a fixed cache key since the client has the org context
	cacheKey := "current"

	// Check cache first
	b.cache.mu.RLock()
	if cached, ok := b.cache.projects[cacheKey]; ok {
		b.cache.mu.RUnlock()
		return cached, nil
	}
	b.cache.mu.RUnlock()

	// Fetch from API
	projects, _, err := b.client.Projects.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Update cache
	b.cache.mu.Lock()
	b.cache.projects[cacheKey] = projects
	b.cache.mu.Unlock()

	return projects, nil
}

func (b *Builder) loadTargetsForProject(ctx context.Context, projectNode *Node, projectSlug string) error {
	// Check cache
	cacheKey := projectSlug
	b.cache.mu.RLock()
	if cached, ok := b.cache.targets[cacheKey]; ok {
		b.cache.mu.RUnlock()
		// Build nodes from cache
		for _, target := range cached {
			targetNode := &Node{
				Type:     NodeTypeTarget,
				Name:     target.Name,
				Slug:     target.Slug,
				Path:     fmt.Sprintf("%s/%s", projectSlug, target.Slug),
				Metadata: target,
				Children: []*Node{},
			}

			// Also load environments for this target if it has any
			if target.EnvironmentCount > 0 {
				_ = b.loadEnvironmentsForTarget(ctx, targetNode, projectSlug, target.Slug)
			}

			projectNode.Children = append(projectNode.Children, targetNode)
		}
		return nil
	}
	b.cache.mu.RUnlock()

	// Fetch from API
	targets, _, err := b.client.Targets.List(ctx, projectSlug, nil)
	if err != nil {
		return fmt.Errorf("failed to list targets: %w", err)
	}

	// Update cache
	b.cache.mu.Lock()
	b.cache.targets[cacheKey] = targets
	b.cache.mu.Unlock()

	// Build nodes
	for _, target := range targets {
		targetNode := &Node{
			Type:     NodeTypeTarget,
			Name:     target.Name,
			Slug:     target.Slug,
			Path:     fmt.Sprintf("%s/%s", projectSlug, target.Slug),
			Metadata: target,
			Children: []*Node{},
		}

		// Also load environments for this target if it has any
		if target.EnvironmentCount > 0 {
			_ = b.loadEnvironmentsForTarget(ctx, targetNode, projectSlug, target.Slug)
		}

		projectNode.Children = append(projectNode.Children, targetNode)
	}

	return nil
}

func (b *Builder) loadEnvironmentsForTarget(ctx context.Context, targetNode *Node, projectSlug, targetSlug string) error {
	// Check cache
	cacheKey := fmt.Sprintf("%s/%s", projectSlug, targetSlug)
	b.cache.mu.RLock()
	if cached, ok := b.cache.envs[cacheKey]; ok {
		b.cache.mu.RUnlock()
		// Build nodes from cache
		for _, env := range cached {
			envNode := &Node{
				Type:     NodeTypeEnvironment,
				Name:     env.Name,
				Slug:     env.Slug,
				Path:     fmt.Sprintf("%s/%s/%s", projectSlug, targetSlug, env.Slug),
				Metadata: env,
				Children: []*Node{}, // Environments have no children
			}
			targetNode.Children = append(targetNode.Children, envNode)
		}
		return nil
	}
	b.cache.mu.RUnlock()

	// Fetch from API
	envs, _, err := b.client.Environments.List(ctx, projectSlug, targetSlug, nil)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	// Update cache
	b.cache.mu.Lock()
	b.cache.envs[cacheKey] = envs
	b.cache.mu.Unlock()

	// Build nodes
	for _, env := range envs {
		envNode := &Node{
			Type:     NodeTypeEnvironment,
			Name:     env.Name,
			Slug:     env.Slug,
			Path:     fmt.Sprintf("%s/%s/%s", projectSlug, targetSlug, env.Slug),
			Metadata: env,
			Children: []*Node{},
		}
		targetNode.Children = append(targetNode.Children, envNode)
	}

	return nil
}

// ClearCache clears the internal cache
func (b *Builder) ClearCache() {
	b.cache.mu.Lock()
	defer b.cache.mu.Unlock()

	b.cache.projects = make(map[string][]*dotenv.Project)
	b.cache.targets = make(map[string][]*dotenv.Target)
	b.cache.envs = make(map[string][]*dotenv.Environment)
}

// Helper functions

func parsePath(path string) []string {
	var parts []string
	for _, part := range strings.Split(path, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// Walk traverses the hierarchy tree, calling the visitor function for each node
func (n *Node) Walk(visitor func(*Node) error) error {
	if err := visitor(n); err != nil {
		return err
	}

	for _, child := range n.Children {
		if err := child.Walk(visitor); err != nil {
			return err
		}
	}

	return nil
}

// Find searches for a node by path
func (n *Node) Find(path string) *Node {
	if n.Path == path {
		return n
	}

	for _, child := range n.Children {
		if found := child.Find(path); found != nil {
			return found
		}
	}

	return nil
}

// CountDescendants returns the total number of descendants
func (n *Node) CountDescendants() int {
	count := len(n.Children)
	for _, child := range n.Children {
		count += child.CountDescendants()
	}
	return count
}

// GetLeaves returns all leaf nodes (nodes without children)
func (n *Node) GetLeaves() []*Node {
	if len(n.Children) == 0 {
		return []*Node{n}
	}

	var leaves []*Node
	for _, child := range n.Children {
		leaves = append(leaves, child.GetLeaves()...)
	}
	return leaves
}

// SortChildren sorts children alphabetically by name
func (n *Node) SortChildren() {
	if n == nil || n.Children == nil {
		return
	}
	sort.Slice(n.Children, func(i, j int) bool {
		return n.Children[i].Name < n.Children[j].Name
	})

	// Recursively sort children
	for _, child := range n.Children {
		child.SortChildren()
	}
}
