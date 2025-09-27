package protect

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/git"
	"github.com/majikmate/assignment-pull-request/internal/paths"
	"github.com/majikmate/assignment-pull-request/internal/regex"
)

const (
	mmUser      = "majikmate"
	mmOwner     = mmUser + ":" + mmUser
	stagePrefix = mmUser + "-protect-sync-stage-"
)

// Processor handles path protection operations
type Processor struct {
	repositoryRoot string
	gitOps         *git.Operations
}

// New creates a new protect processor
func New(repositoryRoot string) *Processor {
	return &Processor{
		repositoryRoot: repositoryRoot,
		gitOps:         git.NewOperationsWithDir(false, repositoryRoot), // Use repository root as working directory
	}
}

// ProtectPaths implements the protect-sync logic in Go:
// 1. Acquire exclusive lock to prevent concurrent operations
// 2. Find protected paths using regex patterns
// 3. Check for unmerged entries under protected paths
// 4. Extract files from HEAD for protected paths
// 5. Mirror to working tree with majikmate ownership and permissions
// 6. Apply skip-worktree flags
func (p *Processor) ProtectPaths(protectedFoldersPattern *regex.Processor) error {
	fmt.Printf("🔒 Starting path protection (protect-sync logic)...\n")

	// Acquire exclusive lock to prevent concurrent protect operations
	lock, err := acquireLock(p.repositoryRoot)
	if err != nil {
		return fmt.Errorf("failed to acquire protect-paths lock: %w", err)
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			fmt.Printf("Warning: failed to release protect-paths lock: %v\n", releaseErr)
		}
	}()

	// Must be running as majikmate user (called via sudo -u majikmate)
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	if currentUser.Username != mmUser {
		return fmt.Errorf("protect-sync must run as %s user (via sudo -u %s)", mmUser, mmUser)
	}

	// Find protected paths using patterns
	protectedPathsInfo, err := p.findProtectedPaths(protectedFoldersPattern)
	if err != nil {
		return err
	}

	if protectedPathsInfo.Empty() {
		fmt.Println("No paths match protected patterns")
		return nil
	}

	fmt.Printf("Processing %d protected path(s)...\n", protectedPathsInfo.Count())

	// Execute the protect-sync workflow
	if err := p.checkUnmergedEntries(protectedPathsInfo); err != nil {
		return err
	}

	stageDir, err := p.buildSnapshotFromHEAD(protectedPathsInfo)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	if err := p.mirrorToWorkingTree(stageDir, protectedPathsInfo); err != nil {
		return err
	}

	if err := p.applySkipWorktreeFlags(protectedPathsInfo); err != nil {
		return err
	}

	fmt.Printf("✅ Path protection completed for %d path(s)\n", protectedPathsInfo.Count())
	return nil
}

