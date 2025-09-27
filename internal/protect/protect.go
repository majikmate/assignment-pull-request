package protect

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/majikmate/assignment-pull-request/internal/paths"
	"github.com/majikmate/assignment-pull-request/internal/regex"
)

// Processor handles path protection operations
type Processor struct {
	repositoryRoot string
}

// New creates a new protect processor
func New(repositoryRoot string) *Processor {
	return &Processor{
		repositoryRoot: repositoryRoot,
	}
}

// ProtectPaths scans for folders matching protected folder regex patterns and sets their ownership to user:prot group:prot recursively
func (p *Processor) ProtectPaths(protectedFoldersPattern *regex.Processor) error {
	fmt.Printf("🔒 Starting folder protection...\n")

	// Create paths processor to find matching directories
	pathsProcessor, err := paths.NewProcessor(p.repositoryRoot, protectedFoldersPattern)
	if err != nil {
		return fmt.Errorf("failed to create paths processor: %w", err)
	}

	// Find all matching directories (only directories, not files)
	matchingPaths, err := pathsProcessor.FindPathsWithOptions(paths.FindOptions{
		IncludeFiles:   false, // Only directories
		IncludeDirs:    true,
		LogPrefix:      "🔒",
		LogDescription: "protected folders",
	})
	if err != nil {
		return fmt.Errorf("failed to find protected paths: %w", err)
	}

	if len(matchingPaths) == 0 {
		fmt.Println("No folders match protected patterns")
		return nil
	}

	fmt.Printf("Protecting %d folder(s)...\n", len(matchingPaths))

	// Change ownership of each protected folder recursively
	for _, pathInfo := range matchingPaths {
		fmt.Printf("  Setting ownership for: %s\n", pathInfo.Path)
		
		// Run chown command recursively
		cmd := exec.Command("sudo", "chown", "-R", "prot:prot", pathInfo.Path)
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			fmt.Printf("    Warning: failed to change ownership of %s: %v\n", pathInfo.Path, err)
			if len(output) > 0 {
				fmt.Printf("    Output: %s\n", strings.TrimSpace(string(output)))
			}
			continue // Continue with other folders even if one fails
		}
		
		fmt.Printf("    ✅ Successfully protected: %s\n", pathInfo.Path)
	}

	fmt.Printf("✅ Folder protection completed for %d folder(s)\n", len(matchingPaths))
	return nil
}