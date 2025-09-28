package protect

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/majikmate/assignment-pull-request/internal/userutil"
)

// RsyncWrapper provides secure rsync operations for githook path protection
type RsyncWrapper struct {
	realUser string
}

// NewRsyncWrapper creates a new secure rsync wrapper
func NewRsyncWrapper() (*RsyncWrapper, error) {
	realUser, err := userutil.GetValidatedRealUser()
	if err != nil {
		return nil, fmt.Errorf("failed to determine real user: %w", err)
	}

	return &RsyncWrapper{
		realUser: realUser,
	}, nil
}

// SyncDirectory performs secure rsync from staging directory to working tree
func (rw *RsyncWrapper) SyncDirectory(source, dest string) error {
	// Validate arguments
	if source == "" || dest == "" {
		return fmt.Errorf("source and destination paths are required")
	}

	// Resolve paths to absolute canonical paths to prevent traversal
	sourceReal, err := filepath.Abs(filepath.Clean(source))
	if err != nil {
		return fmt.Errorf("cannot resolve source path: %w", err)
	}

	destReal, err := filepath.Abs(filepath.Clean(dest))
	if err != nil {
		return fmt.Errorf("cannot resolve destination path: %w", err)
	}

	// Security validations
	if err := rw.validateSourcePath(sourceReal); err != nil {
		return fmt.Errorf("source validation failed: %w", err)
	}

	if err := rw.validateDestinationPath(destReal); err != nil {
		return fmt.Errorf("destination validation failed: %w", err)
	}

	// Additional safety: Ensure source has trailing slash for rsync safety
	if !strings.HasSuffix(source, "/") {
		return fmt.Errorf("source must end with trailing slash")
	}

	// Execute the secure rsync operation
	return rw.executeRsync(sourceReal, destReal)
}

// validateSourcePath validates the source directory meets security requirements
func (rw *RsyncWrapper) validateSourcePath(sourcePath string) error {
	// Source must be under /tmp and match our specific pattern
	stagePattern := regexp.MustCompile(`^/tmp/majikmate-protect-sync-stage-[a-zA-Z0-9]{10,}$`)
	if !stagePattern.MatchString(sourcePath) {
		return fmt.Errorf("invalid source directory pattern: %s", sourcePath)
	}

	// Source must exist, be a directory, not be a symlink
	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot access source directory: %w", err)
	}

	if !sourceInfo.IsDir() {
		return fmt.Errorf("source must be a directory")
	}

	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source must be a real directory, not a symlink")
	}

	// Source must be owned by current user
	if err := rw.validateOwnership(sourcePath, rw.realUser); err != nil {
		return fmt.Errorf("source ownership validation failed: %w", err)
	}

	return nil
}

// validateDestinationPath validates the destination directory meets security requirements
func (rw *RsyncWrapper) validateDestinationPath(destPath string) error {
	// Destination must exist, be a directory, not be a symlink
	destInfo, err := os.Lstat(destPath)
	if err != nil {
		return fmt.Errorf("cannot access destination directory: %w", err)
	}

	if !destInfo.IsDir() {
		return fmt.Errorf("destination must be a directory")
	}

	if destInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("destination must be a real directory, not a symlink")
	}

	// Destination must contain .git (be a git repository)
	gitPath := filepath.Join(destPath, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		return fmt.Errorf("destination is not a git repository")
	}

	// Destination must not be under sensitive system directories (defense-in-depth)
	systemPaths := []string{"/etc/", "/usr/", "/bin/", "/sbin/", "/boot/", "/sys/", "/proc/", "/dev/"}
	for _, systemPath := range systemPaths {
		if strings.HasPrefix(destPath, systemPath) {
			return fmt.Errorf("cannot sync to system directories")
		}
	}

	// Destination must be under /workspaces (primary location restriction)
	if !strings.HasPrefix(destPath, "/workspaces/") {
		return fmt.Errorf("destination must be under /workspaces directory")
	}

	// Parent directory of destination must be owned by current user (prevent cross-user access)
	destParent := filepath.Dir(destPath)
	if err := rw.validateOwnership(destParent, rw.realUser); err != nil {
		return fmt.Errorf("destination parent ownership validation failed: %w", err)
	}

	return nil
}

// validateOwnership checks if a path is owned by the specified user
func (rw *RsyncWrapper) validateOwnership(path, expectedUser string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot determine ownership of %s: %w", path, err)
	}

	// Get the file's owner
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot get file system info for %s", path)
	}

	// Look up the user by UID
	owner, err := user.LookupId(fmt.Sprintf("%d", stat.Uid))
	if err != nil {
		return fmt.Errorf("cannot lookup user by UID %d: %w", stat.Uid, err)
	}

	if owner.Username != expectedUser {
		return fmt.Errorf("path %s must be owned by %s, but is owned by %s", path, expectedUser, owner.Username)
	}

	return nil
}

// executeRsync runs the actual rsync command with secure parameters
func (rw *RsyncWrapper) executeRsync(sourcePath, destPath string) error {
	// Prepare rsync command with explicit, safe parameters
	args := []string{
		"--archive",
		"--verbose",
		"--delete",
		"--no-owner",
		"--no-group",
		"--omit-dir-times",
		"--chown=majikmate:majikmate",
		"--no-specials",
		"--no-devices",
		"--safe-links",
		"--exclude=.git",
		"--exclude=.git/",
		"--exclude=.git/*",
		sourcePath + "/", // Ensure trailing slash
		destPath,
	}

	cmd := exec.Command("rsync", args...)

	// Set up output handling
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	return nil
}
