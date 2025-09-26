package main

import (
	"log"
	"os"

	"github.com/majikmate/assignment-pull-request/internal/checkout"
	"github.com/majikmate/assignment-pull-request/internal/workflow"
)

func main() {
	// Check if this is a post-checkout hook call
	if len(os.Args) < 4 {
		log.Fatal("Usage: post-checkout <old-ref> <new-ref> <branch-checkout-flag>")
	}

	branchCheckout := os.Args[3]

	// Only process branch checkouts
	if branchCheckout != "1" {
		return
	}

	// Get repository root (current working directory)
	repositoryRoot, err := os.Getwd()
	if err != nil {
		log.Printf("Failed to get current working directory: %v", err)
		return
	}

	// Parse workflow files to find assignment and protected folder configurations
	log.Printf("Parsing workflow files for patterns...")
	workflowProcessor := workflow.New()
	err = workflowProcessor.ParseAllFiles()
	if err != nil {
		log.Printf("Failed to parse workflow files: %v", err)
		return // Don't continue if workflow parsing fails
	}

	// Get pattern processors from workflow
	assignmentPattern := workflowProcessor.AssignmentPattern()
	protectedFoldersPattern := workflowProcessor.ProtectedFoldersPattern()

	// Create sparse checkout processor
	checkoutProcessor := checkout.New(repositoryRoot)

	// Configure sparse-checkout with assignment patterns
	if len(assignmentPattern.Patterns()) > 0 {
		log.Printf("Configuring sparse checkout with assignment patterns...")
		err = checkoutProcessor.SparseCheckout(assignmentPattern)
		if err != nil {
			log.Printf("Failed to configure sparse checkout: %v", err)
		}
	} else {
		log.Printf("No assignment patterns found, skipping sparse-checkout configuration")
	}

	// Protect folders with protected folder patterns
	if len(protectedFoldersPattern.Patterns()) > 0 {
		log.Printf("Protecting folders with protected folder patterns...")
		err = checkoutProcessor.ProtectFolders(protectedFoldersPattern)
		if err != nil {
			log.Printf("Failed to protect folders: %v", err)
		}
	} else {
		log.Printf("No protected folder patterns found, skipping folder protection")
	}
}
