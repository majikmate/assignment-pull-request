package checkout

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/assignment"
	"github.com/majikmate/assignment-pull-request/internal/constants"
	"github.com/majikmate/assignment-pull-request/internal/git"
	"github.com/majikmate/assignment-pull-request/internal/workflow"
)

// Processor handles Git sparse-checkout configuration based on assignment patterns
type Processor struct {
	repositoryRoot string
	gitOps         *git.Operations
}

// New creates a new sparse checkout processor
func New(repositoryRoot string) *Processor {
	return &Processor{
		repositoryRoot: repositoryRoot,
		gitOps:         git.NewOperations(false), // Not in dry-run mode
	}
}

// NewWithGitOps creates a new sparse checkout processor with custom git operations
func NewWithGitOps(repositoryRoot string, gitOps *git.Operations) *Processor {
	return &Processor{
		repositoryRoot: repositoryRoot,
		gitOps:         gitOps,
	}
}

// SparseCheckout configures Git sparse-checkout for assignments matching the current branch
// Automatically discovers workflow patterns, finds matching assignments, and sets up sparse-checkout
// to include all non-assignment root folders plus only the assignment folders that match the current branch
func (p *Processor) SparseCheckout() error {
	fmt.Printf("🔍 Starting sparse-checkout configuration...\n")

	// Check if git is initialized

	if _, err := os.Stat(filepath.Join(p.repositoryRoot, ".git")); os.IsNotExist(err) {
		return nil
	}

	// Print current working directory before change
	// ...existing code...

	// Disable sparse-checkout at the very beginning to reset state

	if err := p.gitOps.DisableSparseCheckout(); err != nil {
		// Ignore error if sparse-checkout wasn't enabled
		fmt.Printf("Warning: could not disable sparse-checkout (may not be enabled): %v\n", err)
	}

	// Parse workflow files to find assignment configurations

	workflowProcessor := workflow.New()
	err := workflowProcessor.ParseAllFiles()
	if err != nil {
		fmt.Printf("Failed to parse workflow files: %v\n", err)
		return nil // Don't fail, just skip sparse-checkout configuration
	}

	// Get pattern processors from workflow
	assignmentPattern := workflowProcessor.AssignmentPattern()

	for i, pattern := range assignmentPattern.Patterns() {
		fmt.Printf("  Pattern %d: %s\n", i+1, pattern)
	}

	// Skip operations if no patterns found
	if len(assignmentPattern.Patterns()) == 0 {
		fmt.Println("No assignment patterns found in workflow files, skipping sparse-checkout configuration")
		return nil
	}

	// Create assignment processor

	assignmentProcessor, err := assignment.NewProcessor(p.repositoryRoot, assignmentPattern)
	if err != nil {
		return fmt.Errorf("failed to create assignment processor: %w", err)
	}

	// Get current branch
	currentBranch, err := p.getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get all assignments to identify which root folders contain assignments
	allAssignments, err := assignmentProcessor.ProcessAssignments()
	if err != nil {
		return fmt.Errorf("failed to process assignments: %w", err)
	}

	// Get matching assignments for current branch
	assignmentPaths, err := p.getMatchingAssignments(currentBranch, allAssignments)
	if err != nil {
		return fmt.Errorf("failed to get matching assignments: %w", err)
	}

	if len(assignmentPaths) == 0 {
		fmt.Printf("No assignment folders match current branch '%s'\n", currentBranch)
		return nil
	}

	fmt.Printf("Found %d matching assignment folder(s) for current branch '%s'\n", len(assignmentPaths), currentBranch)
	for _, assignmentFolder := range assignmentPaths {
		fmt.Printf("  - %s\n", assignmentFolder)
	}

	// Scan repository root folders
	rootFolders, err := p.scanRepositoryRootFolders()
	if err != nil {
		return fmt.Errorf("failed to scan repository root folders: %w", err)
	}

	// Create a map of root folders that contain assignments for quick lookup
	assignmentRootFoldersMap := make(map[string]bool)
	for _, assignment := range allAssignments {
		if assignment.Path != "" {
			// Convert absolute path to relative path first
			relativePath, err := filepath.Rel(p.repositoryRoot, assignment.Path)
			if err != nil {
				fmt.Printf("Warning: could not make assignment path relative: %s\n", assignment.Path)
				continue
			}

			// Extract root folder from relative assignment path (e.g., "test/fixtures/labs/lab-1" -> "test")
			normalizedPath := filepath.ToSlash(relativePath)
			pathParts := strings.Split(normalizedPath, "/")
			if len(pathParts) > 0 {
				rootFolder := pathParts[0]
				if rootFolder != "" {
					assignmentRootFoldersMap[rootFolder] = true
				}
			}
		}
	}

	// Create initial paths list (empty, will be populated with root folders and matching assignments)
	paths := []string{}

	// Add only non-assignment root folders to the sparse-checkout paths
	for _, rootFolder := range rootFolders {
		if !assignmentRootFoldersMap[rootFolder] {
			paths = append(paths, rootFolder)
			fmt.Printf("  + %s (non-assignment root)\n", rootFolder)
		} else {
			fmt.Printf("  - %s (assignment root, excluding - only specific assignments will be included)\n", rootFolder)
		}
	}

	// Add only the assignment folders that match the current branch
	for _, path := range assignmentPaths {
		// Convert absolute path to relative path for sparse-checkout
		relativePath, err := filepath.Rel(p.repositoryRoot, path)
		if err != nil {
			fmt.Printf("Warning: could not make path relative: %s\n", path)
			continue
		}
		normalizedPath := filepath.ToSlash(relativePath)
		paths = append(paths, normalizedPath)
		fmt.Printf("  + %s (matching assignment)\n", normalizedPath)
	}

	// Enable sparse-checkout with cone mode for better performance
	if err := p.gitOps.InitSparseCheckoutCone(); err != nil {
		return fmt.Errorf("failed to enable sparse-checkout with cone mode: %w", err)
	}

	// Configure sparse-checkout with the computed paths
	err = p.gitOps.SetSparseCheckoutPaths(paths)
	if err != nil {
		return fmt.Errorf("failed to configure sparse checkout: %w", err)
	}

	fmt.Printf("✅ Sparse checkout configured successfully for %d assignment folder(s)\n", len(assignmentPaths))
	return nil
}

// getCurrentBranch returns the name of the currently checked out branch
func (p *Processor) getCurrentBranch() (string, error) {
	return p.gitOps.GetCurrentBranch()
}

// getMatchingAssignments returns the assignment paths that match the current branch
func (p *Processor) getMatchingAssignments(branch string, allAssignments []assignment.Info) ([]string, error) {
	// Filter assignments matching the current branch
	var matchingAssignments []string
	for _, assignmentInfo := range allAssignments {
		if assignmentInfo.BranchName == branch {
			matchingAssignments = append(matchingAssignments, assignmentInfo.Path)
		}
	}

	return matchingAssignments, nil
}

// scanRepositoryRootFolders scans the repository root directory and returns all folder names
func (p *Processor) scanRepositoryRootFolders() ([]string, error) {
	entries, err := os.ReadDir(p.repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read repository root directory: %w", err)
	}

	var folders []string
	for _, entry := range entries {
		if entry.IsDir() && !isFilteredFolder(entry.Name()) {
			// Skip filtered folders like .git, .github, .devcontainer
			folders = append(folders, entry.Name())
		}
	}

	return folders, nil
}

// isFilteredFolder returns true if the folder should be filtered out from sparse-checkout
func isFilteredFolder(folderName string) bool {
	return slices.Contains(constants.FilteredFolders, folderName)
}
