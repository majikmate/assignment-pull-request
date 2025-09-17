package creator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/assignment"
	"github.com/majikmate/assignment-pull-request/internal/constants"
	"github.com/majikmate/assignment-pull-request/internal/git"
	"github.com/majikmate/assignment-pull-request/internal/github"
	"github.com/majikmate/assignment-pull-request/internal/instructions"
	"github.com/majikmate/assignment-pull-request/internal/regex"
)

// PullRequestInfo holds information about a created pull request
type PullRequestInfo struct {
	Number string `json:"number"`
	Title  string `json:"title"`
}

// Config holds configuration for the PR creator
type Config struct {
	gitHubToken       string
	assignmentPattern *regex.Processor
	repositoryName    string
	defaultBranch     string
	dryRun            bool
}

// NewConfig creates a new Config with the given parameters
func NewConfig(gitHubToken, repositoryName, defaultBranch string, assignmentRegex []string, dryRun bool) *Config {
	return &Config{
		gitHubToken:       gitHubToken,
		repositoryName:    repositoryName,
		defaultBranch:     defaultBranch,
		assignmentPattern: regex.NewWithPatterns(assignmentRegex),
		dryRun:            dryRun,
	}
}

// NewConfigFromEnv creates a new Config from environment variables
func NewConfigFromEnv() *Config {
	return &Config{
		gitHubToken:       os.Getenv(constants.EnvGitHubToken),
		repositoryName:    os.Getenv(constants.EnvGitHubRepository),
		defaultBranch:     getEnvWithDefault(constants.EnvDefaultBranch, constants.DefaultBranch),
		assignmentPattern: regex.NewFromNewlineSeparated(getEnvWithDefault(constants.EnvAssignmentRegex, constants.DefaultAssignmentRegex)),
		dryRun:            isDryRun(getEnvWithDefault(constants.EnvDryRun, constants.DefaultDryRun)),
	}
}

// Creator is the main Assignment PR Creator
type Creator struct {
	config              *Config
	gitOps              *git.Operations
	githubClient        *github.Client
	assignmentProcessor *assignment.Processor
	createdBranches     []string
	createdPullRequests []PullRequestInfo
	pendingPushes       []string
}

// NewWithConfig creates a new Assignment PR Creator with the given configuration
func NewWithConfig(config *Config) (*Creator, error) {
	if config.gitHubToken == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}
	if config.repositoryName == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY environment variable is required")
	}

	// Create assignment processor with pattern processors from config
	assignmentProc, err := assignment.NewProcessor("", config.assignmentPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create assignment processor: %w", err)
	}

	creator := &Creator{
		config:              config,
		gitOps:              git.NewOperations(config.dryRun),
		githubClient:        github.NewClient(config.gitHubToken, config.repositoryName, config.dryRun),
		assignmentProcessor: assignmentProc,
		createdBranches:     make([]string, 0),
		createdPullRequests: make([]PullRequestInfo, 0),
	}

	return creator, nil
}

// NewFromEnv creates a new Assignment PR Creator with environment variables
func NewFromEnv() (*Creator, error) {
	config := NewConfigFromEnv()
	return NewWithConfig(config)
}

// createBranch creates a new branch from the default branch locally
func (c *Creator) createBranch(branchName string) error {
	// First, ensure we're on the default branch
	if err := c.gitOps.SwitchToBranch(c.config.defaultBranch); err != nil {
		return err
	}

	// Create and switch to new branch
	if err := c.gitOps.CreateAndSwitchToBranch(branchName); err != nil {
		return err
	}

	fmt.Printf("✅ Created branch: %s (local)\n", branchName)
	c.createdBranches = append(c.createdBranches, branchName)
	c.pendingPushes = append(c.pendingPushes, branchName)
	return nil
}

