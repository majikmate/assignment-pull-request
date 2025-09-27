package protect

import (
	"fmt"
	"io/fs"
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
	majikOwner   = "majikmate:majikmate"
	dirMode      = "0755"
	fileMode     = "0644"
	stagePrefix  = "majikmate-protect-sync-stage-"
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
		gitOps:         git.NewOperations(false), // Not in dry-run mode
	}
}

// ProtectPaths implements the protect-sync logic in Go:
// 1. Find protected paths using regex patterns
// 2. Check for unmerged entries under protected paths  
// 3. Extract files from HEAD for protected paths
// 4. Mirror to working tree with majikmate ownership and permissions
// 5. Apply skip-worktree flags
func (p *Processor) ProtectPaths(protectedFoldersPattern *regex.Processor) error {
	fmt.Printf("🔒 Starting path protection (protect-sync logic)...\n")

	// Must be running as majikmate user (called via sudo -u majikmate)
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	if currentUser.Username != "majikmate" {
		return fmt.Errorf("protect-sync must run as majikmate user (via sudo -u majikmate)")
	}

	// Find protected paths using patterns
	protectedPathsResult, err := p.findProtectedPaths(protectedFoldersPattern)
	if err != nil {
		return err
	}

	if protectedPathsResult.Empty() {
		fmt.Println("No paths match protected patterns")
		return nil
	}

	fmt.Printf("Processing %d protected path(s)...\n", protectedPathsResult.Count())

	// Execute the protect-sync workflow
	if err := p.checkUnmergedEntries(protectedPathsResult); err != nil {
		return err
	}

	stageDir, err := p.buildSnapshotFromHEAD(protectedPathsResult)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	if err := p.mirrorToWorkingTree(stageDir, protectedPathsResult); err != nil {
		return err
	}

	if err := p.applySkipWorktreeFlags(protectedPathsResult); err != nil {
		return err
	}

	fmt.Printf("✅ Path protection completed for %d path(s)\n", protectedPathsResult.Count())
	return nil
}

