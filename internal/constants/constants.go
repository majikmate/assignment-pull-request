package constants

// Default configuration values for the Assignment Pull Request Creator
const (
	// DefaultAssignmentRegex is the default regex pattern for assignment folders with named groups
	DefaultAssignmentRegex = `^(?P<branch>assignment-\d+)$`

	// DefaultBranch is the default branch name for pull requests
	DefaultBranch = "main"

	// DefaultDryRun is the default dry-run mode setting
	DefaultDryRun = "false"

	// ActionName is the name used to identify this action in workflows
	ActionName = "assignment-pull-request"
)

// Environment variable names
const (
	// EnvGitHubToken is the environment variable for GitHub token
	EnvGitHubToken = "GITHUB_TOKEN"

	// EnvGitHubRepository is the environment variable for repository name
	EnvGitHubRepository = "GITHUB_REPOSITORY"

	// EnvAssignmentRegex is the environment variable for assignment regex patterns
	EnvAssignmentRegex = "ASSIGNMENT_REGEX"

	// EnvDefaultBranch is the environment variable for default branch name
	EnvDefaultBranch = "DEFAULT_BRANCH"

	// EnvDryRun is the environment variable for dry-run mode
	EnvDryRun = "DRY_RUN"
)

// Common patterns and values
const (
	// GitHubActionsWorkflowDir is the directory containing GitHub Actions workflows
	GitHubActionsWorkflowDir = ".github/workflows"

	// GitHubWorkflowTemplatesDir is the directory containing GitHub workflow templates
	GitHubWorkflowTemplatesDir = ".github/workflow-templates"

	// SparseCheckoutFile is the path to git sparse-checkout configuration
	SparseCheckoutFile = ".git/info/sparse-checkout"

	// ReadmeFileName is the standard README file name
	ReadmeFileName = "README.md"

	// ReadmeFileName is the standard README file name
	ReadmeFileNameLowerCase = "readme.md"
)

// File extensions and patterns
const (
	// YamlExtension is the YAML file extension
	YamlExtension = ".yml"

	// YamlAltExtension is the alternative YAML file extension
	YamlAltExtension = ".yaml"

	// MarkdownExtension is the Markdown file extension
	MarkdownExtension = ".md"
)

// Repository folders to filter out from sparse-checkout
var FilteredFolders = []string{".git", ".github", ".devcontainer"}

// Workflow file YAML keys
const (
	// WorkflowAssignmentRegexKey is the YAML key for assignment regex patterns
	WorkflowAssignmentRegexKey = "assignment-regex"
	
	// WorkflowProtectedFolderRegexKey is the YAML key for protected folder regex patterns
	WorkflowProtectedFolderRegexKey = "protected-folder-regex"
)
