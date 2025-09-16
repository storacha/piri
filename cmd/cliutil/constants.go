package cliutil

import (
	"path/filepath"
	"strings"
	"time"
)

// PiriServerShutdownTimeout is the duration in which we expect piri server to shut down gracefully
// This timeout allows fx to wait up to one minute for all lifecycle shutdown hooks to execute.
// Here is how we arrived at this value:
//  - Proving a piece typically takes 30 seconds, so a min is plenty there
//  - Assuming a symmetric 100Mbps connection, upload/download of 256 MB (max piece size) will likely take 20-30 seconds
//    therefore, 1 min should be enough time to let existing connections complete
// Still, if any of the above take more than 1min, they will be closed/rejected after 1min, so we might want
// to make this a configuration value based on user preference, metrics we collect, and capacity of the machine.
// this value must be in seconds, without fractions
const PiriServerShutdownTimeout = time.Minute

// PiriSystemdShutdownBuffer is additional time systemd waits after fx shutdown timeout
// This buffer accounts for process cleanup overhead after fx completes its shutdown sequence.
// The total systemd timeout will be PiriServerShutdownTimeout + PiriSystemdShutdownBuffer
const PiriSystemdShutdownBuffer = 15 * time.Second

// PiriOptDir is the base directory for piri installation
const PiriOptDir = "/opt/piri"

// PiriBinaryBaseDir is the base directory for all versioned binaries
const PiriBinaryBaseDir = "/opt/piri/bin"

// PiriCurrentSymlink is the symlink that points to the current version
const PiriCurrentSymlink = "/opt/piri/bin/current"

// PiriBinaryPath is the path to the current piri binary via symlink
const PiriBinaryPath = "/opt/piri/bin/current/piri"

// PiriSystemDir is the system configuration directory for piri (not versioned)
const PiriSystemDir = "/opt/piri/etc"

// PiriSystemdDir is the directory where piri's systemd service files are stored (not versioned)
const PiriSystemdDir = "/opt/piri/systemd"

// GetVersionedBinaryDir returns the versioned directory for a specific piri binary
func GetVersionedBinaryDir(version string) string {
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

// PiriCLISymlinkPath is the location of the piri symlink in PATH for CLI access
const PiriCLISymlinkPath = "/usr/local/bin/piri"

// PiriServeCommand is the command to start the server (without config flag)
const PiriServeCommand = "serve full"

// PiriUpdateCommand is the internal update command
const PiriUpdateCommand = "update-internal"

// SystemDPath is the directory where systemd service files are installed as symlinks
const SystemDPath = "/etc/systemd/system"

// PiriServiceFile is the filename for the main piri systemd service
const PiriServiceFile = "piri.service"

// PiriServiceFilePath is the full path to the main piri systemd service file
var PiriServiceFilePath = filepath.Join(SystemDPath, PiriServiceFile)

// PiriUpdateServiceFile is the filename for the auto-update systemd service
const PiriUpdateServiceFile = "piri-updater.service"

// PiriUpdateServiceFilePath is the full path to the auto-update service file
var PiriUpdateServiceFilePath = filepath.Join(SystemDPath, PiriUpdateServiceFile)

// PiriUpdateTimerServiceFile is the filename for the auto-update timer that triggers updates
const PiriUpdateTimerServiceFile = "piri-updater.timer"

// PiriUpdateTimerServiceFilePath is the full path to the auto-update timer file
var PiriUpdateTimerServiceFilePath = filepath.Join(SystemDPath, PiriUpdateTimerServiceFile)

// Systemd service names (without .service extension) for systemctl commands
const PiriServiceName = "piri"
const PiriUpdateTimerName = "piri-updater.timer"
const PiriUpdateServiceName = "piri-updater"

// PiriConfigFileName is the default config file name created by init command
const PiriConfigFileName = "piri-config.toml"

// PiriSystemConfigPath is the installed config location
var PiriSystemConfigPath = filepath.Join(PiriSystemDir, PiriConfigFileName)

// ReleaseURL is the GitHub API endpoint for checking latest piri releases
const ReleaseURL = "https://api.github.com/repos/storacha/piri/releases/latest"

// PiriUpdateBootDuration is the delay after system boot before first update check (2 minutes)
const PiriUpdateBootDuration = 2 * time.Minute

// PiriUpdateUnitActiveDuration is the interval between automatic update checks (30 minutes)
const PiriUpdateUnitActiveDuration = 30 * time.Minute

// PiriUpdateRandomizedDelayDuration adds random delay to prevent simultaneous updates across fleet (5 minutes)
const PiriUpdateRandomizedDelayDuration = 5 * time.Minute

// PiriSudoersFile is the sudoers configuration for auto-updater to restart piri service
const PiriSudoersFile = "/etc/sudoers.d/piri-updater"