// findProtectedPaths discovers paths matching the protection patterns and returns a Result for flexible usage
func (p *Processor) findProtectedPaths(protectedFoldersPattern *regex.Processor) (*paths.Result, error) {
	pathsProcessor, err := paths.NewProcessor(p.repositoryRoot, protectedFoldersPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create paths processor: %w", err)
	}

	result, err := pathsProcessor.FindWithOptions(paths.FindOptions{
		IncludeFiles:   true,
		IncludeDirs:    true,
		LogPrefix:      "🔒",
		LogDescription: "protected paths",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find protected paths: %w", err)
	}

	return result, nil
}

// checkUnmergedEntries verifies no merge conflicts exist in protected paths
func (p *Processor) checkUnmergedEntries(protectedPathsResult *paths.Result) error {
	if protectedPathsResult.Empty() {
		return nil
	}

	fmt.Printf("  Checking for merge conflicts in protected paths...\n")
	
	quotedPaths := protectedPathsResult.QuotedRelativePaths()
	commands := []string{
		fmt.Sprintf("cd '%s'", p.repositoryRoot),
		fmt.Sprintf("git ls-files -u -- %s", strings.Join(quotedPaths, " ")),
	}
	
	command := strings.Join(commands, " && ")
	output, err := p.runCommandAsUser(command)
	if err != nil {
		return fmt.Errorf("failed to check for unmerged entries: %w", err)
	}

	if strings.TrimSpace(output) != "" {
		return fmt.Errorf("protect-sync: conflicts under protected prefixes — resolve first")
	}

	return nil
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
func (p *Processor) buildSnapshotFromHEAD(protectedPathsResult *paths.Result) (string, error) {
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

	if protectedPathsResult.Empty() {
		return stageDir, nil
	}

	quotedPaths := protectedPathsResult.QuotedRelativePaths()
	commands := []string{
		fmt.Sprintf("cd '%s'", p.repositoryRoot),
		// Create temporary index file (separate from .git/index)
		"TMPIDX=$(mktemp)",
		// Ensure temp index cleanup on shell exit (belts and suspenders)
		"trap 'rm -f \"$TMPIDX\"' EXIT",
		// Populate temp index with specified paths from HEAD commit
		// GIT_INDEX_FILE redirects Git to use our temporary index instead of .git/index
		// read-tree populates the index with tree objects (directories/files) from HEAD
		// The '|| true' handles cases where paths don't exist in HEAD (no error)
		fmt.Sprintf("GIT_INDEX_FILE=\"$TMPIDX\" git read-tree HEAD -- %s 2>/dev/null || true", strings.Join(quotedPaths, " ")),
		// Check if our temp index actually contains any files (read-tree succeeded)
		"if GIT_INDEX_FILE=\"$TMPIDX\" git ls-files -z | grep -q .; then",
		// Extract all files from temp index to staging directory
		// checkout-index -a = all files in index, --prefix adds directory prefix
		// This creates the actual file content in stageDir/ matching the index structure
		fmt.Sprintf("  GIT_INDEX_FILE=\"$TMPIDX\" git checkout-index -a --prefix='%s/' >/dev/null", stageDir),
		"fi",
		// temp index file is automatically cleaned up by trap on command completion
	}

	command := strings.Join(commands, " && ")
	if _, err = p.runCommandAsUser(command); err != nil {
		return "", fmt.Errorf("failed to build snapshot from HEAD: %w", err)
	}

	return stageDir, nil
}

// mirrorToWorkingTree syncs the snapshot to working tree with majikmate ownership
func (p *Processor) mirrorToWorkingTree(stageDir string, protectedPathsResult *paths.Result) error {
	fmt.Printf("  Mirroring to working tree with majikmate ownership...\n")

	for _, protectedPath := range protectedPathsResult.RelativePaths() {
		if err := p.syncPath(stageDir, protectedPath); err != nil {
			fmt.Printf("    Warning: failed to sync %s: %v\n", protectedPath, err)
		}
	}

	return nil
}

// syncPath handles individual path synchronization
func (p *Processor) syncPath(stageDir, protectedPath string) error {
	srcPath := filepath.Join(stageDir, protectedPath)
	dstPath := filepath.Join(p.repositoryRoot, protectedPath)

	// Handle absent paths from HEAD
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		if _, err := os.Stat(dstPath); err == nil {
			fmt.Printf("    Removing absent path: %s\n", protectedPath)
			return os.RemoveAll(dstPath)
		}
		return nil
	}

	fmt.Printf("    Processing: %s\n", protectedPath)

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Use rsync for reliable synchronization
	rsyncCmd := exec.Command("rsync", "-a", "--delete",
		"--no-perms", "--no-owner", "--no-group", "--omit-dir-times",
		fmt.Sprintf("--chown=%s", majikOwner),
		srcPath+"/", dstPath+"/")

	if output, err := rsyncCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rsync failed: %w\nOutput: %s", err, strings.TrimSpace(string(output)))
	}

	// Set proper permissions
	return p.setPermissions(dstPath)
}

// setPermissions applies the correct file and directory permissions
func (p *Processor) setPermissions(rootPath string) error {
	return filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		mode := fileMode
		if d.IsDir() {
			mode = dirMode
		}

		cmd := exec.Command("chmod", mode, path)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("chmod failed for %s: %w\nOutput: %s", path, err, string(output))
		}

		return nil
	})
}

// applySkipWorktreeFlags sets skip-worktree flags on all tracked files in protected paths
func (p *Processor) applySkipWorktreeFlags(protectedPathsResult *paths.Result) error {
	if protectedPathsResult.Empty() {
		return nil
	}

	fmt.Printf("  Applying skip-worktree flags...\n")

	commands := []string{fmt.Sprintf("cd '%s'", p.repositoryRoot)}
	for _, path := range protectedPathsResult.RelativePaths() {
		commands = append(commands,
			fmt.Sprintf("git ls-files -z -- '%s' | xargs -0 -r git update-index --skip-worktree", path))
	}

	command := strings.Join(commands, " && ")
	if _, err := p.runCommandAsUser(command); err != nil {
		return fmt.Errorf("failed to apply skip-worktree flags: %w", err)
	}

	return nil
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