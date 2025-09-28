package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Common constants
const (
	DefaultRemote = "origin"
	DefaultBranch = "main"
)

// Commander handles git command execution
type Commander struct {
	dryRun bool
}

// NewCommander creates a new git commander
func NewCommander(dryRun bool) *Commander {
	return &Commander{dryRun: dryRun}
}

// RunCommand runs a git command, either for real or simulate in dry-run mode
func (c *Commander) RunCommand(command, description string) error {
	if c.dryRun {
		fmt.Printf("[DRY RUN] %s: %s\n", description, command)
		return nil
	}

	if description != "" {
		fmt.Printf("%s: %s\n", description, command)
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("error running command '%s': %w\nOutput: %s", command, err, string(output))
	}

	if len(output) > 0 {
		fmt.Printf("  Output: %s\n", strings.TrimSpace(string(output)))
	}

	return nil
}

// RunCommandWithOutput runs a git command and returns its output
func (c *Commander) RunCommandWithOutput(command, description string) (string, error) {
	if c.dryRun {
		fmt.Printf("[DRY RUN] %s: %s\n", description, command)
		return "", nil // Return empty string for dry-run
	}

	if description != "" {
		fmt.Printf("%s: %s\n", description, command)
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("error running command '%s': %w\nStderr: %s", command, err, string(exitError.Stderr))
		}
		return "", fmt.Errorf("error running command '%s': %w", command, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// Operations provides higher-level git operations
type Operations struct {
	commander *Commander
	workDir   string // Optional working directory for git commands
}

// NewOperations creates a new git operations handler
func NewOperations(dryRun bool) *Operations {
	return &Operations{
		commander: NewCommander(dryRun),
		workDir:   "", // Use current directory
	}
}

// NewOperationsWithDir creates a new git operations handler with specific working directory
func NewOperationsWithDir(dryRun bool, workDir string) *Operations {
	return &Operations{
		commander: NewCommander(dryRun),
		workDir:   workDir,
	}
}

// Helper function to parse branch listing output
func (o *Operations) parseBranchList(output string, isRemote bool, excludeBranch string) map[string]bool {
	branches := make(map[string]bool)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var branchName string
		if isRemote {
			// Skip HEAD references and symbolic references
			if strings.HasSuffix(line, "/HEAD") || strings.Contains(line, "HEAD ->") || strings.Contains(line, "->") {
				continue
			}
			// Format: "  origin/branch-name"
			if name, ok := strings.CutPrefix(line, DefaultRemote+"/"); ok {
				branchName = name
			}
		} else {
			// Format: "* main" or "  branch-name"
			branchName = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		}

		// Add branch if it's valid and not excluded
		if branchName != "" && branchName != excludeBranch {
			branches[branchName] = true
		}
	}

	return branches
}

// SwitchToBranch switches to the specified branch
func (o *Operations) SwitchToBranch(branchName string) error {
	return o.commander.RunCommand(
		fmt.Sprintf("git checkout %s", branchName),
		fmt.Sprintf("Switch to branch '%s'", branchName),
	)
}

// CreateAndSwitchToBranch creates a new branch and switches to it
func (o *Operations) CreateAndSwitchToBranch(branchName string) error {
	return o.commander.RunCommand(
		fmt.Sprintf("git checkout -b %s", branchName),
		fmt.Sprintf("Create and switch to branch '%s'", branchName),
	)
}

// AddFile stages a file for commit
func (o *Operations) AddFile(filePath string) error {
	return o.commander.RunCommand(
		fmt.Sprintf("git add %s", filePath),
		"Stage file",
	)
}

// Commit creates a commit with the specified message
func (o *Operations) Commit(message string) error {
	return o.commander.RunCommand(
		fmt.Sprintf(`git commit -m "%s"`, message),
		"Commit changes",
	)
}

// FetchAll fetches all remote branches and tags
func (o *Operations) FetchAll() error {
	return o.commander.RunCommand(
		"git fetch --all",
		"Fetch all remote branches and tags",
	)
}

// PushAllBranches pushes all local branches to remote
func (o *Operations) PushAllBranches() error {
	return o.commander.RunCommand(
		fmt.Sprintf("git push %s --all", DefaultRemote),
		"Atomically push all local branches to remote",
	)
}

// PushBranch pushes a specific branch to remote
func (o *Operations) PushBranch(branchName string) error {
	return o.commander.RunCommand(
		fmt.Sprintf("git push %s %s", DefaultRemote, branchName),
		fmt.Sprintf("Push branch '%s' to remote", branchName),
	)
}

// GetLocalBranches returns a map of local branch names
func (o *Operations) GetLocalBranches() (map[string]bool, error) {
	if o.commander.dryRun {
		fmt.Println("[DRY RUN] Would check local branches with command:")
		fmt.Println("  git branch")
		// Return empty set for dry-run to simulate clean repository
		return make(map[string]bool), nil
	}

	// Get local branches
	output, err := o.commander.RunCommandWithOutput(
		"git branch",
		"Get local branches",
	)
	if err != nil {
		return nil, err
	}

	branches := o.parseBranchList(output, false, "")
	fmt.Printf("Found %d local branches\n", len(branches))
	return branches, nil
}

// GetRemoteBranches gets list of remote branch names without creating local tracking branches
func (o *Operations) GetRemoteBranches(defaultBranch string) (map[string]bool, error) {
	if o.commander.dryRun {
		fmt.Println("[DRY RUN] Would check remote branches with command:")
		fmt.Println("  git branch -r")
		// Return empty set for dry-run
		return make(map[string]bool), nil
	}

	// Get list of remote branches
	output, err := o.commander.RunCommandWithOutput(
		"git branch -r",
		"List remote branches",
	)
	if err != nil {
		return nil, err
	}

	branches := o.parseBranchList(output, true, defaultBranch)
	fmt.Printf("Found %d remote branches\n", len(branches))
	return branches, nil
}

// GetCurrentBranch returns the name of the currently checked out branch
func (o *Operations) GetCurrentBranch() (string, error) {
	return o.runCommandInContext("git rev-parse --abbrev-ref HEAD", "Get current branch")
}

// InitSparseCheckout initializes sparse-checkout using modern init command
func (o *Operations) InitSparseCheckout() error {
	return o.commander.RunCommand(
		"git sparse-checkout init",
		"Initialize sparse-checkout",
	)
}

// InitSparseCheckoutCone enables Git sparse-checkout with cone mode using modern init command
func (o *Operations) InitSparseCheckoutCone() error {
	return o.commander.RunCommand(
		"git sparse-checkout init --cone",
		"Initialize sparse-checkout with cone mode",
	)
}

// SetSparseCheckoutPaths sets the sparse-checkout paths using git sparse-checkout command
func (o *Operations) SetSparseCheckoutPaths(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths provided for sparse-checkout")
	}

	// Use git sparse-checkout set command with paths
	pathsStr := strings.Join(paths, " ")
	return o.commander.RunCommand(
		fmt.Sprintf("git sparse-checkout set %s", pathsStr),
		"Set sparse-checkout paths",
	)
}

