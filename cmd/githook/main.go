package main

import (
	"fmt"
	"log"
	"os"

	"github.com/majikmate/assignment-pull-request/internal/checkout"
	"github.com/majikmate/assignment-pull-request/internal/protect"
	"github.com/majikmate/assignment-pull-request/internal/workflow"
)

func main() {
	// Determine the git hook type and repository root
	hookType, repositoryRoot, err := determineHookContext()
	if err != nil {
		log.Printf("Failed to determine hook context: %v", err)
		return
	}

	log.Printf("Processing %s hook in repository: %s", hookType, repositoryRoot)

	// Parse workflow files to find assignment and protected paths configurations
	log.Printf("Parsing workflow files for patterns...")
	workflowProcessor := workflow.New()
	err = workflowProcessor.ParseAllFiles()
	if err != nil {
		log.Printf("Failed to parse workflow files: %v", err)
		return // Don't continue if workflow parsing fails
	}

	// Get pattern processors from workflow
	assignmentPattern := workflowProcessor.AssignmentPattern()
	protectedPathsPattern := workflowProcessor.ProtectedPathsPattern()

	// Handle sparse checkout only for post-checkout with branch checkout
	if shouldProcessSparseCheckout(hookType) {
		if len(assignmentPattern.Patterns()) > 0 {
			log.Printf("Configuring sparse checkout with assignment patterns...")
			
			// Create sparse checkout processor
			checkoutProcessor := checkout.New(repositoryRoot)
			err = checkoutProcessor.SparseCheckout(assignmentPattern)
			if err != nil {
				log.Printf("Failed to configure sparse checkout: %v", err)
			}
		} else {
			log.Printf("No assignment patterns found, skipping sparse-checkout configuration")
		}
	}

	// Handle path protection for all hooks that modify working tree
	if shouldProcessProtectedPaths(hookType) {
		if len(protectedPathsPattern.Patterns()) > 0 {
			log.Printf("Protecting paths with protected paths patterns...")
			
			// Create protect processor
			protectProcessor := protect.New(repositoryRoot)
			err = protectProcessor.ProtectPaths(protectedPathsPattern)
			if err != nil {
				log.Printf("Failed to protect paths: %v", err)
			}
		} else {
			log.Printf("No protected paths patterns found, skipping path protection")
		}
	}
}

// determineHookContext determines the git hook type and repository root
func determineHookContext() (string, string, error) {
	// Get repository root
	repositoryRoot, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Determine hook type from program arguments or environment
	if len(os.Args) >= 2 {
		hookType := os.Args[1]
		return hookType, repositoryRoot, nil
	}

	// Fallback: try to determine from program name or environment
	return "unknown", repositoryRoot, nil
}

// shouldProcessSparseCheckout determines if sparse checkout should be processed for this hook
func shouldProcessSparseCheckout(hookType string) bool {
	// Only process sparse checkout for post-checkout with branch checkout
	if hookType != "post-checkout" {
		return false
	}
	
	// Check if this is a branch checkout (argument 3 should be "1")
	if len(os.Args) >= 4 && os.Args[3] == "1" {
		return true
	}
	
	return false
}

// shouldProcessProtectedPaths determines if path protection should be processed for this hook
func shouldProcessProtectedPaths(hookType string) bool {
	// Process protected paths for all hooks that modify the working tree
	workingTreeModifyingHooks := []string{
		"post-checkout",
		"post-merge", 
		"post-rewrite",
		"post-applypatch",
		"post-commit",
		"post-reset",
	}
	
	for _, hook := range workingTreeModifyingHooks {
		if hookType == hook {
			return true
		}
	}
	
	return false
}
