package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
)

// CommandExecutor interface for running system commands
type CommandExecutor interface {
	Output(name string, args ...string) ([]byte, error)
	Run(name string, args ...string) error
}

// realCommandExecutor executes real system commands
type realCommandExecutor struct{}

func (r *realCommandExecutor) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func (r *realCommandExecutor) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// ServiceManager handles all systemd service operations
type ServiceManager struct {
	// Services to manage
	Services []string
	// Command executor (can be mocked for testing)
	executor CommandExecutor
}

// NewServiceManager creates a new service manager
func NewServiceManager(services ...string) *ServiceManager {
	return NewServiceManagerWithExecutor(&realCommandExecutor{}, services...)
}

// NewServiceManagerWithExecutor creates a new service manager with custom executor
func NewServiceManagerWithExecutor(executor CommandExecutor, services ...string) *ServiceManager {
	return &ServiceManager{
		Services: services,
		executor: executor,
	}
}

// IsActive checks if a service is currently active
func (sm *ServiceManager) IsActive(service string) (bool, error) {
	output, err := sm.executor.Output("systemctl", "is-active", service)
	status := strings.TrimSpace(string(output))
	return status == "active", err
}

// CheckServicesNotRunning verifies that none of the services are running
func (sm *ServiceManager) CheckServicesNotRunning() error {
	var errs error
	for _, service := range sm.Services {
		active, _ := sm.IsActive(service)
		if active {
			errs = multierror.Append(errs, fmt.Errorf("service %s is running, please stop it first: sudo systemctl stop %s", service, service))
		}
	}
	return errs
}

// StopService stops a running service
func (sm *ServiceManager) StopService(service string) error {
	active, _ := sm.IsActive(service)
	if active {
		if err := sm.executor.Run("systemctl", "stop", service); err != nil {
			return fmt.Errorf("failed to stop service %s: %w", service, err)
		}
	}
	return nil
}

// StartService starts a service
func (sm *ServiceManager) StartService(service string) error {
	if err := sm.executor.Run("systemctl", "start", service); err != nil {
		return fmt.Errorf("failed to start service %s: %w", service, err)
	}
	return nil
}

// EnableService enables a service for auto-start
func (sm *ServiceManager) EnableService(service string) error {
	if err := sm.executor.Run("systemctl", "enable", service); err != nil {
		return fmt.Errorf("failed to enable service %s: %w", service, err)
	}
	return nil
}

// DisableService disables a service from auto-start
func (sm *ServiceManager) DisableService(service string) error {
	// The disable command will succeed even if the service doesn't exist
	if err := sm.executor.Run("systemctl", "disable", service); err != nil {
		// Only return error if it's not "service not found"
		if !strings.Contains(err.Error(), "exit status") {
			return fmt.Errorf("failed to disable service %s: %w", service, err)
		}
	}
	return nil
}

// StopAndDisableService stops a running service and disables it
func (sm *ServiceManager) StopAndDisableService(service string) error {
	var errs error

	if err := sm.StopService(service); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := sm.DisableService(service); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs
}

// EnableAndStartService enables and starts a service
func (sm *ServiceManager) EnableAndStartService(service string) error {
	if err := sm.EnableService(service); err != nil {
		return err
	}
	return sm.StartService(service)
}

// RestartService restarts a service
func (sm *ServiceManager) RestartService(service string) error {
	if err := sm.executor.Run("systemctl", "restart", service); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", service, err)
	}
	return nil
}

// RestartServiceWithSudo restarts a service using sudo
func (sm *ServiceManager) RestartServiceWithSudo(service string) error {
	if err := sm.executor.Run("sudo", "systemctl", "restart", service); err != nil {
		return fmt.Errorf("failed to restart service with sudo: %w", err)
	}
	return nil
}

// VerifyServiceRestart attempts to restart a service and verifies it started successfully
func (sm *ServiceManager) VerifyServiceRestart(service string, timeoutSec int, useSudo bool) error {
	// Restart the service
	if useSudo {
		if err := sm.RestartServiceWithSudo(service); err != nil {
			return err
		}
	} else {
		if err := sm.RestartService(service); err != nil {
			return err
		}
	}

	// Wait a bit for service to stabilize
	time.Sleep(2 * time.Second)

	// Check if service is active (with retries)
	for i := 0; i < timeoutSec; i++ {
		active, _ := sm.IsActive(service)

		if active {
			return nil // Success!
		}

		// Check for failure state
		output, _ := sm.executor.Output("systemctl", "is-failed", service)
		if strings.TrimSpace(string(output)) == "failed" {
			return fmt.Errorf("service failed to start")
		}

		// Service might still be activating
		time.Sleep(time.Second)
	}

	return fmt.Errorf("service did not become active within %d seconds", timeoutSec)
}

// ReloadDaemon reloads the systemd daemon configuration
func (sm *ServiceManager) ReloadDaemon() error {
	if err := sm.executor.Run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}
	return nil
}

// StopAllServices stops all managed services
func (sm *ServiceManager) StopAllServices() error {
	var errs error
	for _, service := range sm.Services {
		if err := sm.StopAndDisableService(service); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// ServiceStatus represents the status of a systemd service
type ServiceStatus struct {
	Name    string
	Active  bool
	Enabled bool
	Running bool
	Failed  bool
}

// GetServiceStatus returns detailed status of a service
func (sm *ServiceManager) GetServiceStatus(service string) (*ServiceStatus, error) {
	status := &ServiceStatus{Name: service}

	// Check if active
	active, _ := sm.IsActive(service)
	status.Active = active
	status.Running = active

	// Check if enabled
	output, _ := sm.executor.Output("systemctl", "is-enabled", service)
	status.Enabled = strings.TrimSpace(string(output)) == "enabled"

	// Check if failed
	output, _ = sm.executor.Output("systemctl", "is-failed", service)
	status.Failed = strings.TrimSpace(string(output)) == "failed"

	return status, nil
}

// InstallServiceFiles creates symlinks to the current version of service files
func (sm *ServiceManager) InstallServiceFiles(services []ServiceFile) error {
	for _, svc := range services {
		// The source path should now point through the current symlink
		currentServicePath := filepath.Join(PiriSystemdCurrentSymlink, filepath.Base(svc.SourcePath))
		symlinkPath := svc.TargetPath

		// Remove existing symlink if it exists
		_ = os.Remove(symlinkPath)

		// Create symlink in /etc/systemd/system/ pointing to current version
		if err := os.Symlink(currentServicePath, symlinkPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", svc.Name, err)
		}
	}

	// Reload systemd to pick up new service files
	return sm.ReloadDaemon()
}

// RemoveServiceFiles removes symlinks for systemd service files
func (sm *ServiceManager) RemoveServiceFiles(files []string) error {
	var errs error
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			errs = multierror.Append(errs, fmt.Errorf("failed to remove %s: %w", file, err))
		}
	}
	return errs
}

// ServiceFile represents a systemd service file to be installed
type ServiceFile struct {
	Name       string
	Content    string
	SourcePath string // Where to write the actual file
	TargetPath string // Where to create the symlink
}
