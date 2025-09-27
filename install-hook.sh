#!/usr/bin/env bash

# GitHub Post-Checkout Hook Installer
# This script installs the assignment-pull-request git hooks in exactly the same way 
# as the devcontainer feature, with the exception that it checks if the current folder
# is a complete source tree and installs from that source tree, otherwise installs 
# from the remote module.
#
# Usage: ./install-hook.sh [version]
#   version can be:
#   - (empty)     : Install from @latest (latest release)
#   - v1.2.3      : Install from specific semantic version tag
#   - v1.2        : Install from specific major.minor tag
#   - v1          : Install from specific major tag
#   - branch-name : Install from specific branch name

set -euo pipefail

REPO="majikmate/assignment-pull-request"
MODULE="github.com/majikmate/assignment-pull-request"
BINARY_NAME="githook"
HOOK_NAME="protect-sync-hook"
OWNER_USER="${USER:-vscode}"
VERSION_ARG="${1:-}"

echo "🔧 Installing Assignment Pull Request git hooks..."

# Function to check if current directory is a complete source tree
function is_complete_source_tree() {
    # Check if we have the essential files for a complete source tree
    [[ -f "go.mod" ]] && \
    [[ -f "cmd/githook/main.go" ]] && \
    [[ -d "internal" ]] && \
    [[ -f "src/protected-paths/hooks/protect-sync-hook" ]] && \
    grep -q "module.*assignment-pull-request" go.mod 2>/dev/null
}

# Function to resolve version to clone
function resolve_version_to_clone() {
    local version="$1"
    
    # If no version specified, find the latest stable semantic version
    if [ -z "$version" ]; then
        echo "   📦 Finding latest stable version..."
        
        # First, check if there's a "latest" tag
        local latest_tag_exists
        latest_tag_exists=$(git ls-remote --exit-code --tags "https://github.com/$REPO.git" "refs/tags/latest" >/dev/null 2>&1 && echo "yes" || echo "no")
        
        if [ "$latest_tag_exists" = "yes" ]; then
            echo "   ✅ Found 'latest' tag"
            echo "latest"
            return
        fi
        
        # Get all semantic version tags from the repository
        local all_tags
        all_tags=$(git ls-remote --tags "https://github.com/$REPO.git" | awk '{print $2}' | sed 's|refs/tags/||' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V || echo "")
        
        if [ -n "$all_tags" ]; then
            # Get the latest semantic version (sort -V handles semantic versioning)
            local latest_stable
            latest_stable=$(echo "$all_tags" | tail -n 1)
            echo "   ✅ Found latest stable version: $latest_stable"
            echo "$latest_stable"
        else
            # Fallback: try GitHub releases API
            echo "   📦 No semantic version tags found, checking GitHub releases..."
            local github_latest
            github_latest=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")
            
            if [ -n "$github_latest" ]; then
                echo "   ✅ Found latest release: $github_latest"
                echo "$github_latest"
            else
                echo "   ⚠️  No releases found, using main branch"
                echo "main"
            fi
        fi
        return
    fi
    
    # If version starts with 'v' and follows semantic versioning, validate it exists
    if [[ "$version" =~ ^v[0-9]+(\.[0-9]+)*$ ]]; then
        echo "   🔍 Validating semantic version tag: $version"
        
        # Check if the exact tag exists
        if git ls-remote --exit-code --tags "https://github.com/$REPO.git" "refs/tags/$version" >/dev/null 2>&1; then
            echo "   ✅ Tag $version exists"
            echo "$version"
        else
            # For partial versions like v1 or v1.2, find the latest matching version
            echo "   🔍 Searching for latest version matching $version..."
            local matching_tags
            matching_tags=$(git ls-remote --tags "https://github.com/$REPO.git" | awk '{print $2}' | sed 's|refs/tags/||' | grep -E "^${version}(\.[0-9]+)*$" | sort -V || echo "")
            
            if [ -n "$matching_tags" ]; then
                local latest_match
                latest_match=$(echo "$matching_tags" | tail -n 1)
                echo "   ✅ Found matching version: $latest_match"
                echo "$latest_match"
            else
                echo "   ⚠️  No matching version found for $version, will try as branch"
                echo "$version"
            fi
        fi
        return
    fi
    
    # Check for special "latest" tag
    if [ "$version" = "latest" ]; then
        echo "   🔍 Checking for 'latest' tag..."
        if git ls-remote --exit-code --tags "https://github.com/$REPO.git" "refs/tags/latest" >/dev/null 2>&1; then
            echo "   ✅ Found 'latest' tag"
            echo "latest"
        else
            echo "   ⚠️  No 'latest' tag found, falling back to semantic versioning..."
            # Recursively call with empty version to get latest stable
            resolve_version_to_clone ""
        fi
        return
    fi
    
    # Otherwise, treat as branch name
    echo "   📋 Using as branch name: $version"
    echo "$version"
}