// createReadme creates or augments README.md file in the assignment folder locally
func (c *Creator) createReadme(assignmentPath string) error {
	readmePath := filepath.Join(assignmentPath, constants.ReadmeFileName)

	// Create assignment directory if it doesn't exist
	if !c.config.dryRun {
		if err := os.MkdirAll(assignmentPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", assignmentPath, err)
		}
	} else {
		fmt.Printf("[DRY RUN] Would create directory: mkdir -p %s\n", assignmentPath)
	}

	// Create processor for content generation
	instructionsProcessor := instructions.NewWithDefaults(c.config.defaultBranch, assignmentPath)

	// Check if README already exists
	var readmeContent string
	if _, err := os.Stat(readmePath); err == nil {
		fmt.Printf("README already exists at %s, augmenting...\n", readmePath)

		// Read existing content
		existingBytes, err := os.ReadFile(readmePath)
		if err != nil {
			return fmt.Errorf("failed to read existing README: %w", err)
		}
		existingContent := string(existingBytes)

		// Use processor to augment content
		readmeContent = instructionsProcessor.AugmentExistingReadmeContent(existingContent)

		if !c.config.dryRun {
			if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
				return fmt.Errorf("failed to write augmented README: %w", err)
			}
			fmt.Printf("✅ Augmented %s at %s (local)\n", constants.ReadmeFileName, readmePath)
		} else {
			fmt.Printf("[DRY RUN] Would augment README at %s\n", readmePath)
		}
	} else {
		// Use processor to create new README content
		readmeContent = instructionsProcessor.CreateNewReadmeContent()

		// Write new README
		if !c.config.dryRun {
			if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
				return fmt.Errorf("failed to write new README: %w", err)
			}
			fmt.Printf("✅ Created %s at %s (local)\n", constants.ReadmeFileName, readmePath)
		} else {
			fmt.Printf("[DRY RUN] Would create README at %s\n", readmePath)
		}
	}

	// Add and commit the README
	if err := c.gitOps.AddFile(readmePath); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add README for assignment %s", assignmentPath)
	if _, err := os.Stat(readmePath); err == nil && !c.config.dryRun {
		commitMessage = fmt.Sprintf("Augment README for assignment %s", assignmentPath)
	}

	return c.gitOps.Commit(commitMessage)
}

// createPullRequest creates a pull request for the assignment branch using GitHub API
func (c *Creator) createPullRequest(assignmentPath, branchName string) error {
	title := branchName

	// Try to read README.md file for PR body content
	body, err := c.createPullRequestBody(assignmentPath)
	if err != nil {
		return fmt.Errorf("error creating pull request body for '%s': %w", assignmentPath, err)
	}

	prNumber, err := c.githubClient.CreatePullRequest(title, body, branchName, c.config.defaultBranch)
	if err != nil {
		return fmt.Errorf("error creating pull request for '%s': %w", assignmentPath, err)
	}

	c.createdPullRequests = append(c.createdPullRequests, PullRequestInfo{
		Number: prNumber,
		Title:  title,
	})

	// Add PR link to README after the branch has been pushed
	if err := c.addPullRequestLink(assignmentPath, branchName, prNumber); err != nil {
		fmt.Printf("Warning: failed to add PR link after push for %s: %v\n", prNumber, err)
		// Continue even if PR link addition fails
	}

	fmt.Printf("✅ Created and updated PR %s with PR link in README\n", prNumber)

	return nil
}

// addPullRequestLink adds PR link to README after the branch has been pushed
func (c *Creator) addPullRequestLink(assignmentPath, branchName, prNumber string) error {
	// First, switch to the correct branch
	if err := c.gitOps.SwitchToBranch(branchName); err != nil {
		return fmt.Errorf("failed to switch to branch %s: %w", branchName, err)
	}

	// Add PR link to the top of the README
	if err := c.addPullRequestLinkToReadme(assignmentPath, branchName, prNumber); err != nil {
		fmt.Printf("Warning: failed to add PR link to README: %v\n", err)
		return err
	}

	// Push only the specific branch to avoid conflicts with main
	if err := c.gitOps.PushBranch(branchName); err != nil {
		fmt.Printf("Warning: failed to push branch %s with PR link update: %v\n", branchName, err)
		return err
	}

	return nil
}

// createPullRequestBody creates the pull request body content using the instructions processor
func (c *Creator) createPullRequestBody(assignmentPath string) (string, error) {
	instructionsProcessor := instructions.NewWithDefaults(c.config.defaultBranch, assignmentPath)
	return instructionsProcessor.CreatePullRequestBody()
}

// addPullRequestLinkToReadme adds a link to the pull request at the top of the README file
func (c *Creator) addPullRequestLinkToReadme(assignmentPath, branchName, prNumber string) error {
	readmePath := filepath.Join(assignmentPath, constants.ReadmeFileName)

	// Check if README exists
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		return fmt.Errorf("README file does not exist at %s", readmePath)
	}

	// Read current README content
	currentBytes, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("failed to read README file: %w", err)
	}
	currentContent := string(currentBytes)

	// Use processor to add PR link
	instructionsProcessor := instructions.NewWithDefaults(c.config.defaultBranch, assignmentPath)
	updatedContent := instructionsProcessor.AddPullRequestLinkToReadme(currentContent, c.config.repositoryName, branchName, prNumber)

	// Write updated content back to file
	if !c.config.dryRun {
		if err := os.WriteFile(readmePath, []byte(updatedContent), 0644); err != nil {
			return fmt.Errorf("failed to write updated README: %w", err)
		}
		fmt.Printf("✅ Added PR link %s to README at %s\n", prNumber, readmePath)

		// Add and commit the updated README
		if err := c.gitOps.AddFile(readmePath); err != nil {
			return fmt.Errorf("failed to add updated README to git: %w", err)
		}

		commitMessage := fmt.Sprintf("Add pull request link %s to README", prNumber)
		if err := c.gitOps.Commit(commitMessage); err != nil {
			return fmt.Errorf("failed to commit updated README: %w", err)
		}
	} else {
		fmt.Printf("[DRY RUN] Would add PR link %s to README at %s\n", prNumber, readmePath)
	}

	return nil
}

