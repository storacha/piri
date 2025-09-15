package initalize

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
)

var InstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Piri as a system service",
	Long: `Install configures Piri to run as a systemd service on Linux systems.

This command performs the following operations:
  - Installs the piri binary to /usr/local/bin/piri
  - Creates a dedicated 'piri' system user to run the service
  - Creates the /etc/piri directory for configuration
  - Installs the provided configuration file
  - Creates and enables systemd service files
  - Optionally enables automatic updates (--enable-auto-update)

Requirements:
  - Linux operating system with systemd
  - Root privileges (run with sudo)
  - Configuration file from 'piri init' command

The --dry-run flag allows previewing the installation on any platform without
making changes, useful for testing and verification.

Automatic updates check for new releases every 30 minutes and apply them when
safe to do so (not during proof generation or active transfers).`,
	Args: cobra.NoArgs,
	RunE: runInstall,
}

func init() {
	// TODO config is a persistent flag, consider removal
	InstallCmd.Flags().String("config", "", "Path to configuration file (required)")
	InstallCmd.Flags().Bool("force", false, "Force overwrite existing files")
	InstallCmd.Flags().Bool("dry-run", false, "Preview installation without making changes")
	InstallCmd.Flags().Bool("enable-auto-update", false, "Enable automatic updates (checks every 30 minutes)")

	cobra.CheckErr(InstallCmd.MarkFlagRequired("config"))
	InstallCmd.SetOut(os.Stdout)
	InstallCmd.SetErr(os.Stderr)
}

// detectServiceUser determines which user should run the service
func detectServiceUser() string {
	// Check if running with sudo
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser
	}

	// Fall back to current user
	if currentUser := os.Getenv("USER"); currentUser != "" {
		return currentUser
	}

	// Last resort - use "root" if we can't detect
	return "root"
}

