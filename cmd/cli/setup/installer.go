package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/config"
)

// Installer handles the installation and update of Piri
type Installer struct {
	FileSystem     *FileSystemManager
	ServiceManager *ServiceManager
	Platform       *PlatformChecker
}

// NewInstaller creates a new installer instance
func NewInstaller() (*Installer, error) {
	platform, err := NewPlatformChecker()
	if err != nil {
		return nil, err
	}

	return &Installer{
		FileSystem:     NewFileSystemManager(),
		ServiceManager: NewServiceManager(PiriServiceName, PiriUpdateTimerName),
		Platform:       platform,
	}, nil
}

// InstallState holds the state for installation operations
type InstallState struct {
	ConfigPath       string
	Config           config.FullServerConfig
	Force            bool
	EnableAutoUpdate bool
	ServiceUser      string
	Version          string
	IsUpdate         bool // true if this is an update operation
}

// InstallBinary installs the piri binary to the appropriate location
func (i *Installer) InstallBinary(cmd *cobra.Command, version string) error {
	exeBinPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}

	// Get the versioned binary path
	versionedBinDir := getVersionedBinaryDir(version)
	versionedBinPath := filepath.Join(versionedBinDir, "piri")

	// Copy the binary to the versioned directory
	if err := i.FileSystem.CopyFile(exeBinPath, versionedBinPath, 0755); err != nil {
		return err
	}

	cmd.PrintErrf("  Installed binary to %s\n", versionedBinPath)

	// Create or update the "current" symlink
	if err := i.FileSystem.CreateSymlink(versionedBinDir, PiriCurrentSymlink); err != nil {
		return err
	}

	cmd.PrintErrf("  Created symlink %s -> %s\n", PiriCurrentSymlink, versionedBinDir)
	return nil
}

// InstallConfiguration installs the configuration file
func (i *Installer) InstallConfiguration(cmd *cobra.Command, cfg config.FullServerConfig) error {
	cfgData, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling configuration: %w", err)
	}

	if err := i.FileSystem.WriteFile(PiriSystemConfigPath, cfgData, 0644); err != nil {
		return err
	}

	cmd.PrintErrf("  Wrote config to %s\n", PiriSystemConfigPath)
	return nil
}

// GenerateSystemdServices generates systemd service configurations
func (i *Installer) GenerateSystemdServices(serviceUser string) []ServiceFile {
	return []ServiceFile{
		{
			Name:       PiriServiceFile,
			Content:    i.generatePiriService(serviceUser),
			SourcePath: filepath.Join(PiriSystemdDir, PiriServiceFile),
			TargetPath: filepath.Join(SystemDPath, PiriServiceFile),
		},
		{
			Name:       PiriUpdateServiceFile,
			Content:    i.generatePiriUpdaterService(serviceUser),
			SourcePath: filepath.Join(PiriSystemdDir, PiriUpdateServiceFile),
			TargetPath: filepath.Join(SystemDPath, PiriUpdateServiceFile),
		},
		{
			Name:       PiriUpdateTimerServiceFile,
			Content:    i.generatePiriUpdaterTimer(),
			SourcePath: filepath.Join(PiriSystemdDir, PiriUpdateTimerServiceFile),
			TargetPath: filepath.Join(SystemDPath, PiriUpdateTimerServiceFile),
		},
	}
}

// InstallSystemdServices installs systemd service files
func (i *Installer) InstallSystemdServices(cmd *cobra.Command, state *InstallState) error {
	cmd.PrintErrln("Installing systemd services...")
	if state.EnableAutoUpdate {
		cmd.PrintErrln("  Including auto-update timer (checks every 30 minutes)")
	}

	services := i.GenerateSystemdServices(state.ServiceUser)
	if err := i.ServiceManager.InstallServiceFiles(services); err != nil {
		return err
	}

	cmd.PrintErrln("  Installed systemd service files")
	cmd.PrintErrln("  Reloaded systemd daemon")
	return nil
}