// processAssignments processes all found assignments and creates branches/PRs as needed
func (c *Creator) processAssignments() error {
	fmt.Printf("Looking for assignments matching '%s'\n", c.assignmentProcessor.GetAssignmentRegexStrings())

	// Use assignment processor to discover and validate assignments
	assignments, err := c.assignmentProcessor.ProcessAssignments()
	if err != nil {
		return err
	}

	// No assignments found matching the criteria
	if len(assignments) == 0 {
		fmt.Println("⏭️ No assignments found matching the criteria")
		return nil
	}

	// First, ensure we're on the default branch
	if err := c.gitOps.SwitchToBranch(c.config.defaultBranch); err != nil {
		return err
	}

	// Phase 0: Sync with remote from clean state
	fmt.Println("\n=== Phase 0: Syncing with remote ===")

	// Fetch all remote branches to ensure complete local state
	if err := c.gitOps.FetchAll(); err != nil {
		fmt.Println("❌ Failed to fetch remote branches, aborting")
		return err
	}

	// Phase 1: Get current state after sync
	localBranches, err := c.gitOps.GetLocalBranches()
	if err != nil {
		fmt.Println("❌ Failed to get local branches")
		return err
	}

	// Get remote branches
	remoteBranches, err := c.gitOps.GetRemoteBranches(c.config.defaultBranch)
	if err != nil {
		fmt.Println("❌ Failed to get remote branches")
		return err
	}

	existingPRs, err := c.githubClient.GetExistingPullRequests()
	if err != nil {
		fmt.Println("❌ Failed to get existing pull requests")
		return err
	}

	fmt.Printf("Found %d assignments to process\n", len(assignments))
	fmt.Printf("Existing local branches: %d\n", len(localBranches))
	fmt.Printf("Existing remote branches: %d\n", len(remoteBranches))
	fmt.Printf("Existing PRs: %d\n", len(existingPRs))

	// Phase 2: Process all assignments locally
	fmt.Println("\n=== Phase 2: Local processing ===")

	prNeedsCreation := false

	for _, assignmentInfo := range assignments {
		assignmentPath := assignmentInfo.Path
		branchName := assignmentInfo.BranchName

		fmt.Printf("\nProcessing assignment: %s\n", assignmentPath)
		fmt.Printf("Branch name: %s\n", branchName)

		// Check if branch exists locally, remotely, and if PR exists (or has ever existed)
		_, localBranchExists := localBranches[branchName]
		_, remoteBranchExists := remoteBranches[branchName]
		_, prExists := existingPRs[branchName]

		// Only create branch if:
		// 1. Branch doesn't exist locally AND
		// 2. Branch doesn't exist remotely AND
		// 3. No PR has ever existed for this assignment
		if !localBranchExists && !remoteBranchExists && !prExists {
			fmt.Printf("Branch '%s' doesn't exist anywhere and no PR exists, creating branch...\n", branchName)

			// Create branch locally
			if err := c.createBranch(branchName); err != nil {
				fmt.Printf("❌ Failed to create branch '%s', skipping: %v\n", branchName, err)
				continue
			}

			// Create README content locally
			fmt.Printf("Creating README content for assignment '%s'...\n", assignmentPath)
			if err := c.createReadme(assignmentPath); err != nil {
				fmt.Printf("❌ Failed to create README for '%s', skipping: %v\n", assignmentPath, err)
				continue
			}
		}

		// Track if any PRs need creation
		if !prExists {
			prNeedsCreation = true
		} else {
			fmt.Printf("PR already exists for branch '%s', skipping\n", branchName)
		}
	}

	// Phase 3: Push all changes atomically to remote
	if len(c.pendingPushes) > 0 {
		fmt.Printf("\n=== Phase 3: Atomic push to remote ===\n")
		fmt.Printf("Pushing all local branches (including %d new branches) to remote atomically...\n", len(c.pendingPushes))

		if err := c.gitOps.PushAllBranches(); err != nil {
			fmt.Println("❌ Failed to push branches to remote, aborting PR creation")
			return err
		}

		fmt.Printf("✅ Successfully pushed all local branches to remote atomically\n")
		c.pendingPushes = c.pendingPushes[:0] // Clear the slice
	}

	// Phase 4: Create pull requests
	if prNeedsCreation {
		fmt.Printf("\n=== Phase 4: Pull request creation ===\n")

		for _, assignmentInfo := range assignments {
			assignmentPath := assignmentInfo.Path
			branchName := assignmentInfo.BranchName

			// Only create PR if no pull request exists for this branch name
			if _, prExists := existingPRs[branchName]; !prExists {
				fmt.Printf("Creating pull request for branch '%s'...\n", branchName)
				if err := c.createPullRequest(assignmentPath, branchName); err != nil {
					fmt.Printf("❌ Failed to create PR for '%s': %v\n", branchName, err)
					continue
				}
			}
		}
	} else {
		fmt.Println("\n=== No new assignments to process ===")
		fmt.Println("All assignments already have PRs")
	}

	return nil
}

