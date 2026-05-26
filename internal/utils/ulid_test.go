package utils

import (
	"testing"

	dotenv "github.com/lostlink/dotenv-sdk-go"
	"github.com/stretchr/testify/assert"
)

func TestValidateULID(t *testing.T) {
	tests := []struct {
		name      string
		ulid      string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid ULID uppercase",
			ulid:      "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
			wantError: false,
		},
		{
			name:      "valid ULID lowercase",
			ulid:      "01hkt3m4y8z5v6r9s7x2f0q3we",
			wantError: false,
		},
		{
			name:      "valid ULID mixed case",
			ulid:      "01HkT3M4y8Z5v6R9s7X2f0Q3wE",
			wantError: false,
		},
		{
			name:      "empty ULID",
			ulid:      "",
			wantError: true,
			errorMsg:  "ULID cannot be empty",
		},
		{
			name:      "ULID too short",
			ulid:      "01HKT3M4Y8Z5V6R9",
			wantError: true,
			errorMsg:  "ULID must be exactly 26 characters long",
		},
		{
			name:      "ULID too long",
			ulid:      "01HKT3M4Y8Z5V6R9S7X2F0Q3WE1",
			wantError: true,
			errorMsg:  "ULID must be exactly 26 characters long",
		},
		{
			name:      "ULID with invalid characters",
			ulid:      "01HKT3M4Y8Z5V6R9S7X2F0Q3W!",
			wantError: true,
			errorMsg:  "ULID contains invalid characters",
		},
		{
			name:      "ULID with spaces",
			ulid:      "01HKT 3M4Y8Z5V6R9S7X2F0Q3WE",
			wantError: true,
			errorMsg:  "ULID must be exactly 26 characters long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateULID(tt.ulid)
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsValidULID(t *testing.T) {
	tests := []struct {
		name     string
		ulid     string
		expected bool
	}{
		{
			name:     "valid ULID uppercase",
			ulid:     "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
			expected: true,
		},
		{
			name:     "valid ULID lowercase",
			ulid:     "01hkt3m4y8z5v6r9s7x2f0q3we",
			expected: true,
		},
		{
			name:     "empty string",
			ulid:     "",
			expected: false,
		},
		{
			name:     "too short",
			ulid:     "01HKT3M4Y8Z5V6R9",
			expected: false,
		},
		{
			name:     "too long",
			ulid:     "01HKT3M4Y8Z5V6R9S7X2F0Q3WE1",
			expected: false,
		},
		{
			name:     "invalid characters",
			ulid:     "01HKT3M4Y8Z5V6R9S7X2F0Q3W!",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidULID(tt.ulid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetULIDFromOrg(t *testing.T) {
	tests := []struct {
		name     string
		org      *dotenv.Organization
		expected string
	}{
		{
			name:     "nil organization",
			org:      nil,
			expected: "",
		},
		{
			name: "organization with ULID field",
			org: &dotenv.Organization{
				ID:   "some-id",
				ULID: "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
				Name: "Test Org",
			},
			expected: "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
		},
		{
			name: "organization without ULID field",
			org: &dotenv.Organization{
				ID:   "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
				ULID: "",
				Name: "Test Org",
			},
			expected: "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
		},
		{
			name: "organization with empty ULID and ID",
			org: &dotenv.Organization{
				ID:   "",
				ULID: "",
				Name: "Test Org",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetULIDFromOrg(tt.org)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractULIDFromIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		expected   string
	}{
		{
			name:       "plain ULID",
			identifier: "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
			expected:   "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
		},
		{
			name:       "ULID in parentheses",
			identifier: "Test Org (01HKT3M4Y8Z5V6R9S7X2F0Q3WE)",
			expected:   "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
		},
		{
			name:       "ULID with text before",
			identifier: "Organization: 01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
			expected:   "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
		},
		{
			name:       "multiple ULIDs (takes first)",
			identifier: "01HKT3M4Y8Z5V6R9S7X2F0Q3WE and 01HKT3M4Y8Z5V6R9S7X2F0Q3WF",
			expected:   "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
		},
		{
			name:       "no ULID present",
			identifier: "Test Organization",
			expected:   "",
		},
		{
			name:       "empty string",
			identifier: "",
			expected:   "",
		},
		{
			name:       "lowercase ULID",
			identifier: "org (01hkt3m4y8z5v6r9s7x2f0q3we)",
			expected:   "01hkt3m4y8z5v6r9s7x2f0q3we",
		},
		{
			name:       "ULID in URL format",
			identifier: "/organizations/01HKT3M4Y8Z5V6R9S7X2F0Q3WE/settings",
			expected:   "01HKT3M4Y8Z5V6R9S7X2F0Q3WE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractULIDFromIdentifier(tt.identifier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateULID(t *testing.T) {
	// Test that GenerateULID creates valid ULIDs
	for i := 0; i < 10; i++ {
		ulid := GenerateULID()
		assert.Equal(t, 26, len(ulid))
		assert.True(t, IsValidULID(ulid))
		assert.NoError(t, ValidateULID(ulid))
	}

	// Test that ULIDs are unique
	ulids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ulid := GenerateULID()
		assert.False(t, ulids[ulid], "ULID should be unique")
		ulids[ulid] = true
	}

	// Test that ULIDs are lexicographically sortable by time
	ulid1 := GenerateULID()
	// Small delay to ensure different timestamp
	ulid2 := GenerateULID()
	assert.True(t, ulid1 < ulid2, "ULIDs should be lexicographically sortable by time")
}
