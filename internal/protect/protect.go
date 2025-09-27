package protect

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/git"
	"github.com/majikmate/assignment-pull-request/internal/paths"
	"github.com/majikmate/assignment-pull-request/internal/regex"
)

const (
	rootOwner    = "root:root"
	dirMode      = "0755"
	fileMode     = "0644"
	stagePrefix  = "protect-sync-stage-"
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
// 4. Mirror to working tree with root ownership and permissions
// 5. Apply skip-worktree flags
func (p *Processor) ProtectPaths(protectedFoldersPattern *regex.Processor) error {
	fmt.Printf("🔒 Starting path protection (protect-sync logic)...\n")

	// Must be running as root (called via sudo)
	if os.Geteuid() != 0 {
		return fmt.Errorf("protect-sync must run as root (via sudo)")
	}

	// Find protected paths using patterns
	protectedPaths, err := p.findProtectedPaths(protectedFoldersPattern)
	if err != nil {
		return err
	}

	if len(protectedPaths) == 0 {
		fmt.Println("No paths match protected patterns")
		return nil
	}

	fmt.Printf("Processing %d protected path(s)...\n", len(protectedPaths))

	// Execute the protect-sync workflow
	if err := p.checkUnmergedEntries(protectedPaths); err != nil {
		return err
	}

	stageDir, err := p.buildSnapshotFromHEAD(protectedPaths)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	if err := p.mirrorToWorkingTree(stageDir, protectedPaths); err != nil {
		return err
	}

	if err := p.applySkipWorktreeFlags(protectedPaths); err != nil {
		return err
	}

	fmt.Printf("✅ Path protection completed for %d path(s)\n", len(protectedPaths))
	return nil
}

// findProtectedPaths discovers paths matching the protection patterns
func (p *Processor) findProtectedPaths(protectedFoldersPattern *regex.Processor) ([]string, error) {
	pathsProcessor, err := paths.NewProcessor(p.repositoryRoot, protectedFoldersPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create paths processor: %w", err)
	}

	matchingPaths, err := pathsProcessor.FindPathsWithOptions(paths.FindOptions{
		IncludeFiles:   true,
		IncludeDirs:    true,
		LogPrefix:      "🔒",
		LogDescription: "protected paths",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find protected paths: %w", err)
	}

	// Extract relative paths for git operations
	var protectedPaths []string
	for _, pathInfo := range matchingPaths {
		protectedPaths = append(protectedPaths, pathInfo.RelativePath)
	}

	return protectedPaths, nil
}

// checkUnmergedEntries verifies no merge conflicts exist in protected paths
func (p *Processor) checkUnmergedEntries(protectedPaths []string) error {
	if len(protectedPaths) == 0 {
		return nil
	}

	fmt.Printf("  Checking for merge conflicts in protected paths...\n")
	
	pathArgs := p.quotePathsForShell(protectedPaths)
	command := fmt.Sprintf("cd '%s' && git ls-files -u -- %s", p.repositoryRoot, strings.Join(pathArgs, " "))
	
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
func (p *Processor) buildSnapshotFromHEAD(protectedPaths []string) (string, error) {
	fmt.Printf("  Building snapshot from HEAD...\n")

	stageDir, err := os.MkdirTemp("", stagePrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}

	if len(protectedPaths) == 0 {
		return stageDir, nil
	}

	pathArgs := p.quotePathsForShell(protectedPaths)
	commands := []string{
		fmt.Sprintf("cd '%s'", p.repositoryRoot),
		"TMPIDX=$(mktemp)",
		"trap 'rm -f \"$TMPIDX\"' EXIT",
		fmt.Sprintf("GIT_INDEX_FILE=\"$TMPIDX\" git read-tree HEAD -- %s 2>/dev/null || true", strings.Join(pathArgs, " ")),
		"if GIT_INDEX_FILE=\"$TMPIDX\" git ls-files -z | grep -q .; then",
		fmt.Sprintf("  GIT_INDEX_FILE=\"$TMPIDX\" git checkout-index -a --prefix='%s/' >/dev/null", stageDir),
		"fi",
	}

	command := strings.Join(commands, " && ")
	if _, err := p.runCommandAsUser(command); err != nil {
		os.RemoveAll(stageDir)
		return "", fmt.Errorf("failed to build snapshot from HEAD: %w", err)
	}

	return stageDir, nil
}

// mirrorToWorkingTree syncs the snapshot to working tree with root ownership
func (p *Processor) mirrorToWorkingTree(stageDir string, protectedPaths []string) error {
	fmt.Printf("  Mirroring to working tree with root ownership...\n")

	for _, protectedPath := range protectedPaths {
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
		fmt.Sprintf("--chown=%s", rootOwner),
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
func (p *Processor) applySkipWorktreeFlags(protectedPaths []string) error {
	if len(protectedPaths) == 0 {
		return nil
	}

	fmt.Printf("  Applying skip-worktree flags...\n")

	commands := []string{fmt.Sprintf("cd '%s'", p.repositoryRoot)}
	for _, path := range protectedPaths {
		commands = append(commands,
			fmt.Sprintf("git ls-files -z -- '%s' | xargs -0 -r git update-index --skip-worktree", path))
	}

	command := strings.Join(commands, " && ")
	if _, err := p.runCommandAsUser(command); err != nil {
		return fmt.Errorf("failed to apply skip-worktree flags: %w", err)
	}

	return nil
}

// quotePathsForShell safely quotes paths for shell commands
func (p *Processor) quotePathsForShell(paths []string) []string {
	var quoted []string
	for _, path := range paths {
		quoted = append(quoted, fmt.Sprintf("'%s'", path))
	}
	return quoted
}

// runCommandAsUser executes a command as the original user (not root)
func (p *Processor) runCommandAsUser(command string) (string, error) {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		sudoUser = "root"
	}

	cmd := exec.Command("sudo", "-u", sudoUser, "bash", "-lc", command)
	output, err := cmd.CombinedOutput()
	
	return string(output), err
}