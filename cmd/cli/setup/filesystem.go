package setup

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/hashicorp/go-multierror"
)

// FileSystemManager handles file system operations with rollback support
type FileSystemManager struct {
	// Track created items for potential rollback
	CreatedDirs  []string
	CreatedFiles []string
}

// NewFileSystemManager creates a new file system manager
func NewFileSystemManager() *FileSystemManager {
	return &FileSystemManager{
		CreatedDirs:  make([]string, 0),
		CreatedFiles: make([]string, 0),
	}
}

// CreateDirectory creates a directory with specified permissions
func (fsm *FileSystemManager) CreateDirectory(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	fsm.CreatedDirs = append(fsm.CreatedDirs, path)
	return nil
}

// CreatePiriDirectoryStructure creates the standard /opt/piri directory structure
func (fsm *FileSystemManager) CreatePiriDirectoryStructure(version string) error {
	// Create versioned binary directory
	versionedBinDir := getVersionedBinaryDir(version)
	if err := fsm.CreateDirectory(versionedBinDir, 0755); err != nil {
		return err
	}

	// Create versioned systemd directory
	versionedSystemdDir := getVersionedSystemdDir(version)
	if err := fsm.CreateDirectory(versionedSystemdDir, 0755); err != nil {
		return err
	}

	// Create config directory (not versioned - contains user config)
	if err := fsm.CreateDirectory(PiriSystemDir, 0755); err != nil {
		return err
	}

	return nil
}

// WriteFile writes data to a file with specified permissions
func (fsm *FileSystemManager) WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	fsm.CreatedFiles = append(fsm.CreatedFiles, path)
	return nil
}

// CreateSymlink creates a symbolic link with rollback capability
func (fsm *FileSystemManager) CreateSymlink(oldpath, newpath string) error {
	// Remove existing symlink if it exists
	if err := os.Remove(newpath); err != nil && !os.IsNotExist(err) {
		// If it's not a symlink, return error
		if info, err := os.Lstat(newpath); err == nil && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("path %s exists and is not a symlink", newpath)
		}
	}

	if err := os.Symlink(oldpath, newpath); err != nil {
		return fmt.Errorf("failed to create symlink %s -> %s: %w", newpath, oldpath, err)
	}
	return nil
}

// UpdateSymlinkAtomic performs an atomic symlink update with rollback capability
func (fsm *FileSystemManager) UpdateSymlinkAtomic(symlinkPath, newTarget string) (oldTarget string, rollback func() error, err error) {
	// Get current target for rollback
	oldTarget, err = os.Readlink(symlinkPath)
	if err != nil && !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("failed to read current symlink: %w", err)
	}

	// Create atomic update
	if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("failed to remove old symlink: %w", err)
	}

	if err := os.Symlink(newTarget, symlinkPath); err != nil {
		// Try to restore old symlink if creation fails
		if oldTarget != "" {
			_ = os.Symlink(oldTarget, symlinkPath)
		}
		return "", nil, fmt.Errorf("failed to create new symlink: %w", err)
	}

	// Return rollback function
	rollback = func() error {
		if oldTarget == "" {
			return os.Remove(symlinkPath)
		}
		_ = os.Remove(symlinkPath)
		return os.Symlink(oldTarget, symlinkPath)
	}

	return oldTarget, rollback, nil
}

// CheckExistingFiles checks if files already exist
func (fsm *FileSystemManager) CheckExistingFiles(files []string) error {
	var errs error
	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			errs = multierror.Append(errs, fmt.Errorf("file already exists: %s", file))
		}
	}
	return errs
}

// RemoveFiles removes a list of files, ignoring non-existent files
func (fsm *FileSystemManager) RemoveFiles(files []string) error {
	var errs error
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			errs = multierror.Append(errs, fmt.Errorf("failed to remove %s: %w", file, err))
		}
	}
	return errs
}

// SetOwnership sets the ownership of a path to the specified user
func (fsm *FileSystemManager) SetOwnership(path string, username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("looking up user %s: %w", username, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("parsing uid: %w", err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("parsing gid: %w", err)
	}

	// Walk the directory tree and set ownership on all files and directories
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}

// SetOwnershipFromPath sets ownership based on another path's ownership
func (fsm *FileSystemManager) SetOwnershipFromPath(targetPath, referencePath string) error {
	fileInfo, err := os.Stat(referencePath)
	if err != nil {
		return fmt.Errorf("failed to stat reference path %s: %w", referencePath, err)
	}

	sys := fileInfo.Sys()
	if stat, ok := sys.(*syscall.Stat_t); ok {
		if err := os.Chown(targetPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}
	return nil
}

// CopyFile copies a file from source to destination with permissions
func (fsm *FileSystemManager) CopyFile(src, dst string, perm os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %w", src, err)
	}
	return fsm.WriteFile(dst, data, perm)
}

// Rollback removes all created files and directories (in reverse order)
func (fsm *FileSystemManager) Rollback() error {
	var errs error

	// Remove files first
	for i := len(fsm.CreatedFiles) - 1; i >= 0; i-- {
		if err := os.Remove(fsm.CreatedFiles[i]); err != nil && !os.IsNotExist(err) {
			errs = multierror.Append(errs, fmt.Errorf("failed to rollback file %s: %w", fsm.CreatedFiles[i], err))
		}
	}

	// Then remove directories (in reverse order to handle nested dirs)
	for i := len(fsm.CreatedDirs) - 1; i >= 0; i-- {
		if err := os.RemoveAll(fsm.CreatedDirs[i]); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("failed to rollback directory %s: %w", fsm.CreatedDirs[i], err))
		}
	}

	return errs
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsSymlink checks if a path is a symbolic link
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// NeedsElevatedPrivileges checks if we need elevated privileges to write to a path
func NeedsElevatedPrivileges(path string) bool {
	// Try to open the file for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		// If we get a permission error, we need elevated privileges
		if os.IsPermission(err) {
			return true
		}
		// For other errors, check if the parent directory is writable
		dir := filepath.Dir(path)
		testFile := filepath.Join(dir, ".piri-update-test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			return os.IsPermission(err)
		}
		_ = os.Remove(testFile)
		return false
	}
	_ = file.Close()
	return false
}