# Function to clone remote repository when needed
function clone_remote_repository() {
    echo "   🌐 Cloning remote repository..."
    
    # Create temporary directory and set up cleanup trap
    TEMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TEMP_DIR"' EXIT
    cd "$TEMP_DIR"
    
    # Resolve version to clone
    TARGET_VERSION=$(resolve_version_to_clone "$VERSION_ARG")
    
    if [ "$TARGET_VERSION" = "main" ]; then
        echo "   📦 No releases found, downloading from main branch..."
        git clone --depth 1 "https://github.com/$REPO.git" .
    else
        echo "   📦 Downloading repository at $TARGET_VERSION..."
        if git clone --depth 1 --branch "$TARGET_VERSION" "https://github.com/$REPO.git" . 2>/dev/null; then
            echo "   ✅ Successfully cloned $TARGET_VERSION"
        else
            echo "   ⚠️  Failed to clone $TARGET_VERSION, falling back to main branch..."
            git clone --depth 1 "https://github.com/$REPO.git" .
        fi
    fi
    
    echo "   ✅ Repository cloned successfully"
}

# Function to install hooks and binaries (local or remote)
function install_hooks_and_binaries() {
    local is_local_source=$1
    
    if [ "$is_local_source" = "true" ]; then
        echo "   📁 Installing from local source tree..."
    else
        echo "   🌐 Installing from remote repository..."
        # Clone the remote repository first
        clone_remote_repository
    fi
    
    # Check if Go is available
    if ! command -v go >/dev/null 2>&1; then
        echo "   ❌ Go is required to install from source"
        echo "      Please install Go"
        exit 1
    fi

    # Install the githook binary
    echo "   🔨 Installing githook binary..."
    go install ./cmd/githook
    
    # Install the shared git hook (calls githook binary)
    sudo install -m 0755 src/protected-paths/hooks/protect-sync-hook /etc/git/hooks/protect-sync-hook
    
    # Install the secure rsync wrapper
    sudo install -m 755 src/protected-paths/scripts/githook-rsync /etc/git/hooks/githook-rsync
    
    if [ "$is_local_source" = "true" ]; then
        echo "   ✅ Installed from local source tree"
    else
        # Return to original directory (trap will handle cleanup)
        cd - >/dev/null
        echo "   ✅ Installed from remote repository"
    fi
}

# --- Detect distro package manager and install dependencies ---
if command -v apt-get >/dev/null 2>&1; then
    PKG="apt-get"
    sudo apt-get update -y
    sudo apt-get install -y --no-install-recommends git rsync sudo ca-certificates
elif command -v apk >/dev/null 2>&1; then
    PKG="apk"
    sudo apk add --no-cache git rsync sudo ca-certificates shadow bash
else
    echo "Unsupported distro: need apt-get or apk" >&2
    exit 1
fi

# --- Create dedicated majikmate:majikmate user/group for protected path ownership ---
echo "📱 Creating majikmate user/group for protected path ownership..."

# Create majikmate group if it doesn't exist
if ! getent group majikmate >/dev/null 2>&1; then
    sudo groupadd --system majikmate
    echo "   Created majikmate group"
fi

# Create majikmate user if it doesn't exist  
if ! getent passwd majikmate >/dev/null 2>&1; then
    sudo useradd --system --gid majikmate --home-dir /nonexistent --shell /usr/sbin/nologin majikmate
    echo "   Created majikmate user"
fi

# Add dev user to majikmate group for read access to protected files
sudo usermod -a -G majikmate "$OWNER_USER"
echo "   Added $OWNER_USER to majikmate group"

# --- Install global hooks path and git configuration ---
echo "🔧 Configuring global Git hooks path..."
sudo mkdir -p /etc/git/hooks
sudo git config --system core.hooksPath /etc/git/hooks
echo "   Set core.hooksPath to /etc/git/hooks"

# --- Install hooks and binaries ---
echo "📥 Installing hooks and binaries..."

# Show what version will be installed if installing from remote
if ! is_complete_source_tree && [ -n "$VERSION_ARG" ]; then
    echo "   🎯 Target version: $VERSION_ARG"
elif ! is_complete_source_tree; then
    echo "   🎯 Target version: @latest"
fi

# Check if we're in a complete source tree and install accordingly
if is_complete_source_tree; then
    install_hooks_and_binaries "true"
else
    install_hooks_and_binaries "false"
fi

# Create symbolic links for all post-* hooks that modify the working tree
echo "🔗 Creating hook symlinks..."
for hook in post-checkout post-merge post-rewrite post-applypatch post-commit post-reset; do
  sudo ln -sf protect-sync-hook "/etc/git/hooks/$hook"
  echo "   Linked $hook -> protect-sync-hook"
done

# --- Configure sudo permissions for protected path operations ---
echo "🔐 Configuring sudo permissions..."

# Configure sudoers to allow only our secure wrapper
sudo tee /etc/sudoers.d/githook-protect > /dev/null <<EOF
# Allow $OWNER_USER to run secure githook-rsync wrapper for file ownership operations
$OWNER_USER ALL=(root) NOPASSWD: /etc/git/hooks/githook-rsync
EOF

sudo chmod 440 /etc/sudoers.d/githook-protect
echo "   Configured sudo permissions for $OWNER_USER"

# --- Print installation summary ---
echo "[protected-paths] Git hooks installed. Dev user: $OWNER_USER, Protection user: majikmate."
echo "🎯 Features:"
echo "   - Assignment-based sparse checkout (post-checkout branch changes)"
echo "   - Protected path synchronization (all working tree modifications)"
echo "   - Automatic configuration from workflow YAML files"
echo "   - Go-based implementation"
echo "   - Secure: hooks run as dedicated majikmate user, not root"