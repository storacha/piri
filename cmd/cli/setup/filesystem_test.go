package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileSystemManager_CreateDirectory(t *testing.T) {
	tempDir := t.TempDir()
	fsm := NewFileSystemManager()

	tests := []struct {
		name    string
		path    string
		perm    os.FileMode
		wantErr bool
	}{
		{
			name: "create simple directory",
			path: filepath.Join(tempDir, "simple"),
			perm: 0755,
		},
		{
			name: "create nested directories",
			path: filepath.Join(tempDir, "nested", "deep", "dir"),
			perm: 0755,
		},
		{
			name: "create with restricted permissions",
			path: filepath.Join(tempDir, "restricted"),
			perm: 0700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fsm.CreateDirectory(tt.path, tt.perm)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.DirExists(t, tt.path)
				// Verify it was tracked for rollback
				require.Contains(t, fsm.CreatedDirs, tt.path)
			}
		})
	}
}

func TestFileSystemManager_WriteFile(t *testing.T) {
	tempDir := t.TempDir()
	fsm := NewFileSystemManager()

	testFile := filepath.Join(tempDir, "test.txt")
	testData := []byte("test content")

	err := fsm.WriteFile(testFile, testData, 0644)
	require.NoError(t, err)
	require.FileExists(t, testFile)

	// Verify content
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Equal(t, testData, content)

	// Verify it was tracked for rollback
	require.Contains(t, fsm.CreatedFiles, testFile)
}

func TestFileSystemManager_CreateSymlink(t *testing.T) {
	tempDir := t.TempDir()
	fsm := NewFileSystemManager()

	// Create a target file
	targetFile := filepath.Join(tempDir, "target.txt")
	err := os.WriteFile(targetFile, []byte("target"), 0644)
	require.NoError(t, err)

	// Create symlink
	symlinkPath := filepath.Join(tempDir, "link.txt")
	err = fsm.CreateSymlink(targetFile, symlinkPath)
	require.NoError(t, err)

	// Verify symlink exists and points to correct target
	target, err := os.Readlink(symlinkPath)
	require.NoError(t, err)
	require.Equal(t, targetFile, target)

	// Test overwriting existing symlink
	newTarget := filepath.Join(tempDir, "new_target.txt")
	err = os.WriteFile(newTarget, []byte("new"), 0644)
	require.NoError(t, err)

	err = fsm.CreateSymlink(newTarget, symlinkPath)
	require.NoError(t, err)

	target, err = os.Readlink(symlinkPath)
	require.NoError(t, err)
	require.Equal(t, newTarget, target)
}

func TestFileSystemManager_UpdateSymlinkAtomic(t *testing.T) {
	tempDir := t.TempDir()
	fsm := NewFileSystemManager()

	// Create initial target
	target1 := filepath.Join(tempDir, "v1")
	err := os.Mkdir(target1, 0755)
	require.NoError(t, err)

	symlinkPath := filepath.Join(tempDir, "current")
	err = os.Symlink(target1, symlinkPath)
	require.NoError(t, err)

	// Update to new target
	target2 := filepath.Join(tempDir, "v2")
	err = os.Mkdir(target2, 0755)
	require.NoError(t, err)

	oldTarget, rollback, err := fsm.UpdateSymlinkAtomic(symlinkPath, target2)
	require.NoError(t, err)
	require.Equal(t, target1, oldTarget)
	require.NotNil(t, rollback)

	// Verify symlink points to new target
	current, err := os.Readlink(symlinkPath)
	require.NoError(t, err)
	require.Equal(t, target2, current)

	// Test rollback
	err = rollback()
	require.NoError(t, err)

	current, err = os.Readlink(symlinkPath)
	require.NoError(t, err)
	require.Equal(t, target1, current)
}

func TestFileSystemManager_Rollback(t *testing.T) {
	tempDir := t.TempDir()
	fsm := NewFileSystemManager()

	// Create some directories and files
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir1", "dir2")
	file1 := filepath.Join(dir1, "file1.txt")
	file2 := filepath.Join(dir2, "file2.txt")

	err := fsm.CreateDirectory(dir1, 0755)
	require.NoError(t, err)
	err = fsm.CreateDirectory(dir2, 0755)
	require.NoError(t, err)
	err = fsm.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)
	err = fsm.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)

	// Verify everything exists
	require.DirExists(t, dir1)
	require.DirExists(t, dir2)
	require.FileExists(t, file1)
	require.FileExists(t, file2)

	// Perform rollback
	err = fsm.Rollback()
	require.NoError(t, err)

	// Verify everything is removed
	require.NoDirExists(t, dir1)
	require.NoDirExists(t, dir2)
	require.NoFileExists(t, file1)
	require.NoFileExists(t, file2)
}

