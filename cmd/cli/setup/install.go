package setup

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/build"
	"github.com/storacha/piri/pkg/config"
)

var InstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Piri as a system service",
	Long: `Install configures Piri to run as a systemd service on Linux systems.

This command performs the following operations:
  - Installs the piri binary to /opt/piri/bin/{version}/piri
  - Creates the /opt/piri/etc directory for configuration
  - Installs the provided configuration file to /opt/piri/systemd, and symlinks to /etc/systemd/system/
  - Creates and enables systemd service files
  - Creates sudoers entry for service restart (required for auto-updates)
  - Optionally enables automatic updates (--enable-auto-update)

Requirements:
  - Linux operating system with systemd
  - Root privileges (run with sudo)
  - Configuration file from 'piri init' command

Automatic updates check for new releases every 30 minutes and apply them when
safe to do so (not during proof generation or active transfers).`,
	Args: cobra.NoArgs,
	RunE: runInstall,
}

func init() {
	InstallCmd.Flags().Bool("force", false, "Force overwrite existing files")
	InstallCmd.Flags().Bool("enable-auto-update", false, "Enable automatic updates (checks every 30 minutes)")

	InstallCmd.SetOut(os.Stdout)
	InstallCmd.SetErr(os.Stderr)
}

// runInstall is the main entry point for the install command
func runInstall(cmd *cobra.Command, _ []string) error {
	// Initialize installer
	installer, err := NewInstaller()
	if err != nil {
		return err
	}

	// Check prerequisites
	prereqs := &Prerequisites{
		Platform:     installer.Platform,
		NeedsSystemd: true,
		NeedsRoot:    true,
		CheckServices: []string{PiriServiceName, PiriUpdateTimerName},
		CheckFiles: []string{
			PiriSystemConfigPath,
			PiriServiceFilePath,
			PiriUpdateServiceFilePath,
			PiriUpdateTimerServiceFilePath,
		},
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("reading --force flag: %w", err)
	}

	if err := prereqs.Validate(force); err != nil {
		return err
	}

	// Display version info
	cmd.PrintErrf("Installing version: %s\n", build.Version)
	cmd.PrintErrf("Installation directory: %s\n", PiriOptDir)

	// Parse installation state
	state, err := parseInstallFlags(cmd, installer.Platform.ServiceUser)
	if err != nil {
		return err
	}

	// Perform installation with cleanup on failure
	return doInstall(cmd, installer, state)
}

// parseInstallFlags parses command flags and loads the config
func parseInstallFlags(cmd *cobra.Command, serviceUser string) (*InstallState, error) {
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return nil, fmt.Errorf("reading --force flag: %w", err)
	}

	enableAutoUpdate, err := cmd.Flags().GetBool("enable-auto-update")
	if err != nil {
		return nil, fmt.Errorf("reading --enable-auto-update flag: %w", err)
	}

	// NB: config is a persistent flag accessible by all commands
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, fmt.Errorf("reading --config flag: %w", err)
	}

	// Load config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	var cfg config.FullServerConfig
	if err := toml.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", configPath, err)
	}

	return &InstallState{
		ConfigPath:       configPath,
		Config:           cfg,
		Force:            force,
		EnableAutoUpdate: enableAutoUpdate,
		ServiceUser:      serviceUser,
		Version:          build.Version,
		IsUpdate:         false,
	}, nil
}

// doInstall performs the actual installation
func doInstall(cmd *cobra.Command, installer *Installer, state *InstallState) (err error) {
	// Track if we should clean up on failure
	var installStarted bool
	defer func() {
		if installStarted && err != nil {
			cmd.PrintErrln("Installation failed, cleaning up...")
			if cleanupErr := installer.Cleanup(cmd, state.EnableAutoUpdate); cleanupErr != nil {
				cmd.PrintErrf("Cleanup failed: %v\n", cleanupErr)
			}
		}
	}()

	cmd.PrintErrf("Service will run as user: %s\n", state.ServiceUser)

	// Mark that we've started installation
	installStarted = true

	// Perform the installation
	if err := installer.PerformInstallation(cmd, state); err != nil {
		return err
	}

	// Enable and start services
	if err := installer.EnableAndStartServices(cmd, state.EnableAutoUpdate); err != nil {
		return err
	}

	cmd.PrintErrln("\nInstallation complete!")
	cmd.PrintErrln("Check service status with: systemctl status piri")
	if state.EnableAutoUpdate {
		cmd.PrintErrln("Auto-update status: systemctl status piri-updater.timer")
	}

	return nil
}
