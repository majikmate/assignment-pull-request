package permissions

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/majikmate/assignment-pull-request/internal/git"
	"github.com/majikmate/assignment-pull-request/internal/userutil"
)

// Security-related constants for rsync operations
const (
	// User and ownership constants
	mmUser      = "majikmate"
	mmOwner     = mmUser + ":" + mmUser
	StagePrefix = mmUser + "-protect-sync-stage-"

	// Path constants for security validation (need to be hardcoded)
	githookRsyncPath = "/etc/git/hooks/githook-rsync"
	workspacesPath   = "/workspaces/"
	tmpPath          = "/tmp/"

	// Pattern constants for staging directory validation
	stagePatternRegex = `^` + tmpPath + StagePrefix + `[a-zA-Z0-9]{8,}$`
)

// System paths that are restricted for security (defense-in-depth)
var systemPaths = []string{"/etc/", "/usr/", "/bin/", "/sbin/", "/boot/", "/sys/", "/proc/", "/dev/"}

// Processor provides secure rsync operations for githook path protection
type Processor struct {
	realUser string
}

// NewProcessor creates a new secure rsync wrapper
func NewProcessor() (*Processor, error) {
	realUser, err := userutil.GetValidatedRealUser()
	if err != nil {
		return nil, fmt.Errorf("failed to determine real user: %w", err)
	}

	return &Processor{
		realUser: realUser,
	}, nil
}

// UpdatePermissions performs secure rsync from staging directory to working tree
func (rw *Processor) UpdatePermissions(source, dest string) error {
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
	return rw.updatePermissions(sourceReal, destReal)
}

// validateSourcePath validates the source directory meets security requirements
func (rw *Processor) validateSourcePath(sourcePath string) error {
	// Source must be under /tmp and within a valid staging directory
	stagePattern := regexp.MustCompile(stagePatternRegex)

	// Check if the source path itself matches (for full staging directory sync)
	// or if its parent directory matches (for subdirectory sync)
	var isValid bool

	// First check if the path itself matches the staging pattern
	if stagePattern.MatchString(sourcePath) {
		isValid = true
	} else {
		// Check if the parent directory matches the staging pattern
		parent := filepath.Dir(sourcePath)
		if stagePattern.MatchString(parent) {
			isValid = true
		}
	}

	if !isValid {
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
func (rw *Processor) validateDestinationPath(destPath string) error {
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

	// Destination must be within a git repository
	// Use git rev-parse --git-dir to check if we're in a git repository
	gitOps := git.NewOperationsWithDir(false, destPath)
	if _, err := gitOps.GetGitDir(); err != nil {
		return fmt.Errorf("destination is not within a git repository: %w", err)
	}

	// Destination must not be under sensitive system directories (defense-in-depth)
	for _, systemPath := range systemPaths {
		if strings.HasPrefix(destPath, systemPath) {
			return fmt.Errorf("cannot sync to system directories")
		}
	}

	// Destination must be under /workspaces (primary location restriction)
	if !strings.HasPrefix(destPath, workspacesPath) {
		return fmt.Errorf("destination must be under %s directory", workspacesPath)
	}

	// Parent directory of destination must be owned by current user (prevent cross-user access)
	destParent := filepath.Dir(destPath)
	if err := rw.validateOwnership(destParent, rw.realUser); err != nil {
		return fmt.Errorf("destination parent ownership validation failed: %w", err)
	}

	return nil
}

// validateOwnership checks if a path is owned by the specified user
func (rw *Processor) validateOwnership(path, expectedUser string) error {
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

// updatePermissions runs the actual rsync command with secure parameters
func (rw *Processor) updatePermissions(sourcePath, destPath string) error {
	// First, set ownership on all content in the source staging directory (but not the directory itself)
	chownCmd := exec.Command("find", sourcePath, "-mindepth", "1", "-exec", "chown", mmOwner, "{}", "+")
	if err := chownCmd.Run(); err != nil {
		return fmt.Errorf("failed to set ownership in staging directory: %w", err)
	}

	// Set permissions using chmod with symbolic mode that preserves executable files:
	// u=rwX,go=rX = user: read+write+execute_if_dir_or_executable
	//               group+other: read+execute_if_dir_or_executable
	// 'X' sets execute permission on:
	//   - Directories (always, for traversal)
	//   - Files that already have execute permission (preserves executables)
	// This results in:
	//   - Directories: 0755 (always executable for traversal)
	//   - Regular files: 0644 (not executable unless they were already)
	//   - Executable files: 0755 (preserve executable status)
	chmodCmd := exec.Command("find", sourcePath, "-mindepth", "1", "-exec", "chmod", "u=rwX,go=rX", "{}", "+")
	if err := chmodCmd.Run(); err != nil {
		return fmt.Errorf("failed to set permissions in staging directory: %w", err)
	}

	// Use rsync with specific flags to sync contents without affecting destination directory
	args := []string{
		"--recursive", // Recurse into directories
		"--links",     // Copy symlinks as symlinks
		"--perms",     // Preserve permissions
		"--times",     // Preserve modification times
		"--group",     // Preserve group
		"--owner",     // Preserve owner (from our pre-chown)
		"--verbose",
		"--omit-dir-times", // Don't update timestamps on existing destination directories
		"--no-specials",
		"--no-devices",
		"--safe-links",
		"--exclude=.git",
		"--exclude=.git/",
		"--exclude=.git/*",
		filepath.Clean(sourcePath) + string(filepath.Separator), // Trailing slash means "sync contents of this directory"
		filepath.Clean(destPath) + string(filepath.Separator),   // Trailing slash means "into this directory" (don't replace it)
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

// ExecuteUpdatePermissions executes the githook-rsync binary with sudo for privileged operations
func (rw *Processor) ExecuteUpdatePermissions(stageDir, repositoryRoot string) error {
	if stageDir == "" || repositoryRoot == "" {
		return fmt.Errorf("all parameters are required for githook rsync execution")
	}

	// Resolve paths to absolute canonical paths to prevent traversal
	stageDirReal, err := filepath.Abs(filepath.Clean(stageDir) + string(filepath.Separator))
	if err != nil {
		return fmt.Errorf("cannot resolve stage directory path: %w", err)
	}

	repositoryRootReal, err := filepath.Abs(filepath.Clean(repositoryRoot) + string(filepath.Separator))
	if err != nil {
		return fmt.Errorf("cannot resolve repository root path: %w", err)
	}

	// Validate stage directory using existing security validations
	if err := rw.validateSourcePath(stageDirReal); err != nil {
		return fmt.Errorf("stage directory validation failed: %w", err)
	}

	// Validate repository root using existing security validations
	if err := rw.validateDestinationPath(repositoryRootReal); err != nil {
		return fmt.Errorf("repository root validation failed: %w", err)
	}

	// Run githook-rsync with sudo for ownership operations
	rsyncCmd := exec.Command("sudo", githookRsyncPath, stageDirReal, repositoryRootReal)
	rsyncCmd.Env = append(os.Environ(), "SUDO_USER="+rw.realUser)
	rsyncCmd.Stdout = os.Stdout
	rsyncCmd.Stderr = os.Stderr

	if err := rsyncCmd.Run(); err != nil {
		return fmt.Errorf("atomic rsync failed: %w", err)
	}

	return nil
}
