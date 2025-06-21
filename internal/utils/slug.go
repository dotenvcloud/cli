package utils

import (
	"regexp"
	"strings"
)

// Slugify converts a string into a URL-friendly slug
// Rules:
// - Convert to lowercase
// - Replace spaces with hyphens
// - Remove special characters except: a-z, 0-9, -, _, .
// - Collapse multiple hyphens
// - Trim hyphens from start/end
func Slugify(input string) string {
	// Convert to lowercase
	slug := strings.ToLower(input)
	
	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	
	// Remove special characters except allowed ones
	// Keep: a-z, 0-9, -, .
	reg := regexp.MustCompile(`[^a-z0-9\-.]`)
	slug = reg.ReplaceAllString(slug, "")
	
	// Collapse multiple hyphens into one
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	
	// Trim hyphens and dots from start/end
	slug = strings.Trim(slug, "-.")
	
	// Ensure slug is not empty
	if slug == "" {
		slug = "default"
	}
	
	return slug
}