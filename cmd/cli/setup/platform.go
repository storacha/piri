package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// PlatformChecker provides platform and permission checking utilities
type PlatformChecker struct {
	OS           string
	IsLinux      bool
	IsDarwin     bool
	HasSystemd   bool
	IsRoot       bool
	ServiceUser  string
}

// NewPlatformChecker creates and initializes a platform checker
func NewPlatformChecker() (*PlatformChecker, error) {
	pc := &PlatformChecker{
		OS:       runtime.GOOS,
		IsLinux:  runtime.GOOS == "linux",
		IsDarwin: runtime.GOOS == "darwin",
	}

	// Check for systemd
	pc.HasSystemd = pc.checkSystemd()

	// Check if running as root
	pc.IsRoot = pc.isRunningAsRoot()

	// Detect service user
	user, err := pc.detectServiceUser()
	if err != nil && pc.IsLinux {
		// Only fail on Linux where we need a service user
		return nil, err
	}
	pc.ServiceUser = user

	return pc, nil
}

// checkSystemd checks if systemd is available
func (pc *PlatformChecker) checkSystemd() bool {
	if !pc.IsLinux {
		return false
	}

	// Check if systemctl exists
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}

	// Check if we can query systemd
	if err := exec.Command("systemctl", "--version").Run(); err != nil {
		return false
	}

	return true
}

// isRunningAsRoot checks if the current process is running as root
func (pc *PlatformChecker) isRunningAsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return currentUser.Uid == "0"
}

// detectServiceUser determines which user should run the service
func (pc *PlatformChecker) detectServiceUser() (string, error) {
	// Check if running with sudo
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser, nil
	}

	// Fall back to current user
	if currentUser := os.Getenv("USER"); currentUser != "" {
		return currentUser, nil
	}

	// Try to get current user from system
	u, err := user.Current()
	if err == nil && u.Username != "" {
		return u.Username, nil
	}

	return "", errors.New("could not determine service user")
}

// RequireLinux returns an error if not running on Linux
func (pc *PlatformChecker) RequireLinux() error {
	if !pc.IsLinux {
		return fmt.Errorf("this command is only supported on Linux (systemd required). Current platform: %s", pc.OS)
	}
	return nil
}

// RequireSystemd returns an error if systemd is not available
func (pc *PlatformChecker) RequireSystemd() error {
	if err := pc.RequireLinux(); err != nil {
		return err
	}
	if !pc.HasSystemd {
		return fmt.Errorf("systemd is required but not available")
	}
	return nil
}

// RequireRoot returns an error if not running as root
func (pc *PlatformChecker) RequireRoot() error {
	if !pc.IsRoot {
		return fmt.Errorf("this command requires root privileges. Re-run with `sudo`")
	}
	return nil
}

// IsRunningUnderSystemd checks if the current process is running under systemd
func (pc *PlatformChecker) IsRunningUnderSystemd() bool {
	// Check if INVOCATION_ID is set - systemd sets this for all services
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	// Also check for SYSTEMD_EXEC_PID which is set in newer systemd versions
	if os.Getenv("SYSTEMD_EXEC_PID") != "" {
		return true
	}
	// Check if piri service exists
	if pc.HasSystemd {
		if err := exec.Command("systemctl", "list-units", "--full", "--all", "--plain", "--no-pager", "piri.service").Run(); err == nil {
			return true
		}
	}
	return false
}

// IsManagedInstallation checks if the current installation is managed (in /opt/piri)
func (pc *PlatformChecker) IsManagedInstallation(execPath string) bool {
	return strings.HasPrefix(execPath, PiriOptDir)
}

// GetExecutablePath returns the path to the current executable, resolved of any symlinks
func GetExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	return execPath, nil
}

// RunWithSudo re-executes the current command with sudo
func RunWithSudo() error {
	// Get the original command arguments
	args := os.Args

	// Build the sudo command
	var sudoArgs []string

	// Preserve environment variables that might be needed
	sudoArgs = append(sudoArgs, "-E")    // Preserve environment
	sudoArgs = append(sudoArgs, "--")    // End of sudo options
	sudoArgs = append(sudoArgs, args...) // Original command and arguments

	// Create the sudo command
	sudoCmd := exec.Command("sudo", sudoArgs...)
	sudoCmd.Stdin = os.Stdin
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr

	// Run the command and wait for it to complete
	err := sudoCmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// If sudo was cancelled or failed, return a user-friendly error
			if exitErr.ExitCode() == 1 {
				return fmt.Errorf("command cancelled or authentication failed")
			}
			// Propagate the exit code
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		return fmt.Errorf("failed to run with sudo: %w", err)
	}

	// If sudo succeeded, exit with success
	os.Exit(0)
	return nil
}

// CheckPrerequisites performs common prerequisite checks for installation/update
type Prerequisites struct {
	Platform        *PlatformChecker
	NeedsRoot       bool
	NeedsSystemd    bool
	CheckServices   []string // Services that should not be running
	CheckFiles      []string // Files that should not exist (unless --force)
}

// Validate checks all prerequisites and returns any errors
func (pr *Prerequisites) Validate(force bool) error {
	// Platform checks
	if pr.NeedsSystemd {
		if err := pr.Platform.RequireSystemd(); err != nil {
			return err
		}
	} else if pr.Platform != nil && pr.Platform.IsLinux {
		if err := pr.Platform.RequireLinux(); err != nil {
			return err
		}
	}

	// Root check
	if pr.NeedsRoot {
		if err := pr.Platform.RequireRoot(); err != nil {
			return err
		}
	}

	// Service checks
	if len(pr.CheckServices) > 0 {
		sm := NewServiceManager(pr.CheckServices...)
		if err := sm.CheckServicesNotRunning(); err != nil {
			return err
		}
	}

	// File checks (unless --force)
	if !force && len(pr.CheckFiles) > 0 {
		fsm := NewFileSystemManager()
		if err := fsm.CheckExistingFiles(pr.CheckFiles); err != nil {
			return fmt.Errorf("existing files found (use --force to overwrite): %w", err)
		}
	}

	return nil
}