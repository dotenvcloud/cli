package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	"github.com/dotenv/cli/internal/build"
	"github.com/dotenv/cli/internal/constants"
	"github.com/dotenv/cli/internal/ui"
)

var (
	updateCheck   bool
	updateVersion string
	updateForce   bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update DotEnv CLI to the latest version",
	Long: `Check for updates and install the latest version of DotEnv CLI.

The update command will:
1. Check the current version
2. Query for the latest available version
3. Download and install the update if available
4. Verify the installation`,

	Example: `  # Check for updates and install if available
  dotenv update

  # Only check for updates without installing
  dotenv update --check

  # Install a specific version
  dotenv update --version=1.2.3

  # Force update even if already on latest
  dotenv update --force`,

	RunE: runUpdate,
}

//nolint:gochecknoinits // cobra subcommand flag registration is idiomatic in init
func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false,
		"only check for updates, don't install")
	updateCmd.Flags().StringVar(&updateVersion, "version", "",
		"install specific version")
	updateCmd.Flags().BoolVar(&updateForce, "force", false,
		"force update even if already on latest version")
}

type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	currentVersion := build.GetInfo().Version

	ui.PrintInfof("Current version: %s", currentVersion)

	// Get latest release info
	latest, err := getLatestRelease(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(latest.TagName, "v")

	// Compare versions
	current, err := semver.NewVersion(currentVersion)
	if err != nil {
		// Development version
		current, _ = semver.NewVersion("0.0.0")
	}

	latestSemver, err := semver.NewVersion(latestVersion)
	if err != nil {
		return fmt.Errorf("invalid version format: %s", latestVersion)
	}

	if !updateForce && !latestSemver.GreaterThan(current) {
		ui.PrintSuccessf("You are already on the latest version!")
		return nil
	}

	ui.PrintInfof("New version available: %s", latestVersion)

	if updateCheck {
		fmt.Println("\nRelease notes:")
		fmt.Println(latest.Body)
		fmt.Println("\nRun 'dotenv update' to install")
		return nil
	}

	// Confirm update
	if updateVersion == "" && !updateForce {
		confirm, err := ui.Confirm(
			fmt.Sprintf("Update to version %s?", latestVersion), true)
		if err != nil {
			return err
		}
		if !confirm {
			ui.PrintInfof("Update canceled")
			return nil
		}
	}

	// Download and install
	return installUpdate(cmd.Context(), latest)
}

func getLatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	url := "https://api.github.com/repos/dotenv/cli/releases/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	// GitHub requires a user agent
	req.Header.Set("User-Agent", constants.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func installUpdate(ctx context.Context, release *ReleaseInfo) error {
	downloadURL, err := findReleaseAsset(release)
	if err != nil {
		return err
	}

	ui.PrintInfof("Downloading update...")

	tmpFile, err := downloadRelease(ctx, downloadURL)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	if chmodErr := os.Chmod(tmpFile, 0o755); chmodErr != nil {
		return chmodErr
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	if err := replaceBinary(tmpFile, exePath); err != nil {
		return err
	}

	ui.PrintSuccessf("Successfully updated to version %s!", release.TagName)
	cmd := exec.Command(exePath, "version")
	output, _ := cmd.Output()
	fmt.Print(string(output))
	return nil
}

func findReleaseAsset(release *ReleaseInfo) (string, error) {
	assetName := fmt.Sprintf("dotenv_%s_%s_%s",
		strings.TrimPrefix(release.TagName, "v"),
		runtime.GOOS,
		runtime.GOARCH,
	)
	if runtime.GOOS == platformWindows {
		assetName += ".exe"
	}
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetName) {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no release found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

func downloadRelease(ctx context.Context, url string) (string, error) {
	tmpFile, err := os.CreateTemp("", "dotenv-update-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_ = tmpFile.Close()
		return "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	if size := resp.ContentLength; size > 0 {
		ui.PrintInfof("Download size: %.2f MB", float64(size)/1024/1024)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}
	return tmpPath, nil
}

func replaceBinary(srcPath, exePath string) error {
	if runtime.GOOS == platformWindows {
		backup := exePath + ".backup"
		_ = os.Remove(backup)
		if err := os.Rename(exePath, backup); err != nil {
			return fmt.Errorf("failed to backup current binary: %w", err)
		}
		defer os.Remove(backup)
	}

	if err := os.Rename(srcPath, exePath); err == nil {
		return nil
	}
	return copyFile(srcPath, exePath)
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Chmod(0o755)
}