// CreateSymlink creates the symlink in /usr/local/bin
func (i *Installer) CreateSymlink(cmd *cobra.Command) error {
	cmd.PrintErrln("Creating symlink...")

	// Ensure the directory exists
	symlinkDir := filepath.Dir(PiriCLISymlinkPath)
	if err := i.FileSystem.CreateDirectory(symlinkDir, 0755); err != nil {
		return err
	}

	// Create the symlink
	if err := i.FileSystem.CreateSymlink(PiriBinaryPath, PiriCLISymlinkPath); err != nil {
		// Only warn if we can't create the symlink
		cmd.PrintErrf("  Warning: Could not create symlink: %v\n", err)
		return nil
	}

	cmd.PrintErrf("  Created symlink %s -> %s\n", PiriCLISymlinkPath, PiriBinaryPath)
	return nil
}

// CreateSudoersEntry creates the sudoers entry for service management
func (i *Installer) CreateSudoersEntry(cmd *cobra.Command, serviceUser string, enableAutoUpdate bool) error {
	cmd.PrintErrln("Creating sudoers entry for service management...")

	// Create minimal sudoers rule - ONLY allows restart of piri service
	sudoersContent := fmt.Sprintf("%s ALL=(root) NOPASSWD: /usr/bin/systemctl restart piri\n", serviceUser)

	if err := i.FileSystem.WriteFile(PiriSudoersFile, []byte(sudoersContent), 0440); err != nil {
		return fmt.Errorf("failed to create sudoers file: %w", err)
	}

	cmd.PrintErrf("  Created minimal sudoers entry for piri service restart only\n")
	if !enableAutoUpdate {
		cmd.PrintErrf("  Note: Auto-update is disabled, but can be enabled later with:\n")
		cmd.PrintErrf("        sudo systemctl enable --now piri-updater.timer\n")
	}

	return nil
}

// PerformInstallation performs a complete installation
func (i *Installer) PerformInstallation(cmd *cobra.Command, state *InstallState) error {
	// Create directory structure
	cmd.PrintErrln("Creating directory structure...")
	if err := i.FileSystem.CreatePiriDirectoryStructure(state.Version); err != nil {
		return err
	}
	cmd.PrintErrf("  Created directory structure under %s\n", PiriOptDir)

	// Install binary
	cmd.PrintErrln("Installing Piri binary...")
	if err := i.InstallBinary(cmd, state.Version); err != nil {
		return err
	}

	// Install configuration (if not an update)
	if !state.IsUpdate {
		cmd.PrintErrln("Installing configuration...")
		if err := i.InstallConfiguration(cmd, state.Config); err != nil {
			return err
		}
	}

	// Install systemd services
	if err := i.InstallSystemdServices(cmd, state); err != nil {
		return err
	}

	// Set ownership
	cmd.PrintErrln("Setting ownership...")
	if err := i.FileSystem.SetOwnership(PiriOptDir, state.ServiceUser); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}
	cmd.PrintErrf("  Set ownership of %s to %s\n", PiriOptDir, state.ServiceUser)

	// Create sudoers entry, allowing the auto updater to restart piri process.
	if err := i.CreateSudoersEntry(cmd, state.ServiceUser, state.EnableAutoUpdate); err != nil {
		return err
	}

	// Create symlink
	if err := i.CreateSymlink(cmd); err != nil {
		return err
	}

	return nil
}

// EnableAndStartServices enables and starts the appropriate services
func (i *Installer) EnableAndStartServices(cmd *cobra.Command, enableAutoUpdate bool) error {
	cmd.PrintErrln("Enabling and starting services...")

	// Enable and start main service
	if err := i.ServiceManager.EnableAndStartService(PiriServiceName); err != nil {
		return fmt.Errorf("failed to enable/start piri service: %w", err)
	}
	cmd.PrintErrf("  Enabled and started %s\n", PiriServiceName)

	// Handle auto-update timer
	if enableAutoUpdate {
		cmd.PrintErrln("  Enabling automatic updates...")
		if err := i.ServiceManager.EnableAndStartService(PiriUpdateTimerName); err != nil {
			return fmt.Errorf("failed to enable/start updater timer: %w", err)
		}
		cmd.PrintErrf("  Enabled and started %s\n", PiriUpdateTimerName)
	} else {
		cmd.PrintErrln("  Auto-update: DISABLED")
		cmd.PrintErrln("  To enable later: sudo systemctl enable --now piri-updater.timer")
	}

	return nil
}

