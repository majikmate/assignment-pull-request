package assignment

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/regex"
)

// Info represents an assignment with its path and generated branch name
type Info struct {
	Path       string
	BranchName string
}

// Processor handles assignment discovery and processing
type Processor struct {
	repositoryRoot    string
	assignmentPattern *regex.Processor
}

// NewProcessor creates a new Processor with assignment regex patterns
func NewProcessor(repositoryRoot string, assignmentProcessor *regex.Processor) (*Processor, error) {
	// Get compiled patterns to validate assignment patterns have capturing groups
	assignmentPatterns, err := assignmentProcessor.Compiled()
	if err != nil {
		return nil, fmt.Errorf("failed to compile assignment patterns: %w", err)
	}

	// Validate that assignment patterns have capturing groups
	for _, pattern := range assignmentPatterns {
		if !hasCapturingGroups(pattern) {
			return nil, fmt.Errorf("assignment regex '%s' must contain at least one capturing group (e.g., (?P<name>...) or (...)) to extract branch names", pattern.String())
		}
	}

	return &Processor{
		repositoryRoot:    repositoryRoot,
		assignmentPattern: assignmentProcessor,
	}, nil
}

// ProcessAssignments discovers all assignments and returns assignment info with unique branch names
func (ap *Processor) ProcessAssignments() ([]Info, error) {
	// Find all assignment paths
	assignments, err := ap.findAssignments()
	if err != nil {
		return nil, fmt.Errorf("error finding assignments: %w", err)
	}

	if len(assignments) == 0 {
		return nil, nil
	}

	var results []Info
	branchCounts := make(map[string]int)

	for _, assignment := range assignments {
		branchName, found := ap.extractBranchNameFromPath(assignment)
		if found {
			branchCounts[branchName]++
			uniqueBranchName := branchName
			if branchCounts[branchName] > 1 {
				uniqueBranchName = fmt.Sprintf("%s-%d", branchName, branchCounts[branchName])
			}
			results = append(results, Info{
				Path:       assignment,
				BranchName: uniqueBranchName,
			})
		}
	}

	// Validate branch name uniqueness before returning
	if err := ap.validateBranchNameUniqueness(results); err != nil {
		return nil, err
	}

	return results, nil
}

