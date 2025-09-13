package update

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/build"
)

var SupportedLinuxArch = map[string]bool{
	"amd64": true,
	"arm64": true,
}

var (
	UpdateCmd = &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Check for and apply updates to piri",
		Long:  `Check for new releases and update the piri binary to the latest version.`,
		RunE:  doUpdate,
	}

	dryRun bool
)

func init() {
	UpdateCmd.SetOut(os.Stdout)
	UpdateCmd.SetErr(os.Stderr)
	UpdateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Check for updates without applying them")
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	CreatedAt  time.Time `json:"created_at"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func doUpdate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Get the path to the current binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve any symlinks to get the real path
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Check if we need elevated privileges and handle sudo if necessary
	if !dryRun && needsElevatedPrivileges(execPath) {
		if !cliutil.IsRunningAsRoot() {
			cmd.Println("Update requires administrator privileges...")
			return cliutil.RunWithSudo()
		}
	}

	currentVersion := build.Version
	cmd.Printf("Current version: %s\n", currentVersion)

	// Check for latest release
	release, err := getLatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersionClean := strings.Split(strings.TrimPrefix(currentVersion, "v"), "-")[0]

	cmd.Printf("Latest version: %s\n", latestVersion)

	if currentVersionClean == latestVersion {
		cmd.Println("Already running the latest version")
		return nil
	}

	if dryRun {
		cmd.Printf("Update available: %s -> %s (dry-run mode, not applying)\n", currentVersionClean, latestVersion)
		return nil
	}

	// Find the appropriate asset for this platform
	assetURL, err := findAssetURL(release)
	if err != nil {
		return fmt.Errorf("failed to find appropriate release asset: %w", err)
	}

	cmd.Printf("Downloading update from %s\n", assetURL)

	// Get the filename from the URL
	assetFileName := path.Base(assetURL)

	// Download and parse checksums
	cmd.Println("Fetching checksums...")
	checksum, err := getAssetChecksum(ctx, cmd, release, assetFileName)
	if err != nil {
		return fmt.Errorf("failed to get asset checksum, aborting update: %w", err)
	}

	// Download and verify the archive, then extract the binary
	newBinary, err := downloadAndVerifyBinary(ctx, cmd, assetURL, checksum, true)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer newBinary.Close()

	// Apply the update (no checksum verification here since we already verified the archive)
	cmd.Println("Applying update...")
	err = selfupdate.Apply(newBinary, selfupdate.Options{
		TargetPath:  execPath,
		OldSavePath: execPath + ".old",
	})
	if err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("failed to apply update and rollback: %w", rerr)
		}
		return fmt.Errorf("failed to apply update: %w", err)
	}

	cmd.Printf("Successfully updated to version %s\n", latestVersion)
	cmd.Println("Please restart piri for the update to take effect")

	return nil
}
