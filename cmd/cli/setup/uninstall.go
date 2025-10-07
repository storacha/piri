package setup

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	Args:   cobra.NoArgs,
	RunE:   runUninstall,
	Hidden: true,
}

func init() {
	UninstallCmd.SetOut(os.Stdout)
	UninstallCmd.SetErr(os.Stderr)
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	// Initialize installer for uninstall operations
	installer, err := NewInstaller()
	if err != nil {
		return err
	}

	// Check prerequisites
	prereqs := &Prerequisites{
		Platform:     installer.Platform,
		NeedsSystemd: true,
		NeedsRoot:    true,
	}

	if err := prereqs.Validate(true); err != nil {
		return err
	}

	cmd.PrintErrln("Uninstalling Piri...")

	// Perform uninstall
	if err := installer.Uninstall(cmd); err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}

	cmd.PrintErrln("\nPiri has been successfully uninstalled!")
	cmd.PrintErrln("Note: Binaries in /opt/piri are preserved.")
	cmd.PrintErrln("To completely remove piri: sudo rm -rf /opt/piri")

	return nil
}
