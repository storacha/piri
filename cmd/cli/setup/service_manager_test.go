package setup

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// MockCommandExecutor mocks system command execution
type MockCommandExecutor struct {
	outputs map[string]string
	errors  map[string]error
	calls   []string // Track what commands were called
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
		calls:   make([]string, 0),
	}
}

func (m *MockCommandExecutor) Output(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, key)

	if err, exists := m.errors[key]; exists && err != nil {
		return nil, err
	}

	if output, exists := m.outputs[key]; exists {
		return []byte(output), nil
	}

	return []byte(""), nil
}

func (m *MockCommandExecutor) Run(name string, args ...string) error {
	key := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, key)

	if err, exists := m.errors[key]; exists {
		return err
	}

	return nil
}

func TestServiceManager_IsActive(t *testing.T) {
	tests := []struct {
		name           string
		service        string
		commandOutput  string
		commandError   error
		expectedActive bool
	}{
		{
			name:           "service is active",
			service:        "piri",
			commandOutput:  "active",
			expectedActive: true,
		},
		{
			name:           "service is inactive",
			service:        "piri",
			commandOutput:  "inactive",
			expectedActive: false,
		},
		{
			name:           "service is failed",
			service:        "piri",
			commandOutput:  "failed",
			expectedActive: false,
		},
		{
			name:           "service is activating",
			service:        "piri",
			commandOutput:  "activating",
			expectedActive: false,
		},
		{
			name:           "service not found",
			service:        "nonexistent",
			commandOutput:  "",
			commandError:   errors.New("exit status 4"),
			expectedActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := NewMockCommandExecutor()
			mockExec.outputs["systemctl is-active "+tt.service] = tt.commandOutput
			if tt.commandError != nil {
				mockExec.errors["systemctl is-active "+tt.service] = tt.commandError
			}

			sm := NewServiceManagerWithExecutor(mockExec, tt.service)
			active, _ := sm.IsActive(tt.service)
			require.Equal(t, tt.expectedActive, active)

			// Verify the correct command was called
			require.Contains(t, mockExec.calls, "systemctl is-active "+tt.service)
		})
	}
}

func TestServiceManager_StartService(t *testing.T) {
	tests := []struct {
		name        string
		service     string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "start service successfully",
			service:     "piri",
			shouldError: false,
		},
		{
			name:        "start service fails",
			service:     "piri",
			shouldError: true,
			errorMsg:    "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := NewMockCommandExecutor()
			if tt.shouldError {
				mockExec.errors["systemctl start "+tt.service] = fmt.Errorf("%s", tt.errorMsg)
			}

			sm := NewServiceManagerWithExecutor(mockExec, tt.service)
			err := sm.StartService(tt.service)

			if tt.shouldError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.service)
			} else {
				require.NoError(t, err)
			}

			// Verify the correct command was called
			require.Contains(t, mockExec.calls, "systemctl start "+tt.service)
		})
	}
}

func TestServiceManager_StopService(t *testing.T) {
	mockExec := NewMockCommandExecutor()

	// Service is active, should stop it
	mockExec.outputs["systemctl is-active piri"] = "active"

	sm := NewServiceManagerWithExecutor(mockExec, "piri")
	err := sm.StopService("piri")
	require.NoError(t, err)

	// Verify both commands were called
	require.Contains(t, mockExec.calls, "systemctl is-active piri")
	require.Contains(t, mockExec.calls, "systemctl stop piri")
}

func TestServiceManager_StopService_NotActive(t *testing.T) {
	mockExec := NewMockCommandExecutor()

	// Service is not active, should not try to stop it
	mockExec.outputs["systemctl is-active piri"] = "inactive"

	sm := NewServiceManagerWithExecutor(mockExec, "piri")
	err := sm.StopService("piri")
	require.NoError(t, err)

	// Verify only the is-active check was called
	require.Contains(t, mockExec.calls, "systemctl is-active piri")
	require.NotContains(t, mockExec.calls, "systemctl stop piri")
}

func TestServiceManager_EnableAndDisableService(t *testing.T) {
	mockExec := NewMockCommandExecutor()
	sm := NewServiceManagerWithExecutor(mockExec, "piri")

	// Test enable
	err := sm.EnableService("piri")
	require.NoError(t, err)
	require.Contains(t, mockExec.calls, "systemctl enable piri")

	// Test disable
	err = sm.DisableService("piri")
	require.NoError(t, err)
	require.Contains(t, mockExec.calls, "systemctl disable piri")
}

