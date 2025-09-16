package update

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/client"
)

/*
This represents the ideal update condition
- a challenge has been issued.
- piri completed the challenge
- the next challenge window is in 30 mins

*/

/*
╔═══════════════════════════════════════════════════════════════╗
║                        PROOF SET STATE                        ║
╚═══════════════════════════════════════════════════════════════╝

Note: Timestamps are estimated based on current epoch alignment with system time (30-second epochs).

CONFIGURATION
─────────────────────────
  Proof Set ID:            566
  Proving Period:          60 epochs (30 minutes)
  Challenge Window:        30 epochs (15 minutes)
  Owners:                  0x7469B47e006D0660aB92AE560b27A1075EEcF97F
                           0x0000000000000000000000000000000000000000
  Initialized:             true

SYSTEM VIEW (Local Node)
────────────────────────
  Current Epoch:           3012692 (est. 2025-09-12 19:59:04)
  Next Challenge Epoch:    3012690 (est. 2025-09-12 19:58:04, 1 minutes ago)
  Previous Challenge:      3012660 (est. 2025-09-12 19:43:04, 16 minutes ago)

  Status:
    • Challenge Issued:    true
    • In Challenge Window: true (ends epoch 3012720 (est. 2025-09-12 20:13:04, in 14 minutes))
    • In Fault State:      false
    • Has Proven:          true
    • Is Proving:          false

CONTRACT STATE (On-Chain)
─────────────────────────
  Next Challenge Window:   3012750 (est. 2025-09-12 20:28:04, in 29 minutes)
  Next Challenge Epoch:    3012690 (est. 2025-09-12 19:58:04, 1 minutes ago)
  Max Proving Period:      60 epochs (30 minutes)
  Challenge Window:        0 epochs (0 seconds)
  Challenge Range:         772323072

  Fees:
    • Proof Fee:           114.67 nanoFIL
    • Buffered Fee:        344.02 nanoFIL
*/

var (
	InternalUpdateCmd = &cobra.Command{
		Use:    "update-internal",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE:   doUpdateInternal,
	}
)

func init() {
	UpdateCmd.SetOut(os.Stdout)
	UpdateCmd.SetErr(os.Stderr)
}

// isRunningUnderSystemd checks if the current process is running under systemd
func isRunningUnderSystemd() bool {
	// Check if INVOCATION_ID is set - systemd sets this for all services
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	// Also check for SYSTEMD_EXEC_PID which is set in newer systemd versions
	if os.Getenv("SYSTEMD_EXEC_PID") != "" {
		return true
	}
	// Check if systemctl is available and we can query our service
	if _, err := exec.LookPath("systemctl"); err == nil {
		// Check if piri service exists
		if err := exec.Command("systemctl", "list-units", "--full", "--all", "--plain", "--no-pager", "piri.service").Run(); err == nil {
			return true
		}
	}
	return false
}

func doUpdateInternal(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// only linux can do auto update, since the "auto" bits require service files
	if runtime.GOOS != "linux" {
		return fmt.Errorf("internal update not supported on %s platform", runtime.GOOS)
	}

	// Check if safe to update
	status, err := client.GetNodeStatus(ctx)
	if err != nil {
		// If we can't determine status, skip this update cycle
		cmd.PrintErrf("Cannot determine node status: %v\n", err)
		cmd.Println("Abort update")
		return nil
	}

	if !status.UpgradeSafe {
		if status.IsProving {
			cmd.Println("Node is actively proving, abort update")
		} else if status.InChallengeWindow && !status.HasProven {
			cmd.Println("Node is in an unproven challenge window, abort update")
		} else {
			cmd.Println("Node is busy, abort update")
		}
		return nil
	}

	// Check for available updates
	updateInfo, err := CheckForUpdate(ctx, cmd)
	if err != nil {
		return err
	}

	if !updateInfo.NeedsUpdate {
		cmd.Println("Already running the latest version")
		return nil
	}

	cmd.Println("Update available and safe to install")

	// Get executable path
	execPath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	// Check permissions - fail if we can't update
	if needsElevatedPrivileges(execPath) {
		if !cliutil.IsRunningAsRoot() {
			return fmt.Errorf("internal update lacks permissions for %s", execPath)
		}
	}

	// For managed installations, we need to handle the update differently
	if strings.HasPrefix(execPath, cliutil.PiriOptDir) {
		// This is a managed installation - apply update to new version directory
		if err := applyManagedUpdate(ctx, cmd, updateInfo.Release); err != nil {
			return err
		}
	} else {
		// Regular update (standalone binary)
		if err := DownloadAndApplyUpdate(ctx, cmd, updateInfo.Release, execPath, false); err != nil {
			return err
		}
	}

	// Restart the service if running under systemd
	if isRunningUnderSystemd() {
		// Use sudo to restart the service (sudoers file allows this without password)
		// The sudoers rule ONLY allows: /usr/bin/systemctl restart piri
		if err := exec.Command("sudo", "systemctl", "restart", "piri").Run(); err != nil {
			return fmt.Errorf("failed to restart piri service via sudo systemctl: %w", err)
		}
		cmd.Println("Restarted piri service via systemctl")
	} else {
		// We're not running under systemd - this shouldn't normally happen
		// since update-internal is designed to be called by the systemd timer
		// but we should handle it gracefully
		cmd.Println("Warning: Not running under systemd, cannot auto-restart")
		cmd.Println("Please restart piri manually for the update to take effect")
	}

	return nil
}