// setOwnership sets the ownership of a path to the specified user
func setOwnership(path string, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("looking up user %s: %w", username, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("parsing uid: %w", err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("parsing gid: %w", err)
	}

	// Walk the directory tree and set ownership on all files and directories
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}

// installState tracks what needs to be installed/checked
type installState struct {
	configPath       string
	config           config.FullServerConfig
	force            bool
	dryRun           bool
	enableAutoUpdate bool
	serviceUser      string // User that will run the service
}

// runInstall is the main entry point for the install command
func runInstall(cmd *cobra.Command, _ []string) error {
	state, err := parseInstallFlags(cmd)
	if err != nil {
		return err
	}

	if err := validateInstallation(cmd, state); err != nil {
		return err
	}

	return doInstall(cmd, state)
}

// parseInstallFlags parses command flags and loads the config
func parseInstallFlags(cmd *cobra.Command) (*installState, error) {
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, fmt.Errorf("reading --config flag: %w", err)
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return nil, fmt.Errorf("reading --force flag: %w", err)
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return nil, fmt.Errorf("reading --dry-run flag: %w", err)
	}

	enableAutoUpdate, err := cmd.Flags().GetBool("enable-auto-update")
	if err != nil {
		return nil, fmt.Errorf("reading --enable-auto-update flag: %w", err)
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

	// Detect the actual user (when running with sudo)
	serviceUser := detectServiceUser()

	return &installState{
		configPath:       configPath,
		config:           cfg,
		force:            force,
		dryRun:           dryRun,
		enableAutoUpdate: enableAutoUpdate,
		serviceUser:      serviceUser,
	}, nil
}

// validateInstallation checks prerequisites before installation
func validateInstallation(cmd *cobra.Command, state *installState) error {
	// Check platform - install only works on Linux with systemd (allow dry-run on any platform)
	if runtime.GOOS != "linux" && !state.dryRun {
		return fmt.Errorf("install command is only supported on Linux (systemd required). Current platform: %s. Use --dry-run to preview on other platforms", runtime.GOOS)
	}

	// Check root privileges (skip for dry-run)
	if !state.dryRun && !cliutil.IsRunningAsRoot() {
		return fmt.Errorf("install command requires root privileges. Re-run with `sudo`")
	}

	// Check if services are running (even with --force, we don't override running services)
	if err := checkServicesNotRunning(cmd, state.dryRun); err != nil {
		return err
	}

	// Check for existing files (unless --force)
	if !state.force {
		if err := checkExistingFiles(cmd, state.dryRun); err != nil {
			return err
		}
	}

	return nil
}

// checkServicesNotRunning verifies that piri services are not currently running
func checkServicesNotRunning(cmd *cobra.Command, dryRun bool) error {
	services := []string{cliutil.PiriServiceName, cliutil.PiriUpdateTimerName}

	for _, service := range services {
		if dryRun {
			cmd.PrintErrf("Would check if %s is running\n", service)
			continue
		}

		// Check if service is active
		output, err := exec.Command("systemctl", "is-active", service).Output()
		status := strings.TrimSpace(string(output))

		// systemctl is-active returns exit code 0 if active, non-zero otherwise
		if err == nil && status == "active" {
			cmd.PrintErrf("Error: Service %s is currently running\n", service)
			cmd.PrintErrf("Please stop it first: sudo systemctl stop %s\n", service)
			return fmt.Errorf("service %s is running", service)
		}
	}

	return nil
}

// checkExistingFiles checks if installation files already exist
func checkExistingFiles(cmd *cobra.Command, dryRun bool) error {
	filesToCheck := []string{
		cliutil.PiriSystemConfigPath,
		cliutil.PiriServiceFilePath,
		cliutil.PiriUpdateServiceFilePath,
		cliutil.PiriUpdateTimerServiceFilePath,
	}

	for _, file := range filesToCheck {
		if dryRun {
			cmd.PrintErrf("Would check if %s exists\n", file)
			continue
		}

		if _, err := os.Stat(file); err == nil {
			return fmt.Errorf("file already exists: %s (use --force to overwrite)", file)
		}
	}

	return nil
}

// doInstall performs the actual installation
func doInstall(cmd *cobra.Command, state *installState) (err error) {
	if state.dryRun {
		cmd.PrintErrln("DRY RUN MODE - No changes will be made")
		cmd.PrintErrln()
	}

	// Track if we should clean up on failure (not in dry-run)
	var installStarted bool
	defer func() {
		// If we started installation and encountered an error, cleanup
		if !state.dryRun && installStarted && err != nil {
			cmd.PrintErrln("Installation failed, cleaning up...")
			cleanupInstall()
		}
	}()

	cmd.PrintErrf("Service will run as user: %s\n", state.serviceUser)
	cmd.PrintErrln("Creating directory structure...")
	if !state.dryRun {
		// Mark that we've started installation and should cleanup on error
		installStarted = true

		// Create the /opt/piri directory structure
		if err := os.MkdirAll(cliutil.PiriBinaryDir, 0755); err != nil {
			return fmt.Errorf("failed to create binary directory %s: %w", cliutil.PiriBinaryDir, err)
		}
		if err := os.MkdirAll(cliutil.PiriSystemDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory %s: %w", cliutil.PiriSystemDir, err)
		}
		if err := os.MkdirAll(cliutil.PiriSystemdDir, 0755); err != nil {
			return fmt.Errorf("failed to create systemd directory %s: %w", cliutil.PiriSystemdDir, err)
		}
		cmd.PrintErrf("  Created directory structure under %s\n", cliutil.PiriOptDir)
	} else {
		cmd.PrintErrf("Would create directories:\n")
		cmd.PrintErrf("  - %s\n", cliutil.PiriBinaryDir)
		cmd.PrintErrf("  - %s\n", cliutil.PiriSystemDir)
		cmd.PrintErrf("  - %s\n", cliutil.PiriSystemdDir)
	}

	cmd.PrintErrln("Installing Piri binary...")
	if err := installBinary(cmd, state); err != nil {
		return err
	}

	// Set ownership of the entire /opt/piri tree to the service user
	if !state.dryRun {
		if err := setOwnership(cliutil.PiriOptDir, state.serviceUser); err != nil {
			return fmt.Errorf("failed to set ownership of %s: %w", cliutil.PiriOptDir, err)
		}
		cmd.PrintErrf("  Set ownership to %s\n", state.serviceUser)
	} else {
		cmd.PrintErrf("Would set ownership to: %s\n", state.serviceUser)
	}

	cmd.PrintErrln("Installing configuration...")
	if err := installConfig(cmd, state); err != nil {
		return err
	}
	cmd.PrintErrln("Installing systemd services...")
	if state.enableAutoUpdate {
		cmd.PrintErrln("  Including auto-update timer (checks every 30 minutes)")
	}
	if err := installSystemdServices(cmd, state); err != nil {
		return err
	}

	if !state.dryRun {
		cmd.PrintErrln("Enabling and starting services...")
		if err := enableAndStartServices(cmd, state.enableAutoUpdate); err != nil {
			return err
		}
	} else {
		cmd.PrintErrln("Would enable and start services:")
		cmd.PrintErrf("  - systemctl enable %s\n", cliutil.PiriServiceName)
		cmd.PrintErrf("  - systemctl start %s\n", cliutil.PiriServiceName)
		if state.enableAutoUpdate {
			cmd.PrintErrf("  - systemctl enable %s (auto-update)\n", cliutil.PiriUpdateTimerName)
			cmd.PrintErrf("  - systemctl start %s (auto-update)\n", cliutil.PiriUpdateTimerName)
		} else {
			cmd.PrintErrln("  - Auto-update: DISABLED (use --enable-auto-update to enable)")
		}
	}

	if state.dryRun {
		cmd.PrintErrln("\nDRY RUN COMPLETE - No changes were made")
	} else {
		cmd.PrintErrln("\nInstallation complete!")
		cmd.PrintErrln("Check service status with: systemctl status piri")
		if state.enableAutoUpdate {
			cmd.PrintErrln("Auto-update status: systemctl status piri-updater.timer")
		}
	}

	return nil
}

// installBinary installs the piri binary to the system location
func installBinary(cmd *cobra.Command, state *installState) error {
	exeBinPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}

	if state.dryRun {
		cmd.PrintErrf("Would install binary from %s to %s\n", exeBinPath, cliutil.PiriBinaryPath)
		return nil
	}

	// Check if we need to copy the binary
	if exeBinPath != cliutil.PiriBinaryPath {
		// Check if they're actually the same file (could be symlink or hardlink)
		srcInfo, srcErr := os.Stat(exeBinPath)
		dstInfo, dstErr := os.Stat(cliutil.PiriBinaryPath)

		// Only copy if destination doesn't exist or they're different files
		if dstErr != nil || (srcErr == nil && !os.SameFile(srcInfo, dstInfo)) {
			data, err := os.ReadFile(exeBinPath)
			if err != nil {
				return fmt.Errorf("failed to read piri executable: %w", err)
			}
			if err := os.WriteFile(cliutil.PiriBinaryPath, data, 0755); err != nil {
				return fmt.Errorf("failed to write piri executable: %w", err)
			}
			cmd.PrintErrf("  Installed binary to %s\n", cliutil.PiriBinaryPath)
		} else {
			cmd.PrintErrf("  Binary already at %s\n", cliutil.PiriBinaryPath)
		}
	}

	return nil
}

