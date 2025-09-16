package initalize

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/cmd/cliutil"
)

var UninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Piri system service",
	Long: `Uninstall removes the Piri systemd services and symlinks.

This command performs the following operations:
  - Stops all running piri services (piri.service and piri-updater.timer if enabled)
  - Disables systemd services
  - Removes symlinks from /etc/systemd/system/
  - Removes the CLI symlink from /usr/local/bin/piri
  - Reloads systemd daemon

NOTE: The binaries in /opt/piri are preserved for version management.
To completely remove piri, manually delete /opt/piri after uninstalling.

Requirements:
  - Linux operating system with systemd
  - Root privileges (run with sudo)`,
	Args: cobra.NoArgs,
	RunE: runUninstall,
}

func init() {
	UninstallCmd.SetOut(os.Stdout)
	UninstallCmd.SetErr(os.Stderr)
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	// Check platform - uninstall only works on Linux with systemd
	if runtime.GOOS != "linux" {
		return fmt.Errorf("uninstall command is only supported on Linux (systemd required). Current platform: %s", runtime.GOOS)
	}

	// Check root privileges
	if !cliutil.IsRunningAsRoot() {
		return fmt.Errorf("uninstall command requires root privileges. Re-run with `sudo`")
	}

	cmd.PrintErrln("Uninstalling Piri...")

	// Determine which services might be installed
	// We check for all possible services that could have been installed
	services := []string{
		cliutil.PiriServiceName,
		cliutil.PiriUpdateTimerName,
		cliutil.PiriUpdateServiceFile,
	}

	if err := uninstall(services); err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}

	cmd.PrintErrln("\nPiri has been successfully uninstalled!")
	return nil
}

// stopAndDisableService stops a running service and disables it from auto-start
func stopAndDisableService(service string) error {
	var errs error

	// Check if service exists and is active
	output, err := exec.Command("systemctl", "is-active", service).Output()
	status := strings.TrimSpace(string(output))

	// If service is active, try to stop it
	if err == nil && status == "active" {
		if err := exec.Command("systemctl", "stop", service).Run(); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("failed to stop service %s: %w", service, err))
		}
	}

	// Disable the service to prevent it from starting on boot
	// We do this even if the service wasn't running, as it might be enabled
	// The disable command will succeed even if the service doesn't exist
	if err := exec.Command("systemctl", "disable", service).Run(); err != nil {
		// Only log disable errors if they're not "service not found" errors
		if !strings.Contains(err.Error(), "exit status") {
			errs = multierror.Append(errs, fmt.Errorf("failed to disable service %s: %w", service, err))
		}
	}

	return errs
}

// removeServiceFile removes the symlink for a systemd service file
func removeServiceFile(serviceName string) error {
	symlinkPath := filepath.Join(cliutil.SystemDPath, serviceName)
	if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove symlink %s: %w", symlinkPath, err)
	}
	return nil
}

// uninstall removes piri installation and stops services
// It's used both for cleanup on failed install and for explicit uninstall command
func uninstall(services []string) error {
	var errs error

	// Stop and disable all services
	for _, service := range services {
		if err := stopAndDisableService(service); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	// If we couldn't stop services, don't proceed with cleanup
	if errs != nil {
		return errs
	}

	// Note: We do NOT remove /opt/piri/* - binaries are versioned and preserved
	// Users can manually clean up old versions if desired

	// Clean up service file symlinks
	for _, serviceFile := range []string{
		cliutil.PiriServiceFile,
		cliutil.PiriUpdateServiceFile,
		cliutil.PiriUpdateTimerServiceFile,
	} {
		if err := removeServiceFile(serviceFile); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	// Remove CLI symlink from /usr/local/bin
	if err := os.Remove(cliutil.PiriCLISymlinkPath); err != nil && !os.IsNotExist(err) {
		errs = multierror.Append(errs, fmt.Errorf("failed to remove CLI symlink %s: %w", cliutil.PiriCLISymlinkPath, err))
	}

	// Remove sudoers file (always created during install for potential auto-updates)
	if err := os.Remove(cliutil.PiriSudoersFile); err != nil && !os.IsNotExist(err) {
		errs = multierror.Append(errs, fmt.Errorf("failed to remove sudoers file %s: %w", cliutil.PiriSudoersFile, err))
	}

	// Reload systemd to recognize services are gone
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("failed to reload systemd: %w", err))
	}
	return errs
}