func TestServiceManager_RestartService(t *testing.T) {
	tests := []struct {
		name     string
		useSudo  bool
		expected string
	}{
		{
			name:     "restart without sudo",
			useSudo:  false,
			expected: "systemctl restart piri",
		},
		{
			name:     "restart with sudo",
			useSudo:  true,
			expected: "sudo systemctl restart piri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := NewMockCommandExecutor()
			sm := NewServiceManagerWithExecutor(mockExec, "piri")

			var err error
			if tt.useSudo {
				err = sm.RestartServiceWithSudo("piri")
			} else {
				err = sm.RestartService("piri")
			}

			require.NoError(t, err)
			require.Contains(t, mockExec.calls, tt.expected)
		})
	}
}

func TestServiceManager_VerifyServiceRestart(t *testing.T) {
	mockExec := NewMockCommandExecutor()

	// Service becomes active after restart
	mockExec.outputs["systemctl is-active piri"] = "active"

	sm := NewServiceManagerWithExecutor(mockExec, "piri")
	err := sm.VerifyServiceRestart("piri", 5, false)
	require.NoError(t, err)

	// Verify restart and status check were called
	require.Contains(t, mockExec.calls, "systemctl restart piri")
	require.Contains(t, mockExec.calls, "systemctl is-active piri")
}

func TestServiceManager_VerifyServiceRestart_Failed(t *testing.T) {
	mockExec := NewMockCommandExecutor()

	// Service fails to start
	mockExec.outputs["systemctl is-active piri"] = "inactive"
	mockExec.outputs["systemctl is-failed piri"] = "failed"

	sm := NewServiceManagerWithExecutor(mockExec, "piri")
	err := sm.VerifyServiceRestart("piri", 1, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to start")
}

func TestServiceManager_GetServiceStatus(t *testing.T) {
	mockExec := NewMockCommandExecutor()

	// Set up various status outputs
	mockExec.outputs["systemctl is-active piri"] = "active"
	mockExec.outputs["systemctl is-enabled piri"] = "enabled"
	mockExec.outputs["systemctl is-failed piri"] = "inactive"

	sm := NewServiceManagerWithExecutor(mockExec, "piri")
	status, err := sm.GetServiceStatus("piri")
	require.NoError(t, err)

	require.Equal(t, "piri", status.Name)
	require.True(t, status.Active)
	require.True(t, status.Running)
	require.True(t, status.Enabled)
	require.False(t, status.Failed)
}

func TestServiceManager_CheckServicesNotRunning(t *testing.T) {
	tests := []struct {
		name        string
		services    map[string]string // service -> status
		shouldError bool
		errorMsg    string
	}{
		{
			name: "all services inactive",
			services: map[string]string{
				"piri":              "inactive",
				"piri-updater.timer": "inactive",
			},
			shouldError: false,
		},
		{
			name: "some services active",
			services: map[string]string{
				"piri":              "active",
				"piri-updater.timer": "inactive",
			},
			shouldError: true,
			errorMsg:    "piri is running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := NewMockCommandExecutor()
			var serviceList []string

			for service, status := range tt.services {
				serviceList = append(serviceList, service)
				mockExec.outputs["systemctl is-active "+service] = status
			}

			sm := NewServiceManagerWithExecutor(mockExec, serviceList...)
			err := sm.CheckServicesNotRunning()

			if tt.shouldError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestServiceManager_ReloadDaemon(t *testing.T) {
	mockExec := NewMockCommandExecutor()
	sm := NewServiceManagerWithExecutor(mockExec)

	err := sm.ReloadDaemon()
	require.NoError(t, err)
	require.Contains(t, mockExec.calls, "systemctl daemon-reload")

	// Test with error
	mockExec.errors["systemctl daemon-reload"] = fmt.Errorf("permission denied")
	err = sm.ReloadDaemon()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to reload systemd daemon")
}

func TestServiceManager_StopAllServices(t *testing.T) {
	mockExec := NewMockCommandExecutor()

	// Set up services as active
	mockExec.outputs["systemctl is-active piri"] = "active"
	mockExec.outputs["systemctl is-active piri-updater.timer"] = "active"

	sm := NewServiceManagerWithExecutor(mockExec, "piri", "piri-updater.timer")
	err := sm.StopAllServices()
	require.NoError(t, err)

	// Verify all services were stopped and disabled
	require.Contains(t, mockExec.calls, "systemctl stop piri")
	require.Contains(t, mockExec.calls, "systemctl disable piri")
	require.Contains(t, mockExec.calls, "systemctl stop piri-updater.timer")
	require.Contains(t, mockExec.calls, "systemctl disable piri-updater.timer")
}