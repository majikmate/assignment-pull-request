package userutil

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// GetCurrentUser determines the current user using the most reliable method available
// This is a centralized implementation that handles all user detection scenarios:
// - Standard os/user package (most reliable)
// - Environment variables (USER, LOGNAME)
// - Shell command fallback (whoami)
// - Containerized environment fallback (vscode)
func GetCurrentUser() (string, error) {
	// First try os/user package (most reliable)
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username, nil
	}

	// Fallback to USER environment variable
	if username := os.Getenv("USER"); username != "" {
		return username, nil
	}

	// Fallback to LOGNAME environment variable (POSIX standard)
	if username := os.Getenv("LOGNAME"); username != "" {
		return username, nil
	}

	// Fallback to whoami command (handles edge cases where env vars are missing)
	if cmd := exec.Command("whoami"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			if username := strings.TrimSpace(string(output)); username != "" {
				return username, nil
			}
		}
	}

	// Final fallback for containerized environments
	return "vscode", nil
}

// GetRealUser gets the real user, considering SUDO_USER environment variable
// This handles the common pattern where code needs to determine the original user
// when running under sudo. Returns the SUDO_USER if set, otherwise the current user.
func GetRealUser() (string, error) {
	// Check SUDO_USER first (original user before sudo escalation)
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser, nil
	}

	// Fall back to current user
	return GetCurrentUser()
}

// ValidateUser ensures the user is not root and returns an appropriate error if it is
func ValidateUser(username string) error {
	if username == "root" {
		return fmt.Errorf("refusing to operate as root user")
	}
	return nil
}

// GetValidatedCurrentUser gets the current user and validates it's not root
func GetValidatedCurrentUser() (string, error) {
	username, err := GetCurrentUser()
	if err != nil {
		return "", fmt.Errorf("failed to determine current user: %w", err)
	}

	if err := ValidateUser(username); err != nil {
		return "", err
	}

	return username, nil
}

// GetValidatedRealUser gets the real user (considering SUDO_USER) and validates it's not root
func GetValidatedRealUser() (string, error) {
	username, err := GetRealUser()
	if err != nil {
		return "", fmt.Errorf("failed to determine real user: %w", err)
	}

	if err := ValidateUser(username); err != nil {
		return "", err
	}

	return username, nil
}