// Uninstall performs a complete uninstallation
func (i *Installer) Uninstall(cmd *cobra.Command) error {
	// Stop all services
	cmd.PrintErrln("Stopping services...")
	if err := i.ServiceManager.StopAllServices(); err != nil {
		cmd.PrintErrf("Warning: Failed to stop some services: %v\n", err)
	}

	// Remove service files
	cmd.PrintErrln("Removing service files...")
	serviceFiles := []string{
		filepath.Join(SystemDPath, PiriServiceFile),
		filepath.Join(SystemDPath, PiriUpdateServiceFile),
		filepath.Join(SystemDPath, PiriUpdateTimerServiceFile),
	}
	if err := i.ServiceManager.RemoveServiceFiles(serviceFiles); err != nil {
		cmd.PrintErrf("Warning: Failed to remove some service files: %v\n", err)
	}

	// Remove CLI symlink
	cmd.PrintErrln("Removing CLI symlink...")
	if err := i.FileSystem.RemoveFiles([]string{PiriCLISymlinkPath}); err != nil {
		cmd.PrintErrf("Warning: Failed to remove CLI symlink: %v\n", err)
	}

	// Remove sudoers file
	cmd.PrintErrln("Removing sudoers configuration...")
	if err := i.FileSystem.RemoveFiles([]string{PiriSudoersFile}); err != nil {
		cmd.PrintErrf("Warning: Failed to remove sudoers file: %v\n", err)
	}

	// Reload systemd
	if err := i.ServiceManager.ReloadDaemon(); err != nil {
		cmd.PrintErrf("Warning: Failed to reload systemd: %v\n", err)
	}

	return nil
}

// Service generation functions (moved from util_install.go)
func (i *Installer) generatePiriService(serviceUser string) string {
	return fmt.Sprintf(
		`[Unit]
Description=Piri Storage Node Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s %s
TimeoutStopSec=%d
KillMode=mixed
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`,
		serviceUser,
		serviceUser,
		PiriSystemDir,
		PiriBinaryPath,
		PiriServeCommand,
		int((PiriServerShutdownTimeout + PiriSystemdShutdownBuffer).Seconds()),
	)
}

func (i *Installer) generatePiriUpdaterService(serviceUser string) string {
	return fmt.Sprintf(`[Unit]
Description=Piri Auto-Update Service
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s %s
StandardOutput=journal
StandardError=journal
`, serviceUser, serviceUser, PiriSystemDir, PiriBinaryPath, PiriUpdateCommand)
}

func (i *Installer) generatePiriUpdaterTimer() string {
	return fmt.Sprintf(`[Unit]
Description=Piri Auto-Update Timer
Requires=piri-updater.service

[Timer]
OnBootSec=%s
OnUnitActiveSec=%s
RandomizedDelaySec=%s
Persistent=true

[Install]
WantedBy=timers.target
`, PiriUpdateBootDuration, PiriUpdateUnitActiveDuration, PiriUpdateRandomizedDelayDuration)
}

// Cleanup performs cleanup on installation failure
func (i *Installer) Cleanup(cmd *cobra.Command, enableAutoUpdate bool) error {
	cmd.PrintErrln("Cleaning up after failed installation...")

	// Determine which services to stop based on installation state
	services := []string{PiriServiceName}
	if enableAutoUpdate {
		services = append(services, PiriUpdateTimerName)
	}

	// Stop services
	sm := NewServiceManager(services...)
	if err := sm.StopAllServices(); err != nil {
		cmd.PrintErrf("Cleanup warning: Failed to stop services: %v\n", err)
	}

	// Rollback file system changes
	if err := i.FileSystem.Rollback(); err != nil {
		cmd.PrintErrf("Cleanup warning: Failed to rollback files: %v\n", err)
	}

	return nil
}
