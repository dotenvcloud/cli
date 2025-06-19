package integration

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIVersion(t *testing.T) {
	cmd := exec.Command("../../bin/dotenv", "version", "--short")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "v")
}

func TestCLIHelp(t *testing.T) {
	cmd := exec.Command("../../bin/dotenv", "--help")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "DotEnv CLI")
	assert.Contains(t, output, "Available Commands:")
}

func TestCLICommands(t *testing.T) {
	commands := []string{
		"init", "login", "pull", "push", "list",
		"export", "refresh", "update", "use-context",
	}

	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			cmd := exec.Command("../../bin/dotenv", command, "--help")
			var out bytes.Buffer
			cmd.Stdout = &out

			err := cmd.Run()
			require.NoError(t, err)

			output := out.String()
			assert.Contains(t, output, command)
		})
	}
}

func TestCLIGlobalFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"debug", []string{"--debug", "version"}},
		{"quiet", []string{"--quiet", "version"}},
		{"no-color", []string{"--no-color", "version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("../../bin/dotenv", tt.args...)
			err := cmd.Run()
			require.NoError(t, err)
		})
	}
}
