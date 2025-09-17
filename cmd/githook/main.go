package main

import (
	"log"
	"os"

	"github.com/majikmate/assignment-pull-request/internal/checkout"
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

	// Create sparse checkout processor and configure sparse-checkout
	checkoutProcessor := checkout.New(repositoryRoot)
	err = checkoutProcessor.SparseCheckout()
	if err != nil {
		log.Printf("Failed to configure sparse checkout: %v", err)
	}
}
