package env

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dotenv/cli/internal/formats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen // table-driven test covers many .env parse variants; splitting hurts readability
func TestParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "simple values",
			input: `KEY1=value1
KEY2=value2
KEY3=value3`,
			want: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
		},
		{
			name: "quoted values",
			input: `SINGLE='single quoted'
DOUBLE="double quoted"
MIXED="contains 'single' quotes"`,
			want: map[string]string{
				"SINGLE": "single quoted",
				"DOUBLE": "double quoted",
				"MIXED":  "contains 'single' quotes",
			},
		},
		{
			name: "multiline values",
			input: `MULTILINE="line1
line2
line3"`,
			want: map[string]string{
				"MULTILINE": "line1\nline2\nline3",
			},
		},
		{
			name: "escape sequences",
			input: `ESCAPED="value with \"quotes\" and \n newlines"
TAB="value\twith\ttabs"`,
			want: map[string]string{
				"ESCAPED": "value with \"quotes\" and \n newlines",
				"TAB":     "value\twith\ttabs",
			},
		},
		{
			name: "comments and empty lines",
			input: `# This is a comment
KEY1=value1

# Another comment
KEY2=value2
KEY3=value3 # inline comment`,
			want: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
		},
		{
			name: "empty values",
			input: `EMPTY=
EMPTY_QUOTED=""
SPACES=   `,
			want: map[string]string{
				"EMPTY":        "",
				"EMPTY_QUOTED": "",
				"SPACES":       "",
			},
		},
		{
			name: "special characters",
			input: `URL=https://example.com/path?query=value
EMAIL=user@example.com
PATH=/usr/local/bin:/usr/bin`,
			want: map[string]string{
				"URL":   "https://example.com/path?query=value",
				"EMAIL": "user@example.com",
				"PATH":  "/usr/local/bin:/usr/bin",
			},
		},
		{
			name: "unicode",
			input: `EMOJI=Hello 👋 World 🌍
CHINESE=你好世界
MIXED="Unicode: 你好 👋"`,
			want: map[string]string{
				"EMOJI":   "Hello 👋 World 🌍",
				"CHINESE": "你好世界",
				"MIXED":   "Unicode: 你好 👋",
			},
		},
		{
			name: "spaces around equals",
			input: `SPACE_BEFORE =value
SPACE_AFTER= value
SPACE_BOTH = value
NO_SPACE=value`,
			want: map[string]string{
				"SPACE_BEFORE": "value",
				"SPACE_AFTER":  "value",
				"SPACE_BOTH":   "value",
				"NO_SPACE":     "value",
			},
		},
		{
			name: "export prefix",
			input: `export KEY1=value1
export KEY2="value2"
KEY3=value3`,
			want: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(nil)
			got, err := p.ParseString(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParser_Interpolation(t *testing.T) {
	opts := &formats.Options{
		ExpandVariables: true,
		Variables: map[string]string{
			"BASE_PATH": "/app",
			"PORT":      "3000",
		},
	}

	p := NewParser(opts)

	input := `APP_PATH=${BASE_PATH}/data
SERVER_URL=http://localhost:${PORT}
DEFAULT=${MISSING:-default_value}
SIMPLE=$PORT`

	want := map[string]string{
		"APP_PATH":   "/app/data",
		"SERVER_URL": "http://localhost:3000",
		"DEFAULT":    "default_value",
		"SIMPLE":     "3000",
	}

	got, err := p.ParseString(input)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestParser_EdgeCases(t *testing.T) {
	p := NewParser(&formats.Options{StrictMode: true})

	// Test invalid key
	_, err := p.ParseString(`123_INVALID=value`)
	assert.Error(t, err)

	// Test duplicate key
	_, err = p.ParseString(`KEY=value1
KEY=value2`)
	assert.Error(t, err)

	// Test unclosed quote
	_, err = p.ParseString(`UNCLOSED="no closing quote`)
	assert.Error(t, err)
}

func TestParser_ComplexMultiline(t *testing.T) {
	input := `PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA3Tz2mr7SZiAMfQyuvBjM9Oi..Z1BjP5CE/Wm/Rr500P
RK+Lh9x5eJPo5CAZ3/ANBE0sTK0ZsDGMak2m1g7..3VHqIxFTz0Ta1d+NAj
-----END RSA PRIVATE KEY-----"

JSON_DATA='{"name":"test","value":123}'`

	p := NewParser(nil)
	got, err := p.ParseString(input)
	require.NoError(t, err)

	assert.Contains(t, got["PRIVATE_KEY"], "BEGIN RSA")
	assert.Contains(t, got["PRIVATE_KEY"], "END RSA")
	assert.Equal(t, `{"name":"test","value":123}`, got["JSON_DATA"])
}

func TestParser_WindowsLineEndings(t *testing.T) {
	input := "KEY1=value1\r\nKEY2=value2\r\nKEY3=value3"

	p := NewParser(nil)
	got, err := p.ParseString(input)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}, got)
}

func TestParser_ExtendedMetadata(t *testing.T) {
	input := `# Database configuration
DB_HOST=localhost
DB_PORT=5432

# API settings
API_KEY=secret123
# This is the API URL
API_URL=https://api.example.com`

	p := NewParser(&formats.Options{PreserveComments: true})
	result, err := p.ParseExtended(strings.NewReader(input))
	require.NoError(t, err)

	assert.Equal(t, "localhost", result.Data["DB_HOST"])
	assert.Equal(t, "Database configuration", result.Comments["DB_HOST"])
	assert.Equal(t, "This is the API URL", result.Comments["API_URL"])
	assert.Equal(t, []string{"DB_HOST", "DB_PORT", "API_KEY", "API_URL"}, result.Order)
}

func BenchmarkParser_Parse(b *testing.B) {
	content := generateLargeEnvFile(1000)
	p := NewParser(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ParseString(content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func generateLargeEnvFile(entries int) string {
	var sb strings.Builder
	for i := 0; i < entries; i++ {
		fmt.Fprintf(&sb, "KEY_%d=value_%d\n", i, i)
	}
	return sb.String()
}