func TestFileSystemManager_CheckExistingFiles(t *testing.T) {
	tempDir := t.TempDir()
	fsm := NewFileSystemManager()

	// Create some files
	existingFile := filepath.Join(tempDir, "existing.txt")
	err := os.WriteFile(existingFile, []byte("exists"), 0644)
	require.NoError(t, err)

	nonExistingFile := filepath.Join(tempDir, "missing.txt")

	// Test with mixed existing and non-existing files
	err = fsm.CheckExistingFiles([]string{existingFile, nonExistingFile})
	require.Error(t, err)
	require.Contains(t, err.Error(), "existing.txt")
	require.NotContains(t, err.Error(), "missing.txt")

	// Test with all non-existing files
	err = fsm.CheckExistingFiles([]string{nonExistingFile})
	require.NoError(t, err)
}

func TestFileSystemManager_CopyFile(t *testing.T) {
	tempDir := t.TempDir()
	fsm := NewFileSystemManager()

	// Create source file
	sourceFile := filepath.Join(tempDir, "source.txt")
	sourceContent := []byte("source content")
	err := os.WriteFile(sourceFile, sourceContent, 0644)
	require.NoError(t, err)

	// Copy file
	destFile := filepath.Join(tempDir, "dest.txt")
	err = fsm.CopyFile(sourceFile, destFile, 0755)
	require.NoError(t, err)

	// Verify destination exists with correct content
	require.FileExists(t, destFile)
	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	require.Equal(t, sourceContent, content)

	// Verify permissions (check executable bit)
	info, err := os.Stat(destFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), info.Mode().Perm())

	// Verify it was tracked for rollback
	require.Contains(t, fsm.CreatedFiles, destFile)
}

func TestFileSystemManager_CreatePiriDirectoryStructure(t *testing.T) {
	// Save original config and create test config
	oldConfig := Config
	defer func() {
		Config = oldConfig
		// Reset compatibility variables
		PiriOptDir = Config.OptDir
		PiriBinaryBaseDir = Config.BinaryBaseDir
		PiriSystemDir = Config.SystemDir
		PiriSystemdBaseDir = Config.SystemdBaseDir
		PiriSystemdCurrentSymlink = Config.SystemdCurrentSymlink
	}()

	// Create test config with temp paths
	tempDir := t.TempDir()
	testConfig := &PathConfig{
		OptDir:                tempDir,
		BinaryBaseDir:         filepath.Join(tempDir, "bin"),
		SystemDir:             filepath.Join(tempDir, "etc"),
		SystemdBaseDir:        filepath.Join(tempDir, "systemd"),
		SystemdCurrentSymlink: filepath.Join(tempDir, "systemd", "current"),
	}

	// Override global config
	Config = testConfig
	PiriOptDir = testConfig.OptDir
	PiriBinaryBaseDir = testConfig.BinaryBaseDir
	PiriSystemDir = testConfig.SystemDir
	PiriSystemdBaseDir = testConfig.SystemdBaseDir
	PiriSystemdCurrentSymlink = testConfig.SystemdCurrentSymlink

	fsm := NewFileSystemManager()
	err := fsm.CreatePiriDirectoryStructure("v1.0.0")
	require.NoError(t, err)

	// Verify all directories were created
	require.DirExists(t, filepath.Join(testConfig.BinaryBaseDir, "v1.0.0"))
	require.DirExists(t, testConfig.SystemDir)
	require.DirExists(t, filepath.Join(testConfig.SystemdBaseDir, "v1.0.0"))

	// Verify they were tracked for rollback
	require.Len(t, fsm.CreatedDirs, 3)
}

func TestFileExists(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Test existing file
	require.True(t, FileExists(testFile))

	// Test non-existing file
	require.False(t, FileExists(filepath.Join(tempDir, "missing.txt")))

	// Test existing directory
	require.True(t, FileExists(tempDir))
}

func TestIsSymlink(t *testing.T) {
	tempDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tempDir, "regular.txt")
	err := os.WriteFile(regularFile, []byte("regular"), 0644)
	require.NoError(t, err)

	// Create a symlink
	symlinkPath := filepath.Join(tempDir, "link.txt")
	err = os.Symlink(regularFile, symlinkPath)
	require.NoError(t, err)

	// Test regular file
	require.False(t, IsSymlink(regularFile))

	// Test symlink
	require.True(t, IsSymlink(symlinkPath))

	// Test directory
	require.False(t, IsSymlink(tempDir))

	// Test non-existing path
	require.False(t, IsSymlink(filepath.Join(tempDir, "missing")))
}