// applyManagedUpdate handles updates for managed installations in /opt/piri
func applyManagedUpdate(ctx context.Context, cmd *cobra.Command, release *GitHubRelease) error {
	newVersion := strings.TrimPrefix(release.TagName, "v")
	versionedBinDir := cliutil.GetVersionedBinaryDir(newVersion)
	versionedBinPath := filepath.Join(versionedBinDir, "piri")

	// Check if this version already exists
	if _, err := os.Stat(versionedBinPath); err == nil {
		cmd.Printf("Version %s already installed at %s\n", newVersion, versionedBinPath)
		return nil
	}

	// Create the new version directory
	if err := os.MkdirAll(versionedBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create version directory %s: %w", versionedBinDir, err)
	}

	// Find the appropriate asset for this platform
	assetURL, err := findAssetURL(release)
	if err != nil {
		return fmt.Errorf("failed to find appropriate release asset: %w", err)
	}

	cmd.Printf("Downloading update from %s\n", assetURL)

	// Get the filename from the URL
	assetFileName := filepath.Base(assetURL)

	// Download and parse checksums
	cmd.Println("Fetching checksums...")
	checksum, err := getAssetChecksum(ctx, cmd, release, assetFileName)
	if err != nil {
		return fmt.Errorf("failed to get asset checksum, aborting update: %w", err)
	}

	// Download and verify the archive, then extract the binary
	newBinary, err := downloadAndVerifyBinary(ctx, cmd, assetURL, checksum, false)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer func() {
		_ = newBinary.Close()
	}()

	// Read the binary content
	binaryData, err := io.ReadAll(newBinary)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	// Write the new binary to the versioned directory
	if err := os.WriteFile(versionedBinPath, binaryData, 0755); err != nil {
		return fmt.Errorf("failed to write new binary: %w", err)
	}

	cmd.Printf("Installed new version to %s\n", versionedBinPath)

	// Get the user who owns /opt/piri
	fileInfo, err := os.Stat(cliutil.PiriOptDir)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", cliutil.PiriOptDir, err)
	}
	sys := fileInfo.Sys()
	if stat, ok := sys.(*syscall.Stat_t); ok {
		// Set ownership of the new version directory to match /opt/piri
		if err := os.Chown(versionedBinPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return fmt.Errorf("failed to set ownership: %w", err)
		}
		if err := os.Chown(versionedBinDir, int(stat.Uid), int(stat.Gid)); err != nil {
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	// Update the current symlink to point to the new version
	// First remove the old symlink
	if err := os.Remove(cliutil.PiriCurrentSymlink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old symlink: %w", err)
	}

	// Create new symlink
	if err := os.Symlink(versionedBinDir, cliutil.PiriCurrentSymlink); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	cmd.Printf("Updated symlink %s -> %s\n", cliutil.PiriCurrentSymlink, versionedBinDir)

	// Note: The service restart happens in the calling function after this returns
	return nil
}
