package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestInstaller_GenerateSystemdServices(t *testing.T) {
	installer, err := NewInstaller()
	require.NoError(t, err)

	serviceUser := "testuser"
	services := installer.GenerateSystemdServices(serviceUser)

	// Should generate 3 service files
	require.Len(t, services, 3)

	// Check main service
	mainService := services[0]
	require.Equal(t, PiriServiceFile, mainService.Name)
	require.Contains(t, mainService.Content, fmt.Sprintf("User=%s", serviceUser))
	require.Contains(t, mainService.Content, "ExecStart=")
	require.Contains(t, mainService.Content, PiriServeCommand)

	// Check updater service
	updaterService := services[1]
	require.Equal(t, PiriUpdateServiceFile, updaterService.Name)
	require.Contains(t, updaterService.Content, fmt.Sprintf("User=%s", serviceUser))
	require.Contains(t, updaterService.Content, PiriUpdateCommand)

	// Check timer
	timerService := services[2]
	require.Equal(t, PiriUpdateTimerServiceFile, timerService.Name)
	require.Contains(t, timerService.Content, "OnBootSec=")
	require.Contains(t, timerService.Content, "OnUnitActiveSec=")
}

func TestInstaller_InstallBinary(t *testing.T) {
	// Setup test environment with custom config
	tempDir := t.TempDir()
	testConfig := &PathConfig{
		OptDir:         tempDir,
		BinaryBaseDir:  filepath.Join(tempDir, "bin"),
		CurrentSymlink: filepath.Join(tempDir, "bin", "current"),
		SystemDir:      filepath.Join(tempDir, "etc"),
		SystemdDir:     filepath.Join(tempDir, "systemd"),
	}

	// Save and override global config
	oldConfig := Config
	Config = testConfig
	PiriOptDir = testConfig.OptDir
	PiriBinaryBaseDir = testConfig.BinaryBaseDir
	PiriCurrentSymlink = testConfig.CurrentSymlink
	PiriSystemDir = testConfig.SystemDir
	PiriSystemdDir = testConfig.SystemdDir
	defer func() {
		Config = oldConfig
		PiriOptDir = Config.OptDir
		PiriBinaryBaseDir = Config.BinaryBaseDir
		PiriCurrentSymlink = Config.CurrentSymlink
		PiriSystemDir = Config.SystemDir
		PiriSystemdDir = Config.SystemdDir
	}()

	installer, err := NewInstaller()
	require.NoError(t, err)

	// Create a test command
	cmd := &cobra.Command{}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	// Create directory structure first
	err = installer.FileSystem.CreatePiriDirectoryStructure("v1.0.0")
	require.NoError(t, err)

	// Install binary (will copy the test executable)
	err = installer.InstallBinary(cmd, "v1.0.0")
	require.NoError(t, err)

	// Verify binary was copied
	binaryPath := filepath.Join(testConfig.BinaryBaseDir, "v1.0.0", "piri")
	require.FileExists(t, binaryPath)

	// Verify symlink was created
	require.FileExists(t, testConfig.CurrentSymlink)
	target, err := os.Readlink(testConfig.CurrentSymlink)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(testConfig.BinaryBaseDir, "v1.0.0"), target)
}

func TestInstaller_InstallConfiguration(t *testing.T) {
	tempDir := t.TempDir()

	// Override global paths
	oldSystemConfigPath := PiriSystemConfigPath
	PiriSystemConfigPath = filepath.Join(tempDir, "config.toml")
	defer func() {
		PiriSystemConfigPath = oldSystemConfigPath
	}()

	installer, err := NewInstaller()
	require.NoError(t, err)

	// Create a test command
	cmd := &cobra.Command{}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	// Create test config
	testConfig := config.FullServerConfig{
		Server: config.ServerConfig{
			Port: 3000,
			Host: "localhost",
		},
	}

	err = installer.InstallConfiguration(cmd, testConfig)
	require.NoError(t, err)

	// Verify config was written
	require.FileExists(t, PiriSystemConfigPath)

	// Verify config content
	var readConfig config.FullServerConfig
	data, err := os.ReadFile(PiriSystemConfigPath)
	require.NoError(t, err)
	err = toml.Unmarshal(data, &readConfig)
	require.NoError(t, err)
	require.Equal(t, testConfig.Server.Port, readConfig.Server.Port)
}