// DisableSparseCheckout disables sparse-checkout using modern git command
func (o *Operations) DisableSparseCheckout() error {
	return o.commander.RunCommand(
		"git sparse-checkout disable",
		"Disable sparse-checkout",
	)
}

// GetRepositoryRoot uses Git to find the top-level repository directory
// This is more reliable than os.Getwd() because Git hooks can be called
// from any subdirectory within the repository
func (o *Operations) GetRepositoryRoot() (string, error) {
	return o.runCommandInContext("git rev-parse --show-toplevel", "Get repository root directory")
}

// GetGitDir locates the actual git directory for the repository
// This handles git worktrees, submodules, and other Git configurations
// where .git might not be a directory in the repository root
func (o *Operations) GetGitDir() (string, error) {
	return o.runCommandInContext("git rev-parse --git-dir", "Get git directory")
}

// FindGitDir finds the actual git directory, handling worktrees, submodules, etc.
func (o *Operations) FindGitDir() (string, error) {
	gitDir, err := o.GetGitDir()
	if err != nil {
		return "", fmt.Errorf("failed to find git directory: %w", err)
	}

	// If gitDir is relative and we have a working directory, make it absolute
	if o.workDir != "" && !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(o.workDir, gitDir)
	}

	// Clean the path to remove any redundant elements
	gitDir = filepath.Clean(gitDir)

	// Verify the git directory exists and is accessible
	if _, err := os.Stat(gitDir); err != nil {
		return "", fmt.Errorf("git directory not accessible: %w", err)
	}

	// Verify it's actually a git directory by checking for essential git files/directories
	essentialPaths := []string{
		"HEAD",    // Required: points to current branch
		"refs",    // Required: directory containing references
		"objects", // Required: directory containing git objects
	}

	for _, path := range essentialPaths {
		fullPath := filepath.Join(gitDir, path)
		if _, err := os.Stat(fullPath); err != nil {
			return "", fmt.Errorf("missing essential git component '%s': %w", path, err)
		}
	}

	return gitDir, nil
}