// installConfig installs the configuration file
func installConfig(cmd *cobra.Command, state *installState) error {
	cfgData, err := toml.Marshal(state.config)
	if err != nil {
		return fmt.Errorf("marshaling configuration: %w", err)
	}

	if state.dryRun {
		cmd.PrintErrf("Would write config to: %s\n", cliutil.PiriSystemConfigPath)
		cmd.PrintErrln("\n--- Configuration File ---")
		cmd.Print(string(cfgData))
		cmd.PrintErrln("--- End Configuration ---")
		return nil
	}

	if err := os.WriteFile(cliutil.PiriSystemConfigPath, cfgData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	cmd.PrintErrf("  Wrote config to %s\n", cliutil.PiriSystemConfigPath)
	return nil
}

// installSystemdServices creates and installs systemd service files
func installSystemdServices(cmd *cobra.Command, state *installState) error {
	// Generate service files
	// systemd timeout = fx shutdown timeout + buffer for process cleanup
	systemdTimeout := cliutil.PiriServerShutdownTimeout + cliutil.PiriSystemdShutdownBuffer
	piriService := GeneratePiriService(cliutil.PiriBinaryPath, cliutil.PiriServeCommand, state.serviceUser, systemdTimeout)
	updaterService := GeneratePiriUpdaterService(cliutil.PiriBinaryPath, cliutil.PiriUpdateCommand, state.serviceUser)
	updaterTimer := GeneratePiriUpdaterTimer(cliutil.PiriUpdateBootDuration, cliutil.PiriUpdateUnitActiveDuration, cliutil.PiriUpdateRandomizedDelayDuration)

	services := []struct {
		filename string
		content  string
	}{
		{"piri.service", piriService},
		{"piri-updater.service", updaterService},
		{"piri-updater.timer", updaterTimer},
	}

	// Write service files to /opt/piri/systemd/
	for _, svc := range services {
		servicePath := filepath.Join(cliutil.PiriSystemdDir, svc.filename)
		symlinkPath := filepath.Join(cliutil.SystemDPath, svc.filename)

		if state.dryRun {
			cmd.PrintErrf("Would write service file: %s\n", servicePath)
			cmd.PrintErrf("Would create symlink: %s -> %s\n", symlinkPath, servicePath)
			cmd.PrintErrf("\n--- %s ---\n", svc.filename)
			cmd.Print(svc.content)
			cmd.PrintErrf("--- End %s ---\n\n", svc.filename)
		} else {
			// Write the service file to /opt/piri/systemd/
			if err := os.WriteFile(servicePath, []byte(svc.content), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", svc.filename, err)
			}
			cmd.PrintErrf("  Wrote %s to %s\n", svc.filename, servicePath)

			// Create symlink in /etc/systemd/system/
			// Remove existing symlink if it exists (for --force)
			os.Remove(symlinkPath)
			if err := os.Symlink(servicePath, symlinkPath); err != nil {
				return fmt.Errorf("failed to create symlink for %s: %w", svc.filename, err)
			}
			cmd.PrintErrf("  Created symlink %s\n", symlinkPath)
		}
	}

	if !state.dryRun {
		// Reload systemd to pick up new service files
		if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
			return fmt.Errorf("failed to reload systemd daemon: %w", err)
		}
		cmd.PrintErrln("  Reloaded systemd daemon")
	}

	return nil
}

