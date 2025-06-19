package formats_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dotenv/cli/internal/formats"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		expected formats.Format
	}{
		// Filename detection
		{"env file by extension", ".env", "", formats.FormatEnv},
		{"env file by name", "production.env", "", formats.FormatEnv},
		{"json file", "config.json", "", formats.FormatJSON},
		{"yaml file", "config.yaml", "", formats.FormatYAML},
		{"yml file", "config.yml", "", formats.FormatYAML},

		// Content detection
		{"env content", "", "KEY=value\nANOTHER=test", formats.FormatEnv},
		{"json content", "", `{"key": "value"}`, formats.FormatJSON},
		{"yaml content", "", "key: value\nanother: test", formats.FormatYAML},
		{"shell content", "", "export KEY=value\nexport ANOTHER=test", formats.FormatShell},

		// Edge cases
		{"empty content", "", "", formats.FormatEnv},
		{"unknown extension", "file.txt", "KEY=value", formats.FormatEnv},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := formats.NewDetector()
			result := detector.Detect(tt.filename, []byte(tt.content))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatter_Format(t *testing.T) {
	secrets := map[string]string{
		"DATABASE_URL": "postgres://localhost/test",
		"API_KEY":      "secret-key",
		"DEBUG":        "true",
		"PORT":         "3000",
	}

	tests := []struct {
		name     string
		format   formats.Format
		expected []string
	}{
		{
			name:   "env format",
			format: formats.FormatEnv,
			expected: []string{
				"API_KEY=secret-key",
				"DATABASE_URL=postgres://localhost/test",
				"DEBUG=true",
				"PORT=3000",
			},
		},
		{
			name:   "json format",
			format: formats.FormatJSON,
			expected: []string{
				`{`,
				`"API_KEY": "secret-key"`,
				`"DATABASE_URL": "postgres://localhost/test"`,
				`"DEBUG": "true"`,
				`"PORT": "3000"`,
				`}`,
			},
		},
		{
			name:   "yaml format",
			format: formats.FormatYAML,
			expected: []string{
				"API_KEY: secret-key",
				"DATABASE_URL: postgres://localhost/test",
				"DEBUG: \"true\"",
				"PORT: \"3000\"",
			},
		},
		{
			name:   "shell format",
			format: formats.FormatShell,
			expected: []string{
				`export API_KEY="secret-key"`,
				`export DATABASE_URL="postgres://localhost/test"`,
				`export DEBUG="true"`,
				`export PORT="3000"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := formats.NewFormatter()
			var buf bytes.Buffer

			err := formatter.Format(&buf, secrets, tt.format)
			require.NoError(t, err)

			result := buf.String()
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestFormatter_Parse(t *testing.T) {
	tests := []struct {
		name     string
		format   formats.Format
		content  string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:   "env format",
			format: formats.FormatEnv,
			content: `# Comment
KEY1=value1
KEY2="quoted value"
KEY3='single quoted'

# Another comment
KEY4=with spaces  
`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "quoted value",
				"KEY3": "single quoted",
				"KEY4": "with spaces",
			},
		},
		{
			name:   "json format",
			format: formats.FormatJSON,
			content: `{
  "KEY1": "value1",
  "KEY2": "value2",
  "KEY3": 123,
  "KEY4": true
}`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "123",
				"KEY4": "true",
			},
		},
		{
			name:   "yaml format",
			format: formats.FormatYAML,
			content: `# YAML config
KEY1: value1
KEY2: "quoted value"
KEY3: 123
KEY4: true
nested:
  key: value`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "quoted value",
				"KEY3": "123",
				"KEY4": "true",
			},
		},
		{
			name:   "shell format",
			format: formats.FormatShell,
			content: `#!/bin/bash
# Shell exports
export KEY1="value1"
export KEY2='value2'
export KEY3=value3`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
		},
		{
			name:    "invalid json",
			format:  formats.FormatJSON,
			content: `{invalid json`,
			wantErr: true,
		},
		{
			name:   "invalid yaml",
			format: formats.FormatYAML,
			content: `invalid:
  - yaml
  content`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := formats.NewFormatter()
			reader := strings.NewReader(tt.content)

			result, err := formatter.Parse(reader, tt.format)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatter_EdgeCases(t *testing.T) {
	formatter := formats.NewFormatter()

	t.Run("empty secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.Format(&buf, map[string]string{}, formats.FormatEnv)
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})

	t.Run("special characters in values", func(t *testing.T) {
		secrets := map[string]string{
			"SPECIAL": `value with "quotes" and 'apostrophes'`,
			"NEWLINE": "value\nwith\nnewlines",
			"EQUALS":  "value=with=equals",
		}

		var buf bytes.Buffer
		err := formatter.Format(&buf, secrets, formats.FormatEnv)
		require.NoError(t, err)

		// Parse it back
		parsed, err := formatter.Parse(&buf, formats.FormatEnv)
		require.NoError(t, err)

		// Should handle special characters correctly
		assert.Equal(t, secrets["SPECIAL"], parsed["SPECIAL"])
	})

	t.Run("unicode values", func(t *testing.T) {
		secrets := map[string]string{
			"UNICODE": "Hello 世界 🌍",
			"EMOJI":   "🚀🎉✨",
		}

		for _, format := range []formats.Format{formats.FormatEnv, formats.FormatJSON, formats.FormatYAML} {
			t.Run(string(format), func(t *testing.T) {
				var buf bytes.Buffer
				err := formatter.Format(&buf, secrets, format)
				require.NoError(t, err)

				parsed, err := formatter.Parse(&buf, format)
				require.NoError(t, err)

				assert.Equal(t, secrets, parsed)
			})
		}
	})
}

func TestFormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		format  formats.Format
		content string
		valid   bool
	}{
		{"valid env", formats.FormatEnv, "KEY=value", true},
		{"invalid env", formats.FormatEnv, "invalid content without equals", false},
		{"valid json", formats.FormatJSON, `{"key": "value"}`, true},
		{"invalid json", formats.FormatJSON, `{invalid}`, false},
		{"valid yaml", formats.FormatYAML, "key: value", true},
		{"invalid yaml", formats.FormatYAML, "@@invalid@@", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := formats.NewFormatter()
			err := formatter.Validate([]byte(tt.content), tt.format)

			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func BenchmarkFormatter_Format(b *testing.B) {
	secrets := make(map[string]string)
	for i := 0; i < 100; i++ {
		secrets[fmt.Sprintf("KEY_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	formatter := formats.NewFormatter()

	for _, format := range []formats.Format{formats.FormatEnv, formats.FormatJSON, formats.FormatYAML} {
		b.Run(string(format), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				if err := formatter.Format(&buf, secrets, format); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFormatter_Parse(b *testing.B) {
	// Create test content
	var envContent strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&envContent, "KEY_%d=value_%d\n", i, i)
	}

	formatter := formats.NewFormatter()
	content := envContent.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(content)
		if _, err := formatter.Parse(reader, formats.FormatEnv); err != nil {
			b.Fatal(err)
		}
	}
}
