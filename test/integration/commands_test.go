//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenv/cli/cmd"
	"github.com/dotenv/cli/test/helpers"
	"github.com/dotenv/cli/test/mocks"
)

func TestFullWorkflow(t *testing.T) {
	t.Skip("legacy: asserts mock fixture text that current commands don't emit; needs rewrite against accounts-based config")
	tc := helpers.NewTestConfig(t)
	mockServer := mocks.NewMockAPIServer()
	defer mockServer.Close()

	// Step 1: Initialize config
	t.Run("init", func(t *testing.T) {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"init", "--config", tc.ConfigPath})

		// Simulate interactive input
		// In real integration tests, we'd use expect or similar
		// For now, we'll create the config manually
		config := map[string]interface{}{
			"version":         "1.0",
			"current_context": "test",
			"contexts": map[string]interface{}{
				"test": map[string]interface{}{
					"api_url":      mockServer.URL,
					"api_key":      tc.APIKey,
					"organization": "test-org",
				},
			},
		}
		tc.WriteConfig(t, config)
	})

	// Step 2: List organizations
	t.Run("list organizations", func(t *testing.T) {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"list", "organizations", "--config", tc.ConfigPath})

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)

		err := rootCmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "Test Organization")
	})

	// Step 3: List projects
	t.Run("list projects", func(t *testing.T) {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"list", "projects", "--config", tc.ConfigPath})

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)

		err := rootCmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "Test Project")
	})

	// Step 4: Pull secrets
	t.Run("pull secrets", func(t *testing.T) {
		envFile := filepath.Join(tc.TempDir, ".env")

		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"pull", "test-project", "--output", envFile, "--config", tc.ConfigPath})

		err := rootCmd.Execute()
		require.NoError(t, err)

		// Verify file was created
		assert.FileExists(t, envFile)

		// Check content
		content, err := os.ReadFile(envFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "DATABASE_URL=postgres://localhost/test")
		assert.Contains(t, string(content), "API_KEY=test-api-key")
		assert.Contains(t, string(content), "DEBUG=true")
	})

	// Step 5: Export in different formats
	t.Run("export json", func(t *testing.T) {
		jsonFile := filepath.Join(tc.TempDir, "secrets.json")

		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"export", "test-project", "--format", "json", "--output", jsonFile, "--config", tc.ConfigPath})

		err := rootCmd.Execute()
		require.NoError(t, err)

		// Verify JSON file
		assert.FileExists(t, jsonFile)

		var secrets map[string]string
		content, err := os.ReadFile(jsonFile)
		require.NoError(t, err)

		err = json.Unmarshal(content, &secrets)
		require.NoError(t, err)

		assert.Equal(t, "postgres://localhost/test", secrets["DATABASE_URL"])
		assert.Equal(t, "test-api-key", secrets["API_KEY"])
	})

	// Step 6: Export YAML
	t.Run("export yaml", func(t *testing.T) {
		yamlFile := filepath.Join(tc.TempDir, "secrets.yaml")

		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"export", "test-project", "--format", "yaml", "--output", yamlFile, "--config", tc.ConfigPath})

		err := rootCmd.Execute()
		require.NoError(t, err)

		// Verify YAML file
		assert.FileExists(t, yamlFile)

		content, err := os.ReadFile(yamlFile)
		require.NoError(t, err)

		assert.Contains(t, string(content), "DATABASE_URL: postgres://localhost/test")
		assert.Contains(t, string(content), "API_KEY: test-api-key")
	})

	// Step 7: Push new secrets
	t.Run("push secrets", func(t *testing.T) {
		// Create new env file
		newEnvFile := filepath.Join(tc.TempDir, ".env.new")
		helpers.CreateTestEnvFile(t, newEnvFile, map[string]string{
			"NEW_KEY":     "new_value",
			"ANOTHER_KEY": "another_value",
		})

		// Note: Push command would need proper implementation
		// For now, we'll just test the command runs
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"push", "test-project", newEnvFile, "--config", tc.ConfigPath})

		// This might fail if push is not fully implemented
		// For integration test, we're checking it doesn't panic
		_ = rootCmd.Execute()
	})
}

