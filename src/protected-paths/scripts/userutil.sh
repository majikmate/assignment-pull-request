#!/usr/bin/env bash

# Bash utility functions for user detection - mirrors the Go userutil package
# This provides a consistent user detection interface across all scripts

# get_current_user: Get current user using most reliable method available
# Mirrors userutil.GetCurrentUser() from Go package
get_current_user() {
    local username

    # Try USER environment variable first
    if [ -n "${USER:-}" ]; then
        echo "$USER"
        return 0
    fi

    # Try LOGNAME environment variable (POSIX standard)
    if [ -n "${LOGNAME:-}" ]; then
        echo "$LOGNAME"
        return 0
    fi

    # Try whoami command (handles edge cases where env vars are missing)
    if username=$(whoami 2>/dev/null) && [ -n "$username" ]; then
        echo "$username"
        return 0
    fi

    # Final fallback for containerized environments
    echo "vscode"
    return 0
}

# get_real_user: Get real user considering SUDO_USER environment variable
# Mirrors userutil.GetRealUser() from Go package
get_real_user() {
    # Check SUDO_USER first (original user before sudo escalation)
    if [ -n "${SUDO_USER:-}" ]; then
        echo "$SUDO_USER"
        return 0
    fi

    # Fall back to current user
    get_current_user
}

# validate_user: Ensure the user is not root and return appropriate error if it is
# Mirrors userutil.ValidateUser() from Go package
validate_user() {
    local username="$1"
    
    if [ "$username" = "root" ]; then
        echo "Error: refusing to operate as root user" >&2
        return 1
    fi
    
    return 0
}

# get_validated_current_user: Get current user and validate it's not root
# Mirrors userutil.GetValidatedCurrentUser() from Go package
get_validated_current_user() {
    local username
    username=$(get_current_user)
    
    if ! validate_user "$username"; then
        return 1
    fi
    
    echo "$username"
    return 0
}

# get_validated_real_user: Get real user (considering SUDO_USER) and validate it's not root
# Mirrors userutil.GetValidatedRealUser() from Go package
get_validated_real_user() {
    local username
    username=$(get_real_user)
    
    if ! validate_user "$username"; then
        return 1
    fi
    
    echo "$username"
    return 0
}