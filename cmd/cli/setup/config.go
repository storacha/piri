package setup

import (
	"path/filepath"
	"strings"
	"time"
)

// PathConfig holds all path-related configuration
type PathConfig struct {
	OptDir                string        // Base directory for piri installation
	BinaryBaseDir         string        // Base directory for all versioned binaries
	CurrentSymlink        string        // Symlink that points to the current version
	BinaryPath            string        // Path to the current piri binary via symlink
	SystemDir             string        // System configuration directory (not versioned)
	SystemdBaseDir        string        // Base directory for versioned systemd files
	SystemdCurrentSymlink string        // Symlink to current systemd version
	CLISymlinkPath        string        // Location of piri symlink in PATH
	SystemdPath           string        // /etc/systemd/system - where systemd reads from
	SudoersFile           string        // Sudoers configuration file path
	ConfigFileName        string        // Default config file name
	ServerShutdown        time.Duration // Server shutdown timeout
	SystemdBuffer         time.Duration // Additional systemd shutdown buffer
}

// DefaultPathConfig returns the default path configuration
func DefaultPathConfig() *PathConfig {
	return &PathConfig{
		OptDir:                "/opt/piri",
		BinaryBaseDir:         "/opt/piri/bin",
		CurrentSymlink:        "/opt/piri/bin/current",
		BinaryPath:            "/opt/piri/bin/current/piri",
		SystemDir:             "/opt/piri/etc",
		SystemdBaseDir:        "/opt/piri/systemd",
		SystemdCurrentSymlink: "/opt/piri/systemd/current",
		CLISymlinkPath:        "/usr/local/bin/piri",
		SystemdPath:           "/etc/systemd/system",
		SudoersFile:           "/etc/sudoers.d/piri-updater",
		ConfigFileName:        "piri-config.toml",
		// PiriServerShutdownTimeout is the duration in which we expect piri server to shut down gracefully
		// This timeout allows fx to wait up to one minute for all lifecycle shutdown hooks to execute.
		ServerShutdown: time.Minute,
		// PiriSystemdShutdownBuffer is additional time systemd waits after fx shutdown timeout
		SystemdBuffer: 15 * time.Second,
	}
}

// Global default config (can be overridden in tests)
var Config = DefaultPathConfig()

// Compatibility constants for existing code
var (
	PiriServerShutdownTimeout = Config.ServerShutdown
	PiriSystemdShutdownBuffer = Config.SystemdBuffer
	PiriOptDir                = Config.OptDir
	PiriBinaryBaseDir         = Config.BinaryBaseDir
	PiriCurrentSymlink        = Config.CurrentSymlink
	PiriBinaryPath            = Config.BinaryPath
	PiriSystemDir             = Config.SystemDir
	PiriSystemdBaseDir        = Config.SystemdBaseDir
	PiriSystemdCurrentSymlink = Config.SystemdCurrentSymlink
	PiriSystemdDir            = Config.SystemdCurrentSymlink // Points to current version for backward compat
)

// getVersionedBinaryDir returns the versioned directory for a specific piri binary
func getVersionedBinaryDir(version string) string {
	// Clean version string - remove any commit hash suffixes
	cleanVersion := version
	if idx := strings.Index(version, "-"); idx != -1 {
		cleanVersion = version[:idx]
	}
	// Ensure version starts with 'v'
	if !strings.HasPrefix(cleanVersion, "v") {
		cleanVersion = "v" + cleanVersion
	}
	return filepath.Join(PiriBinaryBaseDir, cleanVersion)
}

// getVersionedSystemdDir returns the versioned directory for systemd service files
func getVersionedSystemdDir(version string) string {
	// Clean version string - remove any commit hash suffixes
	cleanVersion := version
	if idx := strings.Index(version, "-"); idx != -1 {
		cleanVersion = version[:idx]
	}
	// Ensure version starts with 'v'
	if !strings.HasPrefix(cleanVersion, "v") {
		cleanVersion = "v" + cleanVersion
	}
	return filepath.Join(PiriSystemdBaseDir, cleanVersion)
}

// More compatibility constants
var (
	PiriCLISymlinkPath             = Config.CLISymlinkPath
	SystemDPath                    = Config.SystemdPath
	PiriSudoersFile                = Config.SudoersFile
	PiriConfigFileName             = Config.ConfigFileName
	PiriSystemConfigPath           = filepath.Join(Config.SystemDir, Config.ConfigFileName)
	PiriServiceFilePath            = filepath.Join(Config.SystemdPath, PiriServiceFile)
	PiriUpdateServiceFilePath      = filepath.Join(Config.SystemdPath, PiriUpdateServiceFile)
	PiriUpdateTimerServiceFilePath = filepath.Join(Config.SystemdPath, PiriUpdateTimerServiceFile)
)

// Service-related constants
const (
	PiriServeCommand                  = "serve full"
	PiriUpdateCommand                 = "update-internal"
	PiriServiceFile                   = "piri.service"
	PiriUpdateServiceFile             = "piri-updater.service"
	PiriUpdateTimerServiceFile        = "piri-updater.timer"
	PiriServiceName                   = "piri"
	PiriUpdateTimerName               = "piri-updater.timer"
	ReleaseURL                        = "https://api.github.com/repos/storacha/piri/releases/latest"
	PiriUpdateBootDuration            = 2 * time.Minute
	PiriUpdateUnitActiveDuration      = 30 * time.Minute
	PiriUpdateRandomizedDelayDuration = 5 * time.Minute
)
