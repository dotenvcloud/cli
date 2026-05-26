package interactive

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dotenv/cli/internal/hierarchy"
	"github.com/dotenv/cli/internal/ui"
	dotenv "github.com/lostlink/dotenv-sdk-go"
)

// Action represents what to do with the selected resource
type Action string

const (
	ActionSelect        Action = "select"
	ActionCopy          Action = "copy"
	ActionPull          Action = "pull"
	ActionPullLevelOnly Action = "pull-level-only"
	ActionPush          Action = "push"
	ActionView          Action = "view"
	ActionBack          Action = "back"
	ActionExit          Action = "exit"
)

// ErrExit is returned when user wants to exit
var ErrExit = errors.New("exit")

// ErrBack is returned when user wants to go back
var ErrBack = errors.New("back")

// Explorer provides interactive navigation through the resource hierarchy
type Explorer struct {
	root    *hierarchy.Node
	builder *hierarchy.Builder
	client  *dotenv.Client
	history []*hierarchy.Node
	current *hierarchy.Node
}

// NewExplorer creates a new explorer instance
func NewExplorer(root *hierarchy.Node, builder *hierarchy.Builder, client *dotenv.Client) *Explorer {
	return &Explorer{
		root:    root,
		builder: builder,
		client:  client,
		history: make([]*hierarchy.Node, 0),
		current: root,
	}
}

// Run starts the interactive explorer and returns the selected path and action
func (e *Explorer) Run() (selectedPath string, action Action, err error) {
	for {
		// Build options for current level
		options := e.buildOptions()

		// Show prompt
		prompt := e.buildPrompt()

		selected, err := ui.Select(prompt, options)
		if err != nil {
			return "", "", err
		}

		// Handle selection
		action, err = e.handleSelection(selected)
		if err != nil {
			if err == ErrBack {
				// Go back to previous level
				if len(e.history) > 0 {
					e.current = e.history[len(e.history)-1]
					e.history = e.history[:len(e.history)-1]
				}
				continue
			}
			if err == ErrExit {
				return "", ActionExit, nil
			}
			return "", "", err
		}

		// If we have an action, return the current path
		if action != "" && action != ActionBack {
			return e.current.Path, action, nil
		}
	}
}

// buildOptions creates the list of options for the current node
func (e *Explorer) buildOptions() []string {
	options := []string{}

	// Add back option if not at root
	if len(e.history) > 0 {
		options = append(options, "← Back")
	}

	// Add children or actions based on node type
	if e.current.Type == hierarchy.NodeTypeEnvironment ||
		(len(e.current.Children) == 0 && e.current.Type != hierarchy.NodeTypeOrganization) {
		// This is a leaf node, show actions
		levelName := string(e.current.Type)

		options = append(options,
			"📋 Copy path to clipboard",
			fmt.Sprintf("⬇️  Pull %s secrets only", levelName),
			"⬇️  Pull all secrets (merged)",
			"⬆️  Push secrets",
			"👁  View details",
			"✓ Select and exit",
		)
	} else {
		// Show children for navigation
		for _, child := range e.current.Children {
			label := e.formatNodeOption(child)
			options = append(options, label)
		}

		// If this is a project or target, also allow actions including pull
		if e.current.Type == hierarchy.NodeTypeProject || e.current.Type == hierarchy.NodeTypeTarget {
			options = append(options, "") // Separator
			options = append(options, "── Actions for "+e.current.Name+" ──")

			// Customize pull option text based on level
			levelName := "project"
			if e.current.Type == hierarchy.NodeTypeTarget {
				levelName = "target"
			}

			options = append(options,
				"📋 Copy path to clipboard",
				fmt.Sprintf("⬇️  Pull %s secrets only", levelName),
				"⬇️  Pull all secrets (merged)",
				"⬆️  Push secrets",
				"✓ Select and exit",
			)
		}
	}

	// Always add exit option
	options = append(options, "") // Separator
	options = append(options, "✗ Exit")

	return options
}