// CheckUnmergedEntries checks for merge conflicts in the specified paths
func (o *Operations) CheckUnmergedEntries(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	pathsStr := strings.Join(paths, " ")
	command := fmt.Sprintf("git ls-files -u -- %s", pathsStr)

	output, err := o.runCommandInContext(command, "Check for unmerged entries")
	if err != nil {
		return fmt.Errorf("failed to check for unmerged entries: %w", err)
	}

	if strings.TrimSpace(output) != "" {
		return fmt.Errorf("conflicts found in protected paths - resolve first")
	}

	return nil
}

// BuildSnapshotFromHEAD creates a staging directory with files from HEAD using temporary index
func (o *Operations) BuildSnapshotFromHEAD(paths []string, stageDir string) error {
	if len(paths) == 0 {
		return nil
	}

	pathsStr := strings.Join(paths, " ")
	// Create temporary index, populate with HEAD, then checkout files under specific paths
	// Use --ignore-skip-worktree-bits to checkout files even if they have skip-worktree flags
	command := fmt.Sprintf(`TMPIDX=$(mktemp) && trap 'rm -f "$TMPIDX"' EXIT && GIT_INDEX_FILE="$TMPIDX" git read-tree HEAD && if GIT_INDEX_FILE="$TMPIDX" git ls-files -z -- %s | head -c1 | grep -q .; then GIT_INDEX_FILE="$TMPIDX" git ls-files -z -- %s | xargs -0 -r git checkout-index --ignore-skip-worktree-bits --prefix='%s/' >/dev/null; fi`, pathsStr, pathsStr, stageDir)

	_, err := o.runCommandInContext(command, "Build snapshot from HEAD")
	return err
}

// ApplySkipWorktreeFlags applies skip-worktree flags to tracked files in specified paths
func (o *Operations) ApplySkipWorktreeFlags(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	pathsStr := strings.Join(paths, " ")
	command := fmt.Sprintf("git ls-files -z -- %s | xargs -0 -r git update-index --skip-worktree", pathsStr)
	_, err := o.runCommandInContext(command, "Apply skip-worktree flags")
	return err
}

// Helper to run commands with working directory context
func (o *Operations) runCommandInContext(command, description string) (string, error) {
	if o.workDir != "" {
		// Use exec.Command directly when we need to set working directory
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = o.workDir

		if o.commander.dryRun {
			if description != "" {
				fmt.Printf("[DRY RUN] %s: %s (in %s)\n", description, command, o.workDir)
			}
			return "", nil
		}

		if description != "" {
			fmt.Printf("%s: %s (in %s)\n", description, command, o.workDir)
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("error running command '%s': %w\nOutput: %s", command, err, string(output))
		}

		return strings.TrimSpace(string(output)), nil
	}

	// Use commander for current directory operations
	return o.commander.RunCommandWithOutput(command, description)
}
