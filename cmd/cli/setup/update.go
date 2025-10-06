package setup

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/client"
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
		Long: `Check for new releases and update the piri binary to the latest version.

This command downloads and installs the latest version but does not restart the service.
To check if an update is safe, use 'piri status upgrade-check' first.`,
		RunE: doUpdate,
	}

	checkOnly bool
	force     bool
	version   string
)

func init() {
	UpdateCmd.SetOut(os.Stdout)
	UpdateCmd.SetErr(os.Stderr)
	UpdateCmd.Flags().BoolVar(&checkOnly, "check", false, "Check for updates without applying them")
	UpdateCmd.Flags().BoolVar(&force, "force", false, "Skip safety checks and force update")
	UpdateCmd.Flags().StringVar(&version, "version", "", "Update to specific version (e.g., v1.2.3)")
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

	// Create platform checker
	platform, err := NewPlatformChecker()
	if err != nil {
		return err
	}

	// Get executable path
	execPath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	// Check if this is a managed installation
	if platform.IsManagedInstallation(execPath) {
		cmd.Println("This is a managed piri installation.")
		cmd.Println("Manual updates are not supported for managed installations.")
		cmd.Println("")
		cmd.Println("Options:")
		cmd.Println("  1. Enable auto-updates: sudo systemctl enable --now piri-updater.timer")
		cmd.Println("  2. Reinstall with new version: Download new version and run 'sudo piri install --config <config>'")
		return fmt.Errorf("cannot manually update managed installation")
	}

	// Check for updates (allow all updates for manual updates)
	updateInfo, err := checkForUpdate(ctx, cmd, false)
	if err != nil {
		return err
	}

	if !updateInfo.NeedsUpdate {
		cmd.Println("Already running the latest version")
		return nil
	}

	if checkOnly {
		cmd.Printf("Update available: %s -> %s\n", updateInfo.CurrentVersion, updateInfo.LatestVersion)
		return nil
	}

	// Check if we need elevated privileges and handle sudo if necessary
	if NeedsElevatedPrivileges(execPath) {
		if !platform.IsRoot {
			cmd.Printf("Update requires administrator privileges to update piri in path: %s\n", execPath)
			cmd.Println("Re-run with `sudo` to update")
			return nil
		}
	}

	// Check if safe to update (unless --force)
	if !force {
		status, err := client.GetNodeStatus(ctx)
		if err != nil {
			cmd.PrintErrln("Warning: Cannot determine if safe to update:", err)
			cmd.PrintErrln("Use --force to update anyway")
			return fmt.Errorf("cannot determine node status")
		}

		if !status.UpgradeSafe {
			if status.IsProving {
				cmd.PrintErrln("Error: Node is currently proving")
			} else if status.InChallengeWindow && !status.HasProven {
				cmd.PrintErrln("Error: Node is in an unproven challenge window")
			}
			cmd.PrintErrln("Update blocked for safety. Use --force to override")
			return fmt.Errorf("not safe to update")
		}
	}

	// Apply the update (pass empty targetPath for standalone binary)
	if err := downloadAndApplyUpdate(ctx, cmd, updateInfo.Release, execPath, "", true); err != nil {
		return err
	}

	cmd.Println("Restart required for update to take effect")
	return nil
}
