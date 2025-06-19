package interpolation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterpolator_Basic(t *testing.T) {
	vars := map[string]string{
		"USER":     "john",
		"HOME":     "/home/john",
		"APP_NAME": "myapp",
	}

	i := NewInterpolator(vars, nil)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple variable",
			input: "Hello ${USER}!",
			want:  "Hello john!",
		},
		{
			name:  "multiple variables",
			input: "${USER}'s home is ${HOME}",
			want:  "john's home is /home/john",
		},
		{
			name:  "simple dollar format",
			input: "User: $USER, App: $APP_NAME",
			want:  "User: john, App: myapp",
		},
		{
			name:  "default value",
			input: "${MISSING:-default}",
			want:  "default",
		},
		{
			name:  "no substitution",
			input: "No variables here",
			want:  "No variables here",
		},
		{
			name:  "keep unresolved",
			input: "${UNKNOWN_VAR}",
			want:  "${UNKNOWN_VAR}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := i.Interpolate(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInterpolator_ErrorHandling(t *testing.T) {
	i := NewInterpolator(nil, &Options{
		FailOnMissing: true,
	})

	// Test missing variable with error operator
	_, err := i.Interpolate("${MISSING:?Variable not set}")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Variable not set")

	// Test missing variable with fail on missing
	_, err = i.Interpolate("${MISSING}")
	assert.Error(t, err)
}

func TestInterpolator_Recursive(t *testing.T) {
	vars := map[string]string{
		"BASE":     "/app",
		"DATA_DIR": "${BASE}/data",
		"LOG_DIR":  "${DATA_DIR}/logs",
	}

	i := NewInterpolator(vars, &Options{
		RecursiveResolve: true,
	})

	// First add variables to interpolator
	i.SetVariables(vars)

	got, err := i.Interpolate("${LOG_DIR}")
	require.NoError(t, err)
	assert.Equal(t, "/app/data/logs", got)
}

func TestInterpolator_CircularReference(t *testing.T) {
	vars := map[string]string{
		"A": "${B}",
		"B": "${C}",
		"C": "${A}",
	}

	i := NewInterpolator(vars, &Options{
		RecursiveResolve: true,
		MaxDepth:         10,
	})

	_, err := i.Interpolate("${A}")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference")
}

func TestInterpolator_MaxDepth(t *testing.T) {
	vars := map[string]string{
		"A": "${B}",
		"B": "${A}",
	}

	i := NewInterpolator(vars, &Options{
		RecursiveResolve: true,
		MaxDepth:         2,
	})

	_, err := i.Interpolate("${A}")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum interpolation depth exceeded")
}

func TestInterpolator_Map(t *testing.T) {
	data := map[string]string{
		"BASE_URL":    "https://api.example.com",
		"API_VERSION": "v1",
		"ENDPOINT":    "${BASE_URL}/${API_VERSION}",
		"FULL_URL":    "${ENDPOINT}/users",
	}

	i := NewInterpolator(nil, nil)

	result, err := i.InterpolateMap(data)
	require.NoError(t, err)

	assert.Equal(t, "https://api.example.com", result["BASE_URL"])
	assert.Equal(t, "v1", result["API_VERSION"])
	assert.Equal(t, "https://api.example.com/v1", result["ENDPOINT"])
	assert.Equal(t, "https://api.example.com/v1/users", result["FULL_URL"])
}

func TestInterpolator_ComplexPatterns(t *testing.T) {
	vars := map[string]string{
		"USER":   "john",
		"DOMAIN": "example.com",
		"PORT":   "8080",
		"SECURE": "true",
	}

	i := NewInterpolator(vars, nil)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "nested with default",
			input: "${EMAIL:-${USER}@${DOMAIN}}",
			want:  "john@example.com",
		},
		{
			name:  "conditional URL",
			input: "http${SECURE:+s}://${DOMAIN}:${PORT}",
			want:  "https://example.com:8080",
		},
		{
			name:  "escaped dollar",
			input: "Price: \\$${PORT}",
			want:  "Price: \\$8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := i.Interpolate(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
