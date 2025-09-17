package main

import (
	"fmt"
	"os"

	"github.com/majikmate/assignment-pull-request/internal/creator"
)

func main() {
	prCreator, err := creator.NewFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := prCreator.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
