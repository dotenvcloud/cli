package paths

import (
	"fmt"
	"strings"
)

// ResourcePath represents a parsed resource path with its components
type ResourcePath struct {
	Project     string
	Target      string
	Environment string
}

// ParseResourcePath parses a path string like "project/target/environment" into its components
// Returns an error if the path format is invalid
func ParseResourcePath(path string) (*ResourcePath, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}

	// Normalize path by trimming spaces and slashes
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "/")

	if path == "" {
		return nil, fmt.Errorf("empty path after normalization")
	}

	parts := strings.Split(path, "/")
	rp := &ResourcePath{}

	switch len(parts) {
	case 1:
		rp.Project = parts[0]
	case 2:
		rp.Project = parts[0]
		rp.Target = parts[1]
	case 3:
		rp.Project = parts[0]
		rp.Target = parts[1]
		rp.Environment = parts[2]
	default:
		return nil, fmt.Errorf("invalid path format: %s (expected project[/target[/environment]])", path)
	}

	// Validate that none of the parts are empty
	if rp.Project == "" {
		return nil, fmt.Errorf("project cannot be empty in path: %s", path)
	}
	if len(parts) >= 2 && rp.Target == "" {
		return nil, fmt.Errorf("target cannot be empty in path: %s", path)
	}
	if len(parts) >= 3 && rp.Environment == "" {
		return nil, fmt.Errorf("environment cannot be empty in path: %s", path)
	}

	return rp, nil
}

// FormatResourcePath formats components into a path string
// Empty components are ignored
func FormatResourcePath(project, target, environment string) string {
	var parts []string

	if project != "" {
		parts = append(parts, project)
	}
	if target != "" {
		parts = append(parts, target)
	}
	if environment != "" {
		parts = append(parts, environment)
	}

	return strings.Join(parts, "/")
}

// ValidateResourcePath checks if a path string is valid
func ValidateResourcePath(path string) error {
	_, err := ParseResourcePath(path)
	return err
}

// Level returns the depth level of the path
// 0 = empty, 1 = project only, 2 = project/target, 3 = project/target/environment
func (rp *ResourcePath) Level() int {
	if rp == nil {
		return 0
	}

	if rp.Environment != "" {
		return 3
	}
	if rp.Target != "" {
		return 2
	}
	if rp.Project != "" {
		return 1
	}
	return 0
}

// String returns the formatted path string
func (rp *ResourcePath) String() string {
	if rp == nil {
		return ""
	}
	return FormatResourcePath(rp.Project, rp.Target, rp.Environment)
}

// IsComplete returns true if all components are present (project/target/environment)
func (rp *ResourcePath) IsComplete() bool {
	return rp != nil && rp.Project != "" && rp.Target != "" && rp.Environment != ""
}

// Parent returns the parent path (one level up)
// Returns nil if already at root level
func (rp *ResourcePath) Parent() *ResourcePath {
	if rp == nil || rp.Level() == 0 {
		return nil
	}

	switch rp.Level() {
	case 1:
		return nil // project has no parent
	case 2:
		return &ResourcePath{Project: rp.Project}
	case 3:
		return &ResourcePath{Project: rp.Project, Target: rp.Target}
	default:
		return nil
	}
}

// Equals compares two resource paths for equality
func (rp *ResourcePath) Equals(other *ResourcePath) bool {
	if rp == nil || other == nil {
		return rp == other
	}

	return rp.Project == other.Project &&
		rp.Target == other.Target &&
		rp.Environment == other.Environment
}

// Contains checks if this path contains the other path
// e.g., "project/target" contains "project"
func (rp *ResourcePath) Contains(other *ResourcePath) bool {
	if rp == nil || other == nil || rp.Level() < other.Level() {
		return false
	}

	// Check project match
	if other.Project != "" && rp.Project != other.Project {
		return false
	}

	// Check target match if specified
	if other.Target != "" && rp.Target != other.Target {
		return false
	}

	// Check environment match if specified
	if other.Environment != "" && rp.Environment != other.Environment {
		return false
	}

	return true
}

// MatchesPattern checks if the path matches a pattern using case-insensitive substring matching
func (rp *ResourcePath) MatchesPattern(pattern string) bool {
	if rp == nil || pattern == "" {
		return false
	}

	// Convert to lowercase for case-insensitive matching
	pattern = strings.ToLower(pattern)
	pathStr := strings.ToLower(rp.String())

	// Check if pattern appears in the full path
	if strings.Contains(pathStr, pattern) {
		return true
	}

	// Also check individual components
	if strings.Contains(strings.ToLower(rp.Project), pattern) {
		return true
	}
	if rp.Target != "" && strings.Contains(strings.ToLower(rp.Target), pattern) {
		return true
	}
	if rp.Environment != "" && strings.Contains(strings.ToLower(rp.Environment), pattern) {
		return true
	}

	return false
}