func TestInstaller_CreateCLISymlink(t *testing.T) {
	tempDir := t.TempDir()

	// Create test paths
	testBinaryPath := filepath.Join(tempDir, "bin", "piri")
	testSymlinkPath := filepath.Join(tempDir, "symlink", "piri")

	// Create the binary file
	err := os.MkdirAll(filepath.Dir(testBinaryPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(testBinaryPath, []byte("test"), 0755)
	require.NoError(t, err)

	// Override global paths
	oldBinaryPath := PiriBinaryPath
	oldSymlinkPath := PiriCLISymlinkPath
	PiriBinaryPath = testBinaryPath
	PiriCLISymlinkPath = testSymlinkPath
	defer func() {
		PiriBinaryPath = oldBinaryPath
		PiriCLISymlinkPath = oldSymlinkPath
	}()

	installer, err := NewInstaller()
	require.NoError(t, err)

	// Create a test command
	cmd := &cobra.Command{}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	err = installer.CreateSymlink(cmd)
	require.NoError(t, err)

	// Verify symlink was created
	require.FileExists(t, testSymlinkPath)
	target, err := os.Readlink(testSymlinkPath)
	require.NoError(t, err)
	require.Equal(t, testBinaryPath, target)
}

func TestInstaller_CreateSudoersEntry(t *testing.T) {
	tempDir := t.TempDir()
	sudoersFile := filepath.Join(tempDir, "piri-sudoers")

	// Override global path
	oldSudoersFile := PiriSudoersFile
	PiriSudoersFile = sudoersFile
	defer func() {
		PiriSudoersFile = oldSudoersFile
	}()

	installer, err := NewInstaller()
	require.NoError(t, err)

	// Create a test command
	cmd := &cobra.Command{}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	err = installer.CreateSudoersEntry(cmd, "testuser", false)
	require.NoError(t, err)

	// Verify sudoers file was created with correct permissions
	require.FileExists(t, sudoersFile)
	info, err := os.Stat(sudoersFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0440), info.Mode().Perm())

	// Verify content
	content, err := os.ReadFile(sudoersFile)
	require.NoError(t, err)
	require.Contains(t, string(content), "testuser ALL=(root) NOPASSWD:")
	require.Contains(t, string(content), "systemctl restart piri")
}

func TestInstaller_ServiceFileGeneration(t *testing.T) {
	installer, err := NewInstaller()
	require.NoError(t, err)

	tests := []struct {
		name     string
		genFunc  func(string) string
		user     string
		contains []string
	}{
		{
			name:    "piri service",
			genFunc: installer.generatePiriService,
			user:    "testuser",
			contains: []string{
				"User=testuser",
				"Group=testuser",
				"ExecStart=",
				"serve full",
				"TimeoutStopSec=",
			},
		},
		{
			name:    "updater service",
			genFunc: installer.generatePiriUpdaterService,
			user:    "testuser",
			contains: []string{
				"User=testuser",
				"Type=oneshot",
				"update-internal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.genFunc(tt.user)
			for _, expected := range tt.contains {
				require.Contains(t, content, expected)
			}
		})
	}

	// Test timer separately since it doesn't take a user parameter
	t.Run("updater timer", func(t *testing.T) {
		content := installer.generatePiriUpdaterTimer()
		expected := []string{
			"OnBootSec=",
			"OnUnitActiveSec=",
			"RandomizedDelaySec=",
			"Persistent=true",
		}
		for _, exp := range expected {
			require.Contains(t, content, exp)
		}
	})
}

func TestInstallState_Validation(t *testing.T) {
	// Test that InstallState properly holds all required fields
	state := &InstallState{
		ConfigPath:       "/path/to/config.toml",
		Config:           config.FullServerConfig{},
		Force:            true,
		EnableAutoUpdate: true,
		ServiceUser:      "piriuser",
		Version:          "v1.0.0",
		IsUpdate:         false,
	}

	require.Equal(t, "/path/to/config.toml", state.ConfigPath)
	require.True(t, state.Force)
	require.True(t, state.EnableAutoUpdate)
	require.Equal(t, "piriuser", state.ServiceUser)
	require.Equal(t, "v1.0.0", state.Version)
	require.False(t, state.IsUpdate)
}

func TestInstaller_Cleanup(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()

	installer, err := NewInstaller()
	require.NoError(t, err)

	// Create some test files to rollback
	testFile := filepath.Join(tempDir, "test.txt")
	err = installer.FileSystem.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	testDir := filepath.Join(tempDir, "testdir")
	err = installer.FileSystem.CreateDirectory(testDir, 0755)
	require.NoError(t, err)

	// Verify files exist
	require.FileExists(t, testFile)
	require.DirExists(t, testDir)

	// Create a test command
	cmd := &cobra.Command{}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	// Perform cleanup
	err = installer.Cleanup(cmd, false)
	require.NoError(t, err)

	// Verify files were removed
	require.NoFileExists(t, testFile)
	require.NoDirExists(t, testDir)
}

func TestTimeConstants(t *testing.T) {
	// Verify time constants are reasonable
	require.Equal(t, 2*time.Minute, PiriUpdateBootDuration)
	require.Equal(t, 30*time.Minute, PiriUpdateUnitActiveDuration)
	require.Equal(t, 5*time.Minute, PiriUpdateRandomizedDelayDuration)
	require.Equal(t, time.Minute, PiriServerShutdownTimeout)
	require.Equal(t, 15*time.Second, PiriSystemdShutdownBuffer)
}

func TestServiceConstants(t *testing.T) {
	// Verify service-related constants
	require.Equal(t, "piri", PiriServiceName)
	require.Equal(t, "piri-updater.timer", PiriUpdateTimerName)
	require.Equal(t, "piri.service", PiriServiceFile)
	require.Equal(t, "piri-updater.service", PiriUpdateServiceFile)
	require.Equal(t, "piri-updater.timer", PiriUpdateTimerServiceFile)
	require.Equal(t, "serve full", PiriServeCommand)
	require.Equal(t, "update-internal", PiriUpdateCommand)
	require.True(t, strings.Contains(ReleaseURL, "github.com"))
}