// formatNodeOption formats a node for display in the selection list
func (e *Explorer) formatNodeOption(node *hierarchy.Node) string {
	prefix := "→ "
	label := node.Name

	// Add metadata information
	switch node.Type {
	case hierarchy.NodeTypeProject:
		if proj, ok := node.Metadata.(*dotenv.Project); ok {
			// Build label with encryption type indicator
			keyIndicator := ""
			if proj.EncryptionType == "client" {
				keyIndicator = " [client-key]"
			}

			if proj.TargetCount > 0 || proj.EnvironmentCount > 0 {
				label = fmt.Sprintf("%s%s (%d targets, %d environments)",
					node.Name, keyIndicator, proj.TargetCount, proj.EnvironmentCount)
			} else {
				label = node.Name + keyIndicator
			}
		}
		prefix = "📁 "

	case hierarchy.NodeTypeTarget:
		if target, ok := node.Metadata.(*dotenv.Target); ok && target.Description != "" {
			label = fmt.Sprintf("%s - %s", node.Name, target.Description)
		}
		prefix = "🎯 "

	case hierarchy.NodeTypeEnvironment:
		if env, ok := node.Metadata.(*dotenv.Environment); ok {
			status := env.Status
			if status == "" {
				status = "active"
			}
			label = fmt.Sprintf("%s [%s]", node.Name, status)
		}
		prefix = "🌿 "
	}

	return prefix + label
}

// buildPrompt creates the prompt text for the current level
func (e *Explorer) buildPrompt() string {
	path := e.buildPath()

	switch e.current.Type {
	case hierarchy.NodeTypeOrganization:
		return fmt.Sprintf("Select project in %s:", e.current.Name)
	case hierarchy.NodeTypeProject:
		return fmt.Sprintf("Select target in %s:", path)
	case hierarchy.NodeTypeTarget:
		return fmt.Sprintf("Select environment in %s:", path)
	case hierarchy.NodeTypeEnvironment:
		return fmt.Sprintf("What would you like to do with %s?", path)
	default:
		return fmt.Sprintf("Navigate %s:", path)
	}
}

// buildPath constructs the current path string
func (e *Explorer) buildPath() string {
	if e.current.Type == hierarchy.NodeTypeOrganization {
		return "/"
	}
	return e.current.Path
}

// handleSelection processes the user's selection
func (e *Explorer) handleSelection(selected string) (Action, error) {
	// Handle navigation options
	switch {
	case selected == "← Back":
		return ActionBack, ErrBack

	case selected == "✗ Exit":
		return ActionExit, ErrExit

	case strings.HasPrefix(selected, "📋 Copy"):
		return ActionCopy, nil

	case strings.Contains(selected, "secrets only"):
		return ActionPullLevelOnly, nil

	case strings.Contains(selected, "all secrets (merged)"):
		return ActionPull, nil

	case strings.HasPrefix(selected, "⬆️  Push"):
		return ActionPush, nil

	case strings.HasPrefix(selected, "👁  View"):
		return ActionView, nil

	case strings.HasPrefix(selected, "✓ Select"):
		return ActionSelect, nil

	case selected == "" || strings.HasPrefix(selected, "──"):
		// Separator or header, ignore
		return "", nil

	default:
		// This should be a child node
		return e.navigateToChild(selected)
	}
}

// navigateToChild moves to a child node based on the selection
func (e *Explorer) navigateToChild(selected string) (Action, error) {
	// Extract the node name from the formatted option
	// Remove emoji prefix and metadata suffix
	parts := strings.SplitN(selected, " ", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid selection: %s", selected)
	}

	nodeName := parts[1]
	// Remove metadata in parentheses
	if idx := strings.Index(nodeName, " ("); idx > 0 {
		nodeName = nodeName[:idx]
	}
	// Remove description after dash
	if idx := strings.Index(nodeName, " - "); idx > 0 {
		nodeName = nodeName[:idx]
	}
	// Remove status in brackets
	if idx := strings.Index(nodeName, " ["); idx > 0 {
		nodeName = nodeName[:idx]
	}

	// Find the child node
	for _, child := range e.current.Children {
		if child.Name == nodeName {
			// Load children if needed
			if len(child.Children) == 0 && child.Type != hierarchy.NodeTypeEnvironment {
				if err := e.loadChildren(child); err != nil {
					ui.PrintWarning("Failed to load children: %v", err)
				}
			}

			// Navigate to the child
			e.history = append(e.history, e.current)
			e.current = child
			return "", nil
		}
	}

	return "", fmt.Errorf("child not found: %s", nodeName)
}

// loadChildren loads children for a node using the hierarchy builder
func (e *Explorer) loadChildren(node *hierarchy.Node) error {
	ctx := context.Background()
	return e.builder.LoadChildren(ctx, node)
}

// GetCurrentPath returns the current node's path
func (e *Explorer) GetCurrentPath() string {
	return e.current.Path
}

// Reset resets the explorer to the root
func (e *Explorer) Reset() {
	e.current = e.root
	e.history = make([]*hierarchy.Node, 0)
}
