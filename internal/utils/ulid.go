package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oklog/ulid/v2"
	dotenv "github.com/dotenv/sdk-go"
)

const (
	// ULIDLength is the standard length of a ULID
	ULIDLength = 26
	
	// ValidULIDChars contains all valid characters for a ULID (Crockford's base32)
	// Excludes I, L, O, U to avoid confusion
	ValidULIDChars = "0123456789ABCDEFGHJKMNPQRSTVWXYZabcdefghjkmnpqrstvwxyz"
)

// GetULIDFromOrg extracts the ULID from an organization, checking both ULID and ID fields
// This handles the API inconsistency where ULID is sometimes returned in the ID field
func GetULIDFromOrg(org *dotenv.Organization) string {
	if org == nil {
		return ""
	}
	
	// Use ULID if available
	if org.ULID != "" {
		return org.ULID
	}
	
	// Fall back to ID field (API sometimes returns ULID in ID field)
	return org.ID
}

// IsValidULID checks if a string is a valid ULID format
func IsValidULID(s string) bool {
	// ULIDs are exactly 26 characters
	if len(s) != ULIDLength {
		return false
	}

	// Check if all characters are valid base32 (Crockford's encoding)
	// Accept both uppercase and lowercase for flexibility
	for _, c := range s {
		if !strings.ContainsRune(ValidULIDChars, c) {
			return false
		}
	}

	return true
}

// NormalizeULID converts a ULID to uppercase for consistent comparison
func NormalizeULID(ulid string) string {
	return strings.ToUpper(ulid)
}

// ValidateULID validates a ULID and returns an error if invalid
func ValidateULID(ulid string) error {
	if ulid == "" {
		return fmt.Errorf("ULID cannot be empty")
	}
	
	if len(ulid) != ULIDLength {
		return fmt.Errorf("ULID must be exactly 26 characters long")
	}
	
	if !IsValidULID(ulid) {
		return fmt.Errorf("ULID contains invalid characters")
	}
	
	return nil
}

// ExtractULIDFromIdentifier extracts a ULID from a string that may contain other text
// For example: "My Org (01HKT3M4Y8Z5V6R9S7X2F0Q3WE)" returns "01HKT3M4Y8Z5V6R9S7X2F0Q3WE"
func ExtractULIDFromIdentifier(identifier string) string {
	// Create a regex pattern that matches ULID format (26 chars of valid base32)
	// We use word boundaries to ensure we get complete ULIDs
	pattern := fmt.Sprintf(`\b[%s]{%d}\b`, strings.ReplaceAll(ValidULIDChars, "]", "\\]"), ULIDLength)
	re := regexp.MustCompile(pattern)
	
	match := re.FindString(identifier)
	if match != "" && IsValidULID(match) {
		return match
	}
	
	return ""
}

// GenerateULID generates a new ULID
func GenerateULID() string {
	return ulid.Make().String()
}