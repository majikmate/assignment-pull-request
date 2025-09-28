package main

import (
	"fmt"
	"os"

	"github.com/majikmate/assignment-pull-request/internal/protect"
)

func main() {
	// Validate arguments
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Error: Invalid number of arguments\n")
		fmt.Fprintf(os.Stderr, "Usage: %s <source> <destination>\n", os.Args[0])
		os.Exit(1)
	}

	source := os.Args[1]
	dest := os.Args[2]

	// Create rsync wrapper
	wrapper, err := protect.NewRsyncWrapper()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Perform secure sync
	if err := wrapper.SyncDirectory(source, dest); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Sync completed successfully")
}
