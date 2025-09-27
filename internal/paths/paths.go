package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/regex"
)

// Info represents a path that matched a pattern
type Info struct {
	Path         string // Absolute path
	RelativePath string // Relative path from root
}

// Result represents the result of a path search operation
type Result struct {
	paths []Info
}

// Paths returns all path info objects
func (r *Result) Paths() []Info {
	return r.paths
}

// AbsolutePaths returns only the absolute paths
func (r *Result) AbsolutePaths() []string {
	var paths []string
	for _, info := range r.paths {
		paths = append(paths, info.Path)
	}
	return paths
}

// RelativePaths returns only the relative paths
func (r *Result) RelativePaths() []string {
	var paths []string
	for _, info := range r.paths {
		paths = append(paths, info.RelativePath)
	}
	return paths
}

// QuotedRelativePaths returns relative paths quoted for shell usage
func (r *Result) QuotedRelativePaths() []string {
	var quoted []string
	for _, info := range r.paths {
		quoted = append(quoted, fmt.Sprintf("'%s'", info.RelativePath))
	}
	return quoted
}

// QuotedAbsolutePaths returns absolute paths quoted for shell usage
func (r *Result) QuotedAbsolutePaths() []string {
	var quoted []string
	for _, info := range r.paths {
		quoted = append(quoted, fmt.Sprintf("'%s'", info.Path))
	}
	return quoted
}

// Count returns the number of matched paths
func (r *Result) Count() int {
	return len(r.paths)
}

// Empty returns true if no paths were found
func (r *Result) Empty() bool {
	return len(r.paths) == 0
}

// Processor handles generic path discovery and processing
type Processor struct {
	root     string
	patterns *regex.Processor
}

// NewProcessor creates a new Processor with regex patterns for scanning from the specified root directory
func NewProcessor(root string, patterns *regex.Processor) (*Processor, error) {
	// Validate that we have at least one pattern
	patternStrings := patterns.Patterns()
	if len(patternStrings) == 0 {
		return nil, fmt.Errorf("no path patterns provided")
	}

	// Validate that patterns can be compiled
	_, err := patterns.Compiled()
	if err != nil {
		return nil, fmt.Errorf("failed to compile path patterns: %w", err)
	}

	return &Processor{
		root:     root,
		patterns: patterns,
	}, nil
}

// Find discovers all paths matching the processor's regex patterns
func (p *Processor) Find() (*Result, error) {
	return p.FindWithOptions(FindOptions{})
}

// FindOptions controls the behavior of path finding
type FindOptions struct {
	// IncludeFiles controls whether files are included in results (default: true)
	IncludeFiles bool
	// IncludeDirs controls whether directories are included in results (default: true) 
	IncludeDirs bool
	// LogPrefix is the prefix used for logging messages (default: "🔍")
	LogPrefix string
	// LogDescription describes what kind of paths are being searched for (default: "paths")
	LogDescription string
}

// FindWithOptions discovers all paths matching the processor's regex patterns with custom options
func (p *Processor) FindWithOptions(opts FindOptions) (*Result, error) {
	// Set defaults
	if !opts.IncludeFiles && !opts.IncludeDirs {
		opts.IncludeFiles = true
		opts.IncludeDirs = true
	}
	if opts.LogPrefix == "" {
		opts.LogPrefix = "🔍"
	}
	if opts.LogDescription == "" {
		opts.LogDescription = "paths"
	}

	fmt.Printf("%s Searching for %s...\n", opts.LogPrefix, opts.LogDescription)
	var matchedPaths []struct {
		absolutePath string
		relativePath string
	}

	// Determine the root directory to walk
	rootDir := p.root
	if rootDir == "" {
		rootDir = "."
	}

	// Get compiled patterns
	compiledPatterns, err := p.patterns.Compiled()
	if err != nil {
		return nil, fmt.Errorf("failed to compile path patterns: %w", err)
	}

	checkedPaths := 0
	matchedCount := 0

	// Walk the entire directory tree and check each path against patterns
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories (but not the current directory ".")
		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, ".") && path != "." && path != rootDir {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the root directory itself
		if path == rootDir {
			return nil
		}

		// Filter by file type if specified
		if info.IsDir() && !opts.IncludeDirs {
			return nil
		}
		if !info.IsDir() && !opts.IncludeFiles {
			return nil
		}

		checkedPaths++

		// Convert absolute path to relative path from root
		relativePath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		// Use the relative path for pattern matching
		relativeNormalizedPath := filepath.ToSlash(relativePath)

		// Check if this path matches any of the patterns
		for _, pattern := range compiledPatterns {
			if pattern.MatchString(relativeNormalizedPath) {
				matchedPaths = append(matchedPaths, struct {
					absolutePath string
					relativePath string
				}{
					absolutePath: path,
					relativePath: relativePath,
				})
				matchedCount++
				break // Don't check other patterns for this path
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error finding %s: %w", opts.LogDescription, err)
	}

	// Sort paths by absolute path for consistent output
	sort.Slice(matchedPaths, func(i, j int) bool {
		return matchedPaths[i].absolutePath < matchedPaths[j].absolutePath
	})

	fmt.Printf("%s Found %d %s (checked %d paths total)\n", opts.LogPrefix, matchedCount, opts.LogDescription, checkedPaths)

	// Convert paths to Info structs and return Result
	var pathInfos []Info
	for _, pathPair := range matchedPaths {
		pathInfos = append(pathInfos, Info{
			Path:         pathPair.absolutePath,
			RelativePath: pathPair.relativePath,
		})
	}

	return &Result{paths: pathInfos}, nil
}

// GetRegexStrings returns the regex patterns as strings
func (p *Processor) GetRegexStrings() []string {
	return p.patterns.Patterns()
}

// IsPathMatched checks if a specific path matches any of the patterns
func (p *Processor) IsPathMatched(checkPath string) (bool, error) {
	// Get compiled patterns
	compiledPatterns, err := p.patterns.Compiled()
	if err != nil {
		return false, fmt.Errorf("failed to compile path patterns: %w", err)
	}

	// Convert to relative path if it's absolute
	var relativePath string
	if filepath.IsAbs(checkPath) {
		relativePath, err = filepath.Rel(p.root, checkPath)
		if err != nil {
			return false, fmt.Errorf("failed to make path relative: %w", err)
		}
	} else {
		relativePath = checkPath
	}

	// Normalize path to use forward slashes for pattern matching
	normalizedPath := filepath.ToSlash(relativePath)

	// Check if this path matches any of the patterns
	for _, pattern := range compiledPatterns {
		if pattern.MatchString(normalizedPath) {
			return true, nil
		}
	}

	return false, nil
}