// validateBranchNameUniqueness checks that all assignments generate unique branch names
func (ap *Processor) validateBranchNameUniqueness(assignments []Info) error {
	branchToAssignments := make(map[string][]string)

	// Collect all branch names and track which assignments generate them
	for _, assignment := range assignments {
		branchToAssignments[assignment.BranchName] = append(branchToAssignments[assignment.BranchName], assignment.Path)
	}

	// Check for duplicates
	var conflicts []string
	for branchName, assignmentPaths := range branchToAssignments {
		if len(assignmentPaths) > 1 {
			conflicts = append(conflicts, fmt.Sprintf("Branch '%s' would be created by multiple assignments: %v", branchName, assignmentPaths))
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("branch name conflicts detected:\n  %s", strings.Join(conflicts, "\n  "))
	}

	return nil
}

// findAssignments finds all assignment folders matching the processor's regex patterns
func (ap *Processor) findAssignments() ([]string, error) {
	fmt.Printf("📁 Searching for assignment folders...\n")
	var assignments []string

	// Determine the root directory to walk
	rootDir := ap.repositoryRoot
	if rootDir == "" {
		rootDir = "."
	}

	// Get compiled patterns for debugging
	assignmentPatterns, err := ap.assignmentPattern.Compiled()
	if err != nil {
		return nil, fmt.Errorf("failed to compile assignment patterns: %w", err)
	}

	checkedDirs := 0
	matchedDirs := 0

	// Walk the entire directory tree and check each directory against assignment patterns
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files (but not the current directory ".")
		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, ".") && path != "." && path != rootDir {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process directories
		if !info.IsDir() {
			return nil
		}

		// Skip the root directory itself
		if path == rootDir {
			return nil
		}

		checkedDirs++

		// Convert absolute path to relative path from repository root
		relativePath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		// Use the relative path for pattern matching
		relativeNormalizedPath := filepath.ToSlash(relativePath)

		for _, assignmentPattern := range assignmentPatterns {
			if assignmentPattern.MatchString(relativeNormalizedPath) {
				assignments = append(assignments, path)
				matchedDirs++
				break // Don't check other patterns for this path
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error finding assignments: %w", err)
	}

	// Sort assignments
	sort.Strings(assignments)

	return assignments, nil
}

// GetAssignmentRegexStrings returns the assignment regex patterns as strings
func (ap *Processor) GetAssignmentRegexStrings() []string {
	return ap.assignmentPattern.Patterns()
}

// extractBranchNameFromPath extracts a branch name from a path using the processor's compiled patterns
func (ap *Processor) extractBranchNameFromPath(assignmentPath string) (string, bool) {

	assignmentPatterns, err := ap.assignmentPattern.Compiled()
	if err != nil {
		fmt.Printf("    Error: Failed to compile patterns: %v\n", err)
		return "", false
	}

	// Convert absolute path to relative path from repository root
	relativePath, err := filepath.Rel(ap.repositoryRoot, assignmentPath)
	if err != nil {
		fmt.Printf("    Error: Could not make path relative: %v\n", err)
		return "", false
	}

	// Normalize path to use forward slashes for pattern matching
	normalizedPath := filepath.ToSlash(relativePath)

	for _, pattern := range assignmentPatterns {
		if pattern == nil {
			continue
		}

		matches := pattern.FindStringSubmatch(normalizedPath)
		if matches != nil {
			names := pattern.SubexpNames()
			var branchParts []string

			// Collect named groups and their values, sorted alphabetically by name
			namedGroups := make(map[string]string)
			var namedGroupNames []string

			for i, name := range names {
				if name != "" && i < len(matches) && matches[i] != "" {
					part := strings.TrimSpace(matches[i])
					if part != "" {
						namedGroups[name] = part
						namedGroupNames = append(namedGroupNames, name)
					}
				}
			}

			// Sort named group names alphabetically
			if len(namedGroupNames) > 0 {
				sort.Strings(namedGroupNames)
				// Add named groups in alphabetical order
				for _, name := range namedGroupNames {
					branchParts = append(branchParts, namedGroups[name])
				}
			}

			// Collect unnamed groups in order of appearance
			var unnamedParts []string
			for i := 1; i < len(matches); i++ { // Skip index 0 (full match)
				// Skip if this index corresponds to a named group
				isNamedGroup := false
				if i < len(names) && names[i] != "" {
					isNamedGroup = true
				}

				if !isNamedGroup && matches[i] != "" {
					part := strings.TrimSpace(matches[i])
					if part != "" {
						unnamedParts = append(unnamedParts, part)
					}
				}
			}

			// Add unnamed groups after named groups
			branchParts = append(branchParts, unnamedParts...)

			if len(branchParts) == 0 {
				continue
			}

			// Combine parts and sanitize
			branchName := strings.Join(branchParts, "-")
			branchName = ap.sanitizeBranchName(branchName)

			return branchName, true
		}
	}

	return "", false
}

// hasCapturingGroups checks if a compiled regex pattern has at least one capturing group (named or unnamed)
func hasCapturingGroups(regex *regexp.Regexp) bool {
	names := regex.SubexpNames()
	// SubexpNames() returns a slice where the first element is always an empty string
	// for the entire match. If there are more elements, there are capturing groups
	return len(names) > 1
}

// sanitizeBranchName sanitizes a branch name to match Creator's original behavior
// Only sanitizes spaces and slashes, preserves other special characters
func (ap *Processor) sanitizeBranchName(name string) string {
	// Remove leading/trailing whitespace
	branchName := strings.TrimSpace(name)

	// Replace spaces with hyphens
	branchName = regexp.MustCompile(`\s+`).ReplaceAllString(branchName, "-")

	// Remove slashes
	branchName = strings.ReplaceAll(branchName, "/", "-")

	// Remove consecutive hyphens
	branchName = regexp.MustCompile(`-+`).ReplaceAllString(branchName, "-")

	// Convert to lowercase
	branchName = strings.ToLower(branchName)

	// Remove leading/trailing hyphens
	branchName = strings.Trim(branchName, "-")

	return branchName
}