// enableAndStartServices enables and starts the systemd services
func enableAndStartServices(cmd *cobra.Command, enableAutoUpdate bool) error {
	// Enable main service
	if err := exec.Command("systemctl", "enable", cliutil.PiriServiceName).Run(); err != nil {
		return fmt.Errorf("failed to enable piri service: %w", err)
	}
	cmd.PrintErrf("  Enabled %s\n", cliutil.PiriServiceName)

	// Start main service
	if err := exec.Command("systemctl", "start", cliutil.PiriServiceName).Run(); err != nil {
		cmd.PrintErrf("Warning: Failed to start %s: %v\n", cliutil.PiriServiceName, err)
		cmd.PrintErrln("   You can start it manually with: sudo systemctl start piri")
		// Don't fail installation if service doesn't start
	} else {
		cmd.PrintErrf("  Started %s\n", cliutil.PiriServiceName)
	}

	// Only enable/start auto-update timer if requested
	if enableAutoUpdate {
		cmd.PrintErrln("  Enabling automatic updates...")

		if err := exec.Command("systemctl", "enable", cliutil.PiriUpdateTimerName).Run(); err != nil {
			return fmt.Errorf("failed to enable piri updater timer: %w", err)
		}
		cmd.PrintErrf("  Enabled %s\n", cliutil.PiriUpdateTimerName)

		if err := exec.Command("systemctl", "start", cliutil.PiriUpdateTimerName).Run(); err != nil {
			cmd.PrintErrf("Warning: Failed to start %s: %v\n", cliutil.PiriUpdateTimerName, err)
			cmd.PrintErrln("   You can start it manually with: sudo systemctl start piri-updater.timer")
			// Don't fail installation if timer doesn't start
		} else {
			cmd.PrintErrf("  Started %s\n", cliutil.PiriUpdateTimerName)
		}
	} else {
		cmd.PrintErrln("  Auto-update: DISABLED")
		cmd.PrintErrln("  To enable later: sudo systemctl enable --now piri-updater.timer")
	}

	return nil
}

// cleanupInstall attempts to rollback installation on failure
func cleanupInstall() {
	// Best effort cleanup - ignore errors since we're already in error state

	// Remove the entire installation
	os.RemoveAll(cliutil.PiriOptDir)

	// Clean up symlinks to avoid leaving broken links
	for _, service := range []string{"piri.service", "piri-updater.service", "piri-updater.timer"} {
		os.Remove(filepath.Join(cliutil.SystemDPath, service))
	}

	// Reload systemd to recognize services are gone
	exec.Command("systemctl", "daemon-reload").Run()
}
