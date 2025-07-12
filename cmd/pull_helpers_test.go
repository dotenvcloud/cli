package cmd

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestDetermineTargetLevel(t *testing.T) {
	tests := []struct {
		name       string
		hierarchy  struct {
			Project     string  `json:"project"`
			Target      *string `json:"target"`
			Environment *string `json:"environment"`
		}
		expected string
	}{
		{
			name: "environment level",
			hierarchy: struct {
				Project     string  `json:"project"`
				Target      *string `json:"target"`
				Environment *string `json:"environment"`
			}{
				Project:     "test-project",
				Target:      strPtr("production"),
				Environment: strPtr("api"),
			},
			expected: "environment",
		},
		{
			name: "target level",
			hierarchy: struct {
				Project     string  `json:"project"`
				Target      *string `json:"target"`
				Environment *string `json:"environment"`
			}{
				Project:     "test-project",
				Target:      strPtr("production"),
				Environment: nil,
			},
			expected: "target",
		},
		{
			name: "project level",
			hierarchy: struct {
				Project     string  `json:"project"`
				Target      *string `json:"target"`
				Environment *string `json:"environment"`
			}{
				Project:     "test-project",
				Target:      nil,
				Environment: nil,
			},
			expected: "project",
		},
		{
			name: "empty environment string",
			hierarchy: struct {
				Project     string  `json:"project"`
				Target      *string `json:"target"`
				Environment *string `json:"environment"`
			}{
				Project:     "test-project",
				Target:      strPtr("production"),
				Environment: strPtr(""),
			},
			expected: "target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineTargetLevel(tt.hierarchy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSecretContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		format    string
		expected  map[string]string
		wantError bool
	}{
		{
			name: "env format",
			content: `DATABASE_URL=postgres://localhost/test
API_KEY=secret123
# Comment line
EMPTY=`,
			format: "env",
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"API_KEY":      "secret123",
				"EMPTY":        "",
			},
		},
		{
			name: "json format",
			content: `{
				"DATABASE_URL": "postgres://localhost/test",
				"API_KEY": "secret123",
				"NESTED_JSON": "{\"key\":\"value\"}"
			}`,
			format: "json",
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"API_KEY":      "secret123",
				"NESTED_JSON":  "{\"key\":\"value\"}",
			},
		},
		{
			name:      "invalid json",
			content:   `{"broken": json}`,
			format:    "json",
			wantError: true,
		},
		{
			name:      "unsupported format",
			content:   `some content`,
			format:    "xml",
			wantError: true,
		},
		{
			name:    "empty format defaults to env",
			content: `KEY=value`,
			format:  "",
			expected: map[string]string{
				"KEY": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSecretContent(tt.content, tt.format)
			
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatSecrets(t *testing.T) {
	secrets := map[string]string{
		"DATABASE_URL": "postgres://localhost/test",
		"API_KEY":      "secret123",
		"QUOTED":       `value with "quotes"`,
	}

	tests := []struct {
		name     string
		format   string
		secrets  map[string]string
		contains []string
		wantErr  bool
	}{
		{
			name:    "env format",
			format:  "env",
			secrets: secrets,
			contains: []string{
				"DATABASE_URL=postgres://localhost/test",
				"API_KEY=secret123",
				`QUOTED="value with \"quotes\""`,
			},
		},
		{
			name:    "json format",
			format:  "json",
			secrets: secrets,
			contains: []string{
				`"DATABASE_URL": "postgres://localhost/test"`,
				`"API_KEY": "secret123"`,
				`"QUOTED": "value with \"quotes\""`,
			},
		},
		{
			name:    "yaml format",
			format:  "yaml",
			secrets: secrets,
			contains: []string{
				"DATABASE_URL: postgres://localhost/test",
				"API_KEY: secret123",
			},
		},
		{
			name:    "shell format",
			format:  "shell",
			secrets: secrets,
			contains: []string{
				`export DATABASE_URL="postgres://localhost/test"`,
				`export API_KEY="secret123"`,
				`export QUOTED="value with \"quotes\""`,
			},
		},
		{
			name:    "dockerfile format",
			format:  "dockerfile",
			secrets: secrets,
			contains: []string{
				`ENV DATABASE_URL="postgres://localhost/test"`,
				`ENV API_KEY="secret123"`,
				`ENV QUOTED="value with \\"quotes\\""`,
			},
		},
		{
			name:    "unsupported format",
			format:  "xml",
			secrets: secrets,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatSecrets(tt.secrets, tt.format)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, expected := range tt.contains {
					if !assert.Contains(t, result, expected) {
						t.Logf("Result:\n%s", result)
					}
				}
			}
		})
	}
}

// Helper function
func strPtr(s string) *string {
	return &s
}