// setOutputs sets GitHub Actions outputs
func (c *Creator) setOutputs() error {
	// Set outputs for GitHub Actions
	if githubOutput := os.Getenv("GITHUB_OUTPUT"); githubOutput != "" {
		file, err := os.OpenFile(githubOutput, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open GITHUB_OUTPUT file: %w", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Printf("Warning: failed to close output file: %v\n", err)
			}
		}()

		branchesJSON, err := json.Marshal(c.createdBranches)
		if err != nil {
			return fmt.Errorf("failed to marshal created branches: %w", err)
		}

		prsJSON, err := json.Marshal(c.createdPullRequests)
		if err != nil {
			return fmt.Errorf("failed to marshal created pull requests: %w", err)
		}

		if _, err := fmt.Fprintf(file, "created-branches=%s\n", branchesJSON); err != nil {
			return fmt.Errorf("failed to write created-branches output: %w", err)
		}

		if _, err := fmt.Fprintf(file, "created-pull-requests=%s\n", prsJSON); err != nil {
			return fmt.Errorf("failed to write created-pull-requests output: %w", err)
		}
	}

	// Format output with each item on separate lines
	fmt.Println("\nSummary:")

	// Print each created branch on its own line
	fmt.Printf("Created branches (%d):\n", len(c.createdBranches))
	if len(c.createdBranches) > 0 {
		for _, branch := range c.createdBranches {
			fmt.Printf("  - %s\n", branch)
		}
	} else {
		fmt.Println("  none")
	}

	// Print each created PR on its own line with title
	fmt.Printf("Created pull requests (%d):\n", len(c.createdPullRequests))
	if len(c.createdPullRequests) > 0 {
		for _, pr := range c.createdPullRequests {
			fmt.Printf("  - %s: %s\n", pr.Number, pr.Title)
		}
	} else {
		fmt.Println("  none")
	}

	return nil
}

// Run is the main execution method using local git with atomic remote operations
func (c *Creator) Run() error {
	fmt.Println("Starting Assignment Pull Request Creator")
	if c.config.dryRun {
		fmt.Println("🏃 DRY RUN MODE: Simulating local git operations without making actual changes")
	} else {
		fmt.Println("🔄 LIVE MODE: Using local git operations with atomic remote push")
	}
	fmt.Printf("Repository: %s\n", c.config.repositoryName)
	fmt.Printf("Assignment regex: %s\n", c.config.assignmentPattern.Patterns())
	fmt.Printf("Default branch: %s\n", c.config.defaultBranch)
	fmt.Printf("Dry run mode: %t\n", c.config.dryRun)

	if err := c.processAssignments(); err != nil {
		return err
	}

	if err := c.setOutputs(); err != nil {
		return err
	}

	if c.config.dryRun {
		fmt.Println("\n🏃 DRY RUN MODE: Assignment Pull Request Creator simulation completed")
		fmt.Println("In real mode, all local changes would be pushed atomically to remote")
	} else {
		fmt.Println("\nAssignment Pull Request Creator completed successfully")
		fmt.Println("All changes have been pushed to remote repository")
	}

	return nil
}

// getEnvWithDefault returns the environment variable value or a default value if not set
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// isDryRun checks if dry run mode is enabled
func isDryRun(dryRunStr string) bool {
	return strings.ToLower(dryRunStr) == "true" || dryRunStr == "1" || strings.ToLower(dryRunStr) == "yes"
}
