package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// UserHomeDir returns the user's home directory
func UserHomeDir() (string, error) {
	// Check for DOTENV_CONFIG_DIR override first
	if configDir := os.Getenv("DOTENV_CONFIG_DIR"); configDir != "" {
		// Return parent of config dir as home
		return filepath.Dir(configDir), nil
	}

	home := os.Getenv("HOME")

	if home == "" && runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
	}

	if home == "" {
		home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
	}

	if home == "" {
		return "", fmt.Errorf("unable to determine home directory")
	}

	return home, nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EnsureDir ensures a directory exists
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0700)
}

// IsTerminal checks if output is a terminal
func IsTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
