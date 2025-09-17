package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/client"
)

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

func doUpdateInternal(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Create platform checker
	platform, err := NewPlatformChecker()
	if err != nil {
		return err
	}

	// Check prerequisites
	if err := platform.RequireLinux(); err != nil {
		return fmt.Errorf("internal update %w", err)
	}

	// Get executable path
	execPath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	// Check permissions based on installation type
	if !platform.IsManagedInstallation(execPath) {
		return fmt.Errorf("cannot auto update an unmanaged installation")
	}

	if !platform.IsRunningUnderSystemd() {
		return fmt.Errorf("cannot auto update an unmanaged installation")
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
	updateInfo, err := checkForUpdate(ctx, cmd)
	if err != nil {
		return err
	}

	if !updateInfo.NeedsUpdate {
		cmd.Println("Already running the latest version")
		return nil
	}

	cmd.Println("Update available and safe to install")

	// For managed installations: check if we can write to /opt/piri/bin/ directory
	// to create new version directories and update symlinks, we should be able to as `piri install` sets this up
	binDir := filepath.Join(PiriOptDir, "bin")
	if NeedsElevatedPrivileges(binDir) {
		if !platform.IsRoot {
			return fmt.Errorf("internal update lacks permissions for %s", binDir)
		}
	}

	// This is a managed installation - apply update to new version directory
	rollbackFunc, err := applyManagedUpdate(ctx, cmd, updateInfo.Release)
	if err != nil {
		return err
	}

	// If rollbackFunc is nil, the version was already installed and active
	if rollbackFunc == nil {
		cmd.Println("Version already active, no restart needed")
		return nil
	}

	// Restart the service to apply update
	cmd.Println("Restarting service to apply update...")

	sm := NewServiceManager("piri")
	// Use the new verifyServiceRestart function which checks the service actually starts
	if err := sm.VerifyServiceRestart("piri", 10, true); err != nil {
		cmd.PrintErrln("Service restart failed, attempting rollback...")

		if rollbackFunc != nil {
			// Rollback symlink to previous version
			if rollbackErr := rollbackFunc(); rollbackErr != nil {
				return fmt.Errorf("restart failed and rollback failed: restart=%w, rollback=%w",
					err, rollbackErr)
			}

			cmd.Println("Rolled back to previous version, attempting restart...")

			// Try to restart with old version
			if restartErr := sm.VerifyServiceRestart("piri", 10, true); restartErr != nil {
				return fmt.Errorf("critical: service won't start with either version: new=%w, old=%w",
					err, restartErr)
			}

			cmd.Println("Successfully rolled back and restarted with previous version")
			return fmt.Errorf("update failed but successfully rolled back: %w", err)
		}

		// No rollback available (standalone binary path)
		return fmt.Errorf("service restart failed and no rollback available: %w", err)
	}

	cmd.Println("Service restarted successfully with new version")

	return nil
}

// applyManagedUpdate handles updates for managed installations in /opt/piri
// Returns a rollback function that can restore the previous version if the update fails
func applyManagedUpdate(ctx context.Context, cmd *cobra.Command, release *GitHubRelease) (rollback func() error, err error) {
	newVersion := strings.TrimPrefix(release.TagName, "v")
	versionedBinDir := getVersionedBinaryDir(newVersion)
	versionedBinPath := filepath.Join(versionedBinDir, "piri")

	// Check if this version already exists
	if FileExists(versionedBinPath) {
		// Check if the symlink actually points to this version
		currentTarget, err := os.Readlink(PiriCurrentSymlink)
		if err == nil && currentTarget == versionedBinDir {
			// Symlink points to this version - it's truly installed
			cmd.Printf("Version %s already installed and active at %s\n", newVersion, versionedBinPath)
			return nil, nil
		}

		// Version exists but symlink points elsewhere - this version previously failed
		cmd.Printf("Found failed version %s at %s, removing and re-downloading...\n", newVersion, versionedBinPath)
		if err := os.RemoveAll(versionedBinDir); err != nil {
			cmd.Printf("Warning: Failed to remove old version directory: %v\n", err)
			// Continue anyway - the download will fail if we can't write
		}
	}

	fsm := NewFileSystemManager()

	// Create the new version directory
	if err := fsm.CreateDirectory(versionedBinDir, 0755); err != nil {
		return nil, err
	}

	// Use the unified download function with the specific target path
	// Pass empty execPath since we're specifying targetPath
	if err := downloadAndApplyUpdate(ctx, cmd, release, "", versionedBinPath, false); err != nil {
		// If download fails, try to clean up the version directory we created
		_ = os.RemoveAll(versionedBinDir)
		return nil, fmt.Errorf("failed to download and install update: %w", err)
	}

	cmd.Printf("Installed new version to %s\n", versionedBinPath)

	// Set ownership to match /opt/piri
	if err := fsm.SetOwnershipFromPath(versionedBinPath, PiriOptDir); err != nil {
		return nil, err
	}
	if err := fsm.SetOwnershipFromPath(versionedBinDir, PiriOptDir); err != nil {
		return nil, err
	}

	// Perform symlink update with rollback capability
	oldTarget, symlinkRollbackFunc, err := fsm.UpdateSymlinkAtomic(
		PiriCurrentSymlink,
		versionedBinDir,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update symlink: %w", err)
	}

	if oldTarget != "" {
		cmd.Printf("Updated symlink %s -> %s (previous: %s)\n",
			PiriCurrentSymlink, versionedBinDir, oldTarget)
	} else {
		cmd.Printf("Created symlink %s -> %s\n",
			PiriCurrentSymlink, versionedBinDir)
	}

	// Create versioned systemd directory and write service files
	installer, err := NewInstaller()
	if err != nil {
		return nil, fmt.Errorf("failed to create installer: %w", err)
	}

	// Get the service user from platform
	platform, err := NewPlatformChecker()
	if err != nil {
		return nil, fmt.Errorf("failed to get platform info: %w", err)
	}

	// Create versioned systemd directory
	versionedSystemdDir := getVersionedSystemdDir(newVersion)
	if err := fsm.CreateDirectory(versionedSystemdDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create systemd directory: %w", err)
	}

	// Generate and write service files to the versioned directory
	serviceFiles := installer.GenerateSystemdServices(platform.ServiceUser, newVersion)
	for _, svc := range serviceFiles {
		if err := fsm.WriteFile(svc.SourcePath, []byte(svc.Content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write service file %s: %w", svc.Name, err)
		}
	}

	// Set ownership on systemd directory
	if err := fsm.SetOwnershipFromPath(versionedSystemdDir, PiriOptDir); err != nil {
		return nil, fmt.Errorf("failed to set ownership on systemd dir: %w", err)
	}

	// Update systemd symlink atomically
	oldSystemdTarget, systemdRollbackFunc, err := fsm.UpdateSymlinkAtomic(
		PiriSystemdCurrentSymlink,
		versionedSystemdDir,
	)
	if err != nil {
		// Rollback binary symlink and fail
		_ = symlinkRollbackFunc()
		return nil, fmt.Errorf("failed to update systemd symlink: %w", err)
	}

	if oldSystemdTarget != "" {
		cmd.Printf("Updated systemd symlink %s -> %s (previous: %s)\n",
			PiriSystemdCurrentSymlink, versionedSystemdDir, oldSystemdTarget)
	} else {
		cmd.Printf("Created systemd symlink %s -> %s\n",
			PiriSystemdCurrentSymlink, versionedSystemdDir)
	}

	// Reload systemd daemon to pick up the new symlinks
	sm := NewServiceManager("piri")
	if err := sm.executor.Run("sudo", "systemctl", "daemon-reload"); err != nil {
		cmd.Printf("Warning: Failed to reload systemd daemon: %v\n", err)
	}

	// Create combined rollback function that handles both binary and systemd symlinks
	rollbackFunc := func() error {
		var errs error

		// Rollback binary symlink
		if err := symlinkRollbackFunc(); err != nil {
			errs = fmt.Errorf("failed to rollback binary symlink: %w", err)
		}

		// Rollback systemd symlink
		if err := systemdRollbackFunc(); err != nil {
			if errs != nil {
				errs = fmt.Errorf("%v; failed to rollback systemd symlink: %w", errs, err)
			} else {
				errs = fmt.Errorf("failed to rollback systemd symlink: %w", err)
			}
		}

		// Reload systemd daemon after rollback
		if err := sm.executor.Run("sudo", "systemctl", "daemon-reload"); err != nil {
			if errs != nil {
				errs = fmt.Errorf("%v; failed to reload systemd after rollback: %w", errs, err)
			} else {
				errs = fmt.Errorf("failed to reload systemd after rollback: %w", err)
			}
		}

		return errs
	}

	// Return the combined rollback function for use if restart fails
	return rollbackFunc, nil
}
