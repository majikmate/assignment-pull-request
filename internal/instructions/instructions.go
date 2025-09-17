package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/constants"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Processor handles reading and processing instruction files for a specific assignment
type Processor struct {
	branch         string
	assignmentPath string
}

// New creates a new instructions processor for the given assignment path
func New(assignmentPath string) *Processor {
	return &Processor{
		branch:         "main", // Default fallback
		assignmentPath: assignmentPath,
	}
}

// NewWithDefaults creates a new instructions processor with branch and assignment path
func NewWithDefaults(branch, assignmentPath string) *Processor {
	return &Processor{
		branch:         branch,
		assignmentPath: assignmentPath,
	}
}

// CreatePullRequestBody creates pull request body content from the processor's assignment path
func (p *Processor) CreatePullRequestBody() (string, error) {
	// Try to find README.md in the assignment directory
	readmePath := p.findReadmeFile()

	if readmePath != "" {
		content, err := p.readAndProcessReadme(readmePath)
		if err != nil {
			fmt.Printf("Warning: failed to read README file '%s': %v\n", readmePath, err)
			fmt.Printf("Falling back to generic template\n")
		} else {
			return content, nil
		}
	}

	// Fall back to generic template
	return p.createGenericPullRequestBody(), nil
}

// findReadmeFile looks for README.md in the assignment directory
func (p *Processor) findReadmeFile() string {
	candidates := []string{
		filepath.Join(p.assignmentPath, constants.ReadmeFileName),
		filepath.Join(p.assignmentPath, constants.ReadmeFileNameLowerCase),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// readAndProcessReadme reads the README file and processes image links
func (p *Processor) readAndProcessReadme(readmePath string) (string, error) {
	// Read the README file content
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return "", fmt.Errorf("failed to read README file: %w", err)
	}

	// Process image links in the content
	processedContent := p.rewriteImageLinks(string(content))

	return processedContent, nil
}

// rewriteImageLinks rewrites relative image links to reference the assignment path
func (p *Processor) rewriteImageLinks(content string) string {
	// Regex to match markdown image syntax: ![alt text](relative/path/to/image)
	// Note: This handles standard paths; escaped parentheses in paths are extremely rare
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	return imageRegex.ReplaceAllStringFunc(content, func(match string) string {
		submatches := imageRegex.FindStringSubmatch(match)
		if len(submatches) != 3 {
			return match // Return original if parsing fails
		}

		altText := submatches[1]
		imagePath := submatches[2]

		// Skip if it's already an absolute URL
		if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
			return match
		}

		// Skip if it's already an absolute path from repo root (Unix-style in markdown)
		if strings.HasPrefix(imagePath, "/") {
			return match
		}

		// Check if it's an absolute path (cross-platform)
		if filepath.IsAbs(imagePath) {
			return match
		}

		// Rewrite relative path for GitHub pull requests and issues
		// Join the assignment path with the relative image path
		rewrittenPath := filepath.Join(p.assignmentPath, imagePath)
		// Ensure we use forward slashes for GitHub compatibility
		rewrittenPath = filepath.ToSlash(rewrittenPath)

		// For pull requests and issues, use blob URL format with ?raw=true
		// This ensures images display correctly in PR descriptions
		rewrittenPath = fmt.Sprintf("../blob/%s/%s?raw=true", p.branch, rewrittenPath)

		return fmt.Sprintf("![%s](%s)", altText, rewrittenPath)
	})
}

// createGenericPullRequestBody creates the default generic pull request body
func (p *Processor) createGenericPullRequestBody() string {
	return fmt.Sprintf(`## Assignment Pull Request

This pull request contains the setup for the assignment located at
`+"`%s`"+`.

### Changes included:
- ✅ Created `+constants.ReadmeFileName+` with assignment template
- ✅ Set up branch structure for assignment submission

### Next steps:
1. Review the assignment requirements in the `+constants.ReadmeFileName+`
2. Add any additional assignment materials
3. Students can fork this repository and work on their submissions

---
<sub>*Created by the [Assignment Pull Request](https://github.com/majikmate/assignment-pull-request) action.*</sub>
`, p.assignmentPath)
}

// CreateNewReadmeContent creates content for a new README file
func (p *Processor) CreateNewReadmeContent() string {
	caser := cases.Title(language.English)
	assignmentTitle := caser.String(strings.ReplaceAll(p.assignmentPath, string(filepath.Separator), " - "))

	return fmt.Sprintf(`# %s

This is the README for the assignment located at `+"`%s`"+`.

## Instructions

Please add your assignment instructions and requirements here.

## Submission

Please add your submission guidelines here.

---
<sub>*Generated by the [Assignment Pull Request](https://github.com/majikmate/assignment-pull-request) action.*</sub>
`, assignmentTitle, p.assignmentPath)
}

// AugmentExistingReadmeContent adds augmentation content to existing README content
func (p *Processor) AugmentExistingReadmeContent(existingContent string) string {
	augmentationComment := `

---
<sub>*Augmented by the [Assignment Pull Request](https://github.com/majikmate/assignment-pull-request) action.*</sub>
`
	return strings.TrimSpace(existingContent) + augmentationComment
}

// AddPullRequestLinkToReadme adds a pull request link to the top of README content
func (p *Processor) AddPullRequestLinkToReadme(content, repositoryName, branchName, prNumber string) string {
	// Create the PR link banner
	prLink := fmt.Sprintf("https://github.com/%s/pull/%s", repositoryName, strings.TrimPrefix(prNumber, "#"))
	prBanner := fmt.Sprintf(`# Overview

Please continue work on [Pull Request %s: %s](%s)

`, prNumber, branchName, prLink)

	// Add the banner at the very top of the content
	return prBanner + content
}
