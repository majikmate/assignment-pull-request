package git

import (
	"fmt"
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
}

// NewOperations creates a new git operations handler
func NewOperations(dryRun bool) *Operations {
	return &Operations{
		commander: NewCommander(dryRun),
	}
}

// Helper function to run git rev-parse commands
func (o *Operations) revParse(args string, description string) (string, error) {
	return o.commander.RunCommandWithOutput(
		fmt.Sprintf("git rev-parse %s", args),
		description,
	)
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
	return o.Push("--all", "Atomically push all local branches to remote")
}

// PushBranch pushes a specific branch to remote
func (o *Operations) PushBranch(branchName string) error {
	return o.Push(branchName, fmt.Sprintf("Push branch '%s' to remote", branchName))
}

// Push provides a consolidated push operation
func (o *Operations) Push(target string, description string) error {
	return o.commander.RunCommand(
		fmt.Sprintf("git push %s %s", DefaultRemote, target),
		description,
	)
}

// MergeBranchToMain merges a specific branch into main
func (o *Operations) MergeBranchToMain(branchName string) error {
	return o.MergeBranchTo(branchName, DefaultBranch)
}

// UpdateBranchFromMain updates a branch with the latest changes from main
func (o *Operations) UpdateBranchFromMain(branchName string) error {
	return o.UpdateBranchFrom(branchName, DefaultBranch)
}

// MergeBranchTo merges a specific branch into target branch
func (o *Operations) MergeBranchTo(sourceBranch, targetBranch string) error {
	// First switch to target branch
	if err := o.SwitchToBranch(targetBranch); err != nil {
		return err
	}

	// Merge the source branch
	return o.commander.RunCommand(
		fmt.Sprintf("git merge %s --no-ff", sourceBranch),
		fmt.Sprintf("Merge branch '%s' into %s", sourceBranch, targetBranch),
	)
}

// UpdateBranchFrom updates a branch with the latest changes from source branch
func (o *Operations) UpdateBranchFrom(targetBranch, sourceBranch string) error {
	// Switch to the target branch
	if err := o.SwitchToBranch(targetBranch); err != nil {
		return err
	}

	// Merge source into target branch
	return o.commander.RunCommand(
		fmt.Sprintf("git merge %s --no-ff", sourceBranch),
		fmt.Sprintf("Update branch '%s' with latest changes from %s", targetBranch, sourceBranch),
	)
}

// PullMainFromRemote pulls the latest changes from remote main
func (o *Operations) PullMainFromRemote() error {
	return o.PullBranchFromRemote(DefaultBranch)
}

// PullBranchFromRemote pulls the latest changes from remote for specified branch
func (o *Operations) PullBranchFromRemote(branchName string) error {
	// Switch to branch first
	if err := o.SwitchToBranch(branchName); err != nil {
		return err
	}

	// Pull latest changes
	return o.commander.RunCommand(
		fmt.Sprintf("git pull %s %s", DefaultRemote, branchName),
		fmt.Sprintf("Pull latest changes from remote %s", branchName),
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
	return o.revParse("--abbrev-ref HEAD", "Get current branch")
}

// InitSparseCheckout initializes sparse-checkout using modern init command
func (o *Operations) InitSparseCheckout() error {
	return o.sparseCheckoutCommand("init", "Initialize sparse-checkout")
}

// InitSparseCheckoutCone enables Git sparse-checkout with cone mode using modern init command
func (o *Operations) InitSparseCheckoutCone() error {
	return o.sparseCheckoutCommand("init --cone", "Initialize sparse-checkout with cone mode")
}

// SetSparseCheckoutPaths sets the sparse-checkout paths using git sparse-checkout command
func (o *Operations) SetSparseCheckoutPaths(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths provided for sparse-checkout")
	}

	// Use git sparse-checkout set command with paths
	pathsStr := strings.Join(paths, " ")
	return o.sparseCheckoutCommand(
		fmt.Sprintf("set %s", pathsStr),
		"Set sparse-checkout paths",
	)
}

// DisableSparseCheckout disables sparse-checkout using modern git command
func (o *Operations) DisableSparseCheckout() error {
	return o.sparseCheckoutCommand("disable", "Disable sparse-checkout")
}

// Helper function for sparse-checkout commands
func (o *Operations) sparseCheckoutCommand(args string, description string) error {
	return o.commander.RunCommand(
		fmt.Sprintf("git sparse-checkout %s", args),
		description,
	)
}

// ApplyCheckout applies sparse-checkout changes by reading the tree
func (o *Operations) ApplyCheckout() error {
	return o.commander.RunCommand(
		"git read-tree -m -u HEAD",
		"Apply checkout changes",
	)
}

// IsRepository checks if the current directory is a Git repository
func (o *Operations) IsRepository() (bool, error) {
	_, err := o.revParse("--git-dir", "")
	if err != nil {
		// If the command fails, it's likely not a git repository
		return false, nil
	}
	return true, nil
}

// GetCommitHash returns the current commit hash
func (o *Operations) GetCommitHash() (string, error) {
	return o.revParse("HEAD", "Get commit hash")
}

// GetShortCommitHash returns the short current commit hash
func (o *Operations) GetShortCommitHash() (string, error) {
	return o.revParse("--short HEAD", "Get short commit hash")
}

// GetRepositoryRoot uses Git to find the top-level repository directory
// This is more reliable than os.Getwd() because Git hooks can be called
// from any subdirectory within the repository
func (o *Operations) GetRepositoryRoot() (string, error) {
	return o.revParse("--show-toplevel", "Get repository root directory")
}

// GetGitDir locates the actual git directory for the repository
// This handles git worktrees, submodules, and other Git configurations
// where .git might not be a directory in the repository root
func (o *Operations) GetGitDir() (string, error) {
	return o.revParse("--git-dir", "Get git directory")
}

// FindRepositoryRoot finds the repository root directory from any subdirectory
// This is a standalone function that doesn't require an Operations instance
func FindRepositoryRoot() (string, error) {
	ops := NewOperations(false)
	return ops.GetRepositoryRoot()
}

// FindGitDir finds the actual git directory from the given repository root
// This is a standalone function that handles worktrees, submodules, etc.
func FindGitDir(repositoryRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repositoryRoot
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to find git directory: %w", err)
	}
	
	gitDir := strings.TrimSpace(string(output))
	
	// If gitDir is relative, make it absolute relative to repositoryRoot
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repositoryRoot, gitDir)
	}
	
	// Clean the path to remove any redundant elements
	return filepath.Clean(gitDir), nil
}
