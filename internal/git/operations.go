package git

import (
	"fmt"
	"os/exec"
	"strings"
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
		"git push --all origin",
		"Atomically push all local branches to remote",
	)
}

// PushBranch pushes a specific branch to remote
func (o *Operations) PushBranch(branchName string) error {
	return o.commander.RunCommand(
		fmt.Sprintf("git push origin %s", branchName),
		fmt.Sprintf("Push branch '%s' to remote", branchName),
	)
}

// MergeBranchToMain merges a specific branch into main
func (o *Operations) MergeBranchToMain(branchName string) error {
	// First switch to main
	if err := o.SwitchToBranch("main"); err != nil {
		return err
	}

	// Merge the branch
	return o.commander.RunCommand(
		fmt.Sprintf("git merge %s --no-ff", branchName),
		fmt.Sprintf("Merge branch '%s' into main", branchName),
	)
}

// UpdateBranchFromMain updates a branch with the latest changes from main
func (o *Operations) UpdateBranchFromMain(branchName string) error {
	// Switch to the branch
	if err := o.SwitchToBranch(branchName); err != nil {
		return err
	}

	// Merge main into this branch
	return o.commander.RunCommand(
		"git merge main --no-ff",
		fmt.Sprintf("Update branch '%s' with latest changes from main", branchName),
	)
}

// PullMainFromRemote pulls the latest changes from remote main
func (o *Operations) PullMainFromRemote() error {
	// Switch to main first
	if err := o.SwitchToBranch("main"); err != nil {
		return err
	}

	// Pull latest changes
	return o.commander.RunCommand(
		"git pull origin main",
		"Pull latest changes from remote main",
	)
}

// GetLocalBranches returns a map of local branch names
func (o *Operations) GetLocalBranches() (map[string]bool, error) {
	branches := make(map[string]bool)

	if o.commander.dryRun {
		fmt.Println("[DRY RUN] Would check local branches with command:")
		fmt.Println("  git branch")
		// Return empty set for dry-run to simulate clean repository
		return branches, nil
	}

	// Get local branches
	output, err := o.commander.RunCommandWithOutput(
		"git branch",
		"Get local branches",
	)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			// Format: "* main" or "  branch-name"
			branchName := strings.TrimSpace(strings.TrimPrefix(line, "*"))
			if branchName != "" {
				branches[branchName] = true
			}
		}
	}

	fmt.Printf("Found %d local branches\n", len(branches))
	return branches, nil
}

// GetRemoteBranches gets list of remote branch names without creating local tracking branches
func (o *Operations) GetRemoteBranches(defaultBranch string) (map[string]bool, error) {
	remoteBranches := make(map[string]bool)

	if o.commander.dryRun {
		fmt.Println("[DRY RUN] Would check remote branches with command:")
		fmt.Println("  git branch -r")
		// Return empty set for dry-run
		return remoteBranches, nil
	}

	// Get list of remote branches
	output, err := o.commander.RunCommandWithOutput(
		"git branch -r",
		"List remote branches",
	)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines, HEAD references, and symbolic references
		if line == "" || strings.HasSuffix(line, "/HEAD") || strings.Contains(line, "HEAD ->") || strings.Contains(line, "->") {
			continue
		}

		// Format: "  origin/branch-name"
		if branchName, ok := strings.CutPrefix(line, "origin/"); ok {
			// Skip default branch and empty names
			if branchName != defaultBranch && branchName != "" {
				remoteBranches[branchName] = true
			}
		}
	}

	fmt.Printf("Found %d remote branches\n", len(remoteBranches))
	return remoteBranches, nil
}

// GetCurrentBranch returns the name of the currently checked out branch
func (o *Operations) GetCurrentBranch() (string, error) {
	return o.commander.RunCommandWithOutput(
		"git rev-parse --abbrev-ref HEAD",
		"Get current branch",
	)
}

// InitSparseCheckout initializes sparse-checkout using modern init command
func (o *Operations) InitSparseCheckout() error {
	return o.commander.RunCommand(
		"git sparse-checkout init",
		"Initialize sparse-checkout",
	)
}

// EnableSparseCheckoutCone enables Git sparse-checkout with cone mode using modern init command
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

// ApplyCheckout applies sparse-checkout changes by reading the tree
func (o *Operations) ApplyCheckout() error {
	return o.commander.RunCommand(
		"git read-tree -m -u HEAD",
		"Apply checkout changes",
	)
}

// IsRepository checks if the current directory is a Git repository
func (o *Operations) IsRepository() (bool, error) {
	_, err := o.commander.RunCommandWithOutput(
		"git rev-parse --git-dir",
		"",
	)
	if err != nil {
		// If the command fails, it's likely not a git repository
		return false, nil
	}
	return true, nil
}

// GetCommitHash returns the current commit hash
func (o *Operations) GetCommitHash() (string, error) {
	return o.commander.RunCommandWithOutput(
		"git rev-parse HEAD",
		"Get commit hash",
	)
}

// GetShortCommitHash returns the short current commit hash
func (o *Operations) GetShortCommitHash() (string, error) {
	return o.commander.RunCommandWithOutput(
		"git rev-parse --short HEAD",
		"Get short commit hash",
	)
}