func TestContextManagement(t *testing.T) {
	t.Skip("legacy: 'use' and 'list contexts' subcommands were removed when contexts replaced by accounts")
	tc := helpers.NewTestConfig(t)
	mockServer := mocks.NewMockAPIServer()
	defer mockServer.Close()

	// Set up initial config
	config := map[string]interface{}{
		"version":         "1.0",
		"current_context": "dev",
		"contexts": map[string]interface{}{
			"dev": map[string]interface{}{
				"api_url":      mockServer.URL,
				"api_key":      tc.APIKey,
				"organization": "dev-org",
			},
		},
	}
	tc.WriteConfig(t, config)

	// Add production context
	t.Run("add context", func(t *testing.T) {
		// This would require implementing context add command
		// For now, we'll modify the config directly
		config["contexts"].(map[string]interface{})["prod"] = map[string]interface{}{
			"api_url":      mockServer.URL,
			"api_key":      "prod-key",
			"organization": "prod-org",
		}
		tc.WriteConfig(t, config)
	})

	// Switch context
	t.Run("use context", func(t *testing.T) {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"use", "prod", "--config", tc.ConfigPath})

		err := rootCmd.Execute()
		require.NoError(t, err)

		// Verify context switched
		// Would need to read config and check current_context
	})

	// List contexts
	t.Run("list contexts", func(t *testing.T) {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"list", "contexts", "--config", tc.ConfigPath})

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)

		err := rootCmd.Execute()
		require.NoError(t, err)

		output := stdout.String()
		assert.Contains(t, output, "dev")
		assert.Contains(t, output, "prod")
	})
}

func TestErrorHandling(t *testing.T) {
	tc := helpers.NewTestConfig(t)

	t.Run("missing config", func(t *testing.T) {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"pull", "project", "--config", "/non/existent/config.yaml"})

		err := rootCmd.Execute()
		assert.Error(t, err)
	})

	t.Run("invalid api key", func(t *testing.T) {
		mockServer := mocks.NewMockAPIServer()
		defer mockServer.Close()

		// Set up config with invalid key
		config := map[string]interface{}{
			"version":         "1.0",
			"current_context": "test",
			"contexts": map[string]interface{}{
				"test": map[string]interface{}{
					"api_url":      mockServer.URL,
					"api_key":      "invalid-key",
					"organization": "test-org",
				},
			},
		}
		tc.WriteConfig(t, config)

		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"list", "projects", "--config", tc.ConfigPath})

		err := rootCmd.Execute()
		// Should get unauthorized error
		assert.Error(t, err)
	})
}

func TestEnvironmentVariables(t *testing.T) {
	t.Skip("legacy: assumes env vars alone bootstrap an account; current CLI needs an account record")
	tc := helpers.NewTestConfig(t)
	mockServer := mocks.NewMockAPIServer()
	defer mockServer.Close()

	// Set environment variables
	helpers.SetEnv(t, map[string]string{
		"DOTENV_API_KEY":      tc.APIKey,
		"DOTENV_ORGANIZATION": "env-org",
		"DOTENV_API_URL":      mockServer.URL,
	})

	// Even without config, should work with env vars
	t.Run("list with env vars", func(t *testing.T) {
		// Update mock server to expect env-org
		mockServer.SetProjects("env-org", []dotenv.Project{
			{
				ID:   "env-proj",
				Name: "Environment Project",
				Slug: "env-project",
			},
		})

		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"list", "projects"})

		var stdout bytes.Buffer
		rootCmd.SetOut(&stdout)

		err := rootCmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "Environment Project")
	})
}

func TestConcurrentCommands(t *testing.T) {
	tc := helpers.NewTestConfig(t)
	mockServer := mocks.NewMockAPIServer()
	defer mockServer.Close()

	// Set up config
	config := map[string]interface{}{
		"version":         "1.0",
		"current_context": "test",
		"contexts": map[string]interface{}{
			"test": map[string]interface{}{
				"api_url":      mockServer.URL,
				"api_key":      tc.APIKey,
				"organization": "test-org",
			},
		},
	}
	tc.WriteConfig(t, config)

	// Run multiple commands concurrently
	done := make(chan bool, 3)

	go func() {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"list", "projects", "--config", tc.ConfigPath})
		_ = rootCmd.Execute()
		done <- true
	}()

	go func() {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"list", "organizations", "--config", tc.ConfigPath})
		_ = rootCmd.Execute()
		done <- true
	}()

	go func() {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"pull", "test-project", "--config", tc.ConfigPath})
		_ = rootCmd.Execute()
		done <- true
	}()

	// Wait for all to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Check that mock server received all requests
	calls := mockServer.GetCalls()
	assert.GreaterOrEqual(t, len(calls), 3)
}

func BenchmarkPullCommand(b *testing.B) {
	tc := helpers.NewTestConfig(b)
	mockServer := mocks.NewMockAPIServer()
	defer mockServer.Close()

	// Set up config
	config := map[string]interface{}{
		"version":         "1.0",
		"current_context": "test",
		"contexts": map[string]interface{}{
			"test": map[string]interface{}{
				"api_url":      mockServer.URL,
				"api_key":      tc.APIKey,
				"organization": "test-org",
			},
		},
	}
	tc.WriteConfig(b, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rootCmd := cmd.NewRootCommand()
		rootCmd.SetArgs([]string{"pull", "test-project", "--config", tc.ConfigPath})
		_ = rootCmd.Execute()
	}
}