// findProtectedPaths discovers paths matching the protection patterns and returns Info for flexible usage
func (p *Processor) findProtectedPaths(protectedFoldersPattern *regex.Processor) (*paths.Info, error) {
	pathsProcessor, err := paths.NewProcessor(p.repositoryRoot, protectedFoldersPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create paths processor: %w", err)
	}

	info, err := pathsProcessor.FindWithOptions(paths.FindOptions{
		IncludeFiles:   true,
		IncludeDirs:    true,
		LogPrefix:      "🔒",
		LogDescription: "protected paths",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find protected paths: %w", err)
	}

	return info, nil
}

// checkUnmergedEntries verifies no merge conflicts exist in protected paths
func (p *Processor) checkUnmergedEntries(protectedPathsInfo *paths.Info) error {
	if protectedPathsInfo.Empty() {
		return nil
	}

	fmt.Printf("  Checking for merge conflicts in protected paths...\n")

	quotedPaths := protectedPathsInfo.QuotedRelativePaths()
	return p.gitOps.CheckUnmergedEntries(quotedPaths)
}

// buildSnapshotFromHEAD creates a staging directory with files from HEAD
//
// This function uses Git's temporary index feature to safely extract files from HEAD
// without disturbing the working directory or the main index. The approach:
//
// 1. Create isolated temporary index file (not Git's main .git/index)
// 2. Populate it with specific paths from HEAD using git read-tree
// 3. Extract files to staging directory using git checkout-index
// 4. Automatic cleanup ensures no temporary index files leak
//
// Why this approach vs. alternatives:
// - git checkout HEAD -- paths: Would modify working directory directly (unsafe)
// - git archive: Cannot handle sparse path patterns reliably
// - git show HEAD:path: Requires individual file handling, complex for directories
// - Temporary index: Atomic, isolated, handles directories/files uniformly
func (p *Processor) buildSnapshotFromHEAD(protectedPathsInfo *paths.Info) (string, error) {
	fmt.Printf("  Building snapshot from HEAD...\n")

	// Create staging directory where we'll extract the clean HEAD version
	stageDir, err := os.MkdirTemp("", stagePrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}

	// Set up cleanup for staging directory in case of early return
	defer func() {
		if err != nil {
			os.RemoveAll(stageDir)
		}
	}()

	if protectedPathsInfo.Empty() {
		return stageDir, nil
	}

	// Use git operations to build snapshot from HEAD
	quotedPaths := protectedPathsInfo.QuotedRelativePaths()
	if err := p.gitOps.BuildSnapshotFromHEAD(quotedPaths, stageDir); err != nil {
		return "", fmt.Errorf("failed to build snapshot from HEAD: %w", err)
	}

	// Set permissions in staging area before atomic sync
	if err := p.setPermissions(stageDir); err != nil {
		return "", fmt.Errorf("failed to set permissions in staging area: %w", err)
	}

	return stageDir, nil
}

// mirrorToWorkingTree syncs the snapshot to working tree with majikmate ownership
func (p *Processor) mirrorToWorkingTree(stageDir string, protectedPathsInfo *paths.Info) error {
	fmt.Printf("  Mirroring to working tree with %s ownership...\n", mmUser)

	if protectedPathsInfo.Empty() {
		return nil
	}

	// Use rsync to atomically sync all protected paths from staging to working tree
	// Permissions are already set in staging area, so rsync will preserve them atomically
	// Source needs trailing slash to sync directory contents, destination should not have trailing slash
	rsyncSource := filepath.Join(stageDir, "") + string(filepath.Separator) // Ensure trailing slash
	rsyncDest := filepath.Clean(p.repositoryRoot)                           // Clean path, no trailing slash
	rsyncCmd := exec.Command("rsync", "-av", "--delete",
		"--no-owner", "--no-group", "--omit-dir-times",
		fmt.Sprintf("--chown=%s", mmOwner),
		rsyncSource, rsyncDest)

	fmt.Printf("    Executing atomic rsync for all protected paths...\n")
	output, err := rsyncCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("atomic rsync failed: %w\nOutput: %s", err, strings.TrimSpace(string(output)))
	}

	fmt.Printf("    ✅ Atomic sync completed for %d protected path(s)\n", protectedPathsInfo.Count())
	return nil
}

// setPermissions sets correct permissions on all files in the staging area
func (p *Processor) setPermissions(stageDir string) error {
	fmt.Printf("    Setting permissions in staging area...\n")

	// Use chmod -R with symbolic mode that preserves executable files:
	// u=rwX,go=rX = user: read+write+execute_if_dir_or_executable
	//               group+other: read+execute_if_dir_or_executable
	// 'X' sets execute permission on:
	//   - Directories (always, for traversal)
	//   - Files that already have execute permission (preserves executables)
	// This results in:
	//   - Directories: 0755 (always executable for traversal)
	//   - Regular files: 0644 (not executable unless they were already)
	//   - Executable files: 0755 (preserve executable status)
	commands := []string{
		fmt.Sprintf("cd '%s'", stageDir),
		"chmod -R u=rwX,go=rX .", // Smart permission setting that preserves executables
	}

	command := strings.Join(commands, " && ")
	if _, err := p.runCommandAsUser(command); err != nil {
		return fmt.Errorf("failed to set permissions in staging area: %w", err)
	}

	return nil
}

// applySkipWorktreeFlags sets skip-worktree flags on all tracked files in protected paths
func (p *Processor) applySkipWorktreeFlags(protectedPathsInfo *paths.Info) error {
	if protectedPathsInfo.Empty() {
		return nil
	}

	fmt.Printf("  Applying skip-worktree flags...\n")

	quotedPaths := protectedPathsInfo.QuotedRelativePaths()
	return p.gitOps.ApplySkipWorktreeFlags(quotedPaths)
}

// runCommandAsUser executes a command as the original user (never root)
// Fails if SUDO_USER is empty or "root" to prevent privilege escalation
func (p *Processor) runCommandAsUser(command string) (string, error) {
	sudoUser := os.Getenv("SUDO_USER")

	// Fail if sudoUser is empty or root - we don't want to run as root
	if sudoUser == "" {
		return "", fmt.Errorf("SUDO_USER environment variable is not set - cannot determine original user")
	}
	if sudoUser == "root" {
		return "", fmt.Errorf("SUDO_USER is 'root' - refusing to run commands as root user")
	}

	cmd := exec.Command("sudo", "-u", sudoUser, "bash", "-lc", command)
	output, err := cmd.CombinedOutput()

	return string(output), err
}
