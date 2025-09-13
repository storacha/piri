package cliutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"syscall"
)

// IsRunningAsRoot checks if the current process is running as root
func IsRunningAsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return currentUser.Uid == "0"
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
