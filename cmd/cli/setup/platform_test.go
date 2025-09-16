package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlatformChecker_OSDetection(t *testing.T) {
	platform, err := NewPlatformChecker()
	require.NoError(t, err)

	// Test OS detection matches runtime
	require.Equal(t, runtime.GOOS, platform.OS)

	if runtime.GOOS == "linux" {
		require.True(t, platform.IsLinux)
		require.False(t, platform.IsDarwin)
	} else if runtime.GOOS == "darwin" {
		require.False(t, platform.IsLinux)
		require.True(t, platform.IsDarwin)
	}
}

func TestPlatformChecker_RequireLinux(t *testing.T) {
	platform, err := NewPlatformChecker()
	require.NoError(t, err)

	err = platform.RequireLinux()
	if runtime.GOOS == "linux" {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
		require.Contains(t, err.Error(), "only supported on Linux")
	}
}

func TestPlatformChecker_RequireRoot(t *testing.T) {
	platform, err := NewPlatformChecker()
	require.NoError(t, err)

	err = platform.RequireRoot()
	if platform.IsRoot {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
		require.Contains(t, err.Error(), "root privileges")
	}
}

func TestPlatformChecker_IsManagedInstallation(t *testing.T) {
	platform, err := NewPlatformChecker()
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "managed installation",
			path:     "/opt/piri/bin/current/piri",
			expected: true,
		},
		{
			name:     "managed subdirectory",
			path:     "/opt/piri/something/else",
			expected: true,
		},
		{
			name:     "standalone installation",
			path:     "/usr/local/bin/piri",
			expected: false,
		},
		{
			name:     "home directory",
			path:     "/home/user/piri",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := platform.IsManagedInstallation(tt.path)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetExecutablePath(t *testing.T) {
	// This test just verifies the function doesn't error
	// The actual path will vary by test environment
	path, err := GetExecutablePath()
	require.NoError(t, err)
	require.NotEmpty(t, path)

	// Path should be absolute
	require.True(t, filepath.IsAbs(path))
}

func TestNeedsElevatedPrivileges(t *testing.T) {
	// Test with temp directory where we have write access
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(tempFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Should not need elevated privileges for our own temp file
	require.False(t, NeedsElevatedPrivileges(tempFile))

	// Test with system directory (likely needs privileges)
	// Note: This might vary depending on test environment
	if runtime.GOOS != "windows" {
		systemPath := "/etc/test-piri"
		needsPriv := NeedsElevatedPrivileges(systemPath)
		// If not running as root, should need privileges
		if os.Geteuid() != 0 {
			require.True(t, needsPriv)
		}
	}
}

func TestPrerequisites_Validate(t *testing.T) {
	platform, err := NewPlatformChecker()
	require.NoError(t, err)

	tests := []struct {
		name      string
		prereqs   Prerequisites
		force     bool
		expectErr bool
		errMsg    string
	}{
		{
			name: "no requirements",
			prereqs: Prerequisites{
				Platform:     platform,
				NeedsSystemd: false,
				NeedsRoot:    false,
			},
			force:     false,
			expectErr: false,
		},
		{
			name: "root requirement not met",
			prereqs: Prerequisites{
				Platform:  platform,
				NeedsRoot: true,
			},
			force:     false,
			expectErr: !platform.IsRoot,
			errMsg:    "root privileges",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.prereqs.Validate(tt.force)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPrerequisites_ValidateWithFiles(t *testing.T) {
	tempDir := t.TempDir()
	platform, err := NewPlatformChecker()
	require.NoError(t, err)

	// Create an existing file
	existingFile := filepath.Join(tempDir, "existing.txt")
	err = os.WriteFile(existingFile, []byte("exists"), 0644)
	require.NoError(t, err)

	nonExistingFile := filepath.Join(tempDir, "missing.txt")

	prereqs := Prerequisites{
		Platform:   platform,
		CheckFiles: []string{existingFile, nonExistingFile},
	}

	// Without force, should fail due to existing file
	err = prereqs.Validate(false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "existing")

	// With force, should succeed
	err = prereqs.Validate(true)
	require.NoError(t, err)

	// With only non-existing files, should succeed
	prereqs.CheckFiles = []string{nonExistingFile}
	err = prereqs.Validate(false)
	require.NoError(t, err)
}

func TestPlatformChecker_ServiceUserDetection(t *testing.T) {
	platform, err := NewPlatformChecker()
	require.NoError(t, err)

	// Service user should be detected (from env vars or current user)
	// We can't test sudo behavior easily, but we can verify we get a user
	if platform.IsLinux || platform.IsDarwin {
		// Should have detected some user
		require.NotEmpty(t, platform.ServiceUser)
	}
}

func TestPlatformChecker_detectServiceUser(t *testing.T) {
	platform := &PlatformChecker{}

	// Test with SUDO_USER set
	oldSudoUser := os.Getenv("SUDO_USER")
	defer os.Setenv("SUDO_USER", oldSudoUser)

	os.Setenv("SUDO_USER", "testuser")
	user, err := platform.detectServiceUser()
	require.NoError(t, err)
	require.Equal(t, "testuser", user)

	// Test with USER env var
	os.Unsetenv("SUDO_USER")
	oldUser := os.Getenv("USER")
	defer os.Setenv("USER", oldUser)

	os.Setenv("USER", "regularuser")
	user, err = platform.detectServiceUser()
	require.NoError(t, err)
	require.Equal(t, "regularuser", user)
}