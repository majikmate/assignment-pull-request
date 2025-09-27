#!/usr/bin/env bash

# GitHub Post-Checkout Hook Installer
# This script installs the assignment-pull-request git hooks in exactly the same way 
# as the devcontainer feature, with the exception that it checks if the current folder 
# is a complete source tree and installs from that source tree, otherwise installs 
# from the remote module.

set -euo pipefail

REPO="majikmate/assignment-pull-request"
MODULE="github.com/majikmate/assignment-pull-request"
BINARY_NAME="githook"
HOOK_NAME="protect-sync-hook"
OWNER_USER="${USER:-vscode}"

echo "🔧 Installing Assignment Pull Request git hooks..."

# --- Detect distro package manager ---
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

# Function to check if current directory is a complete source tree
function is_complete_source_tree() {
    # Check if we have the essential files for a complete source tree
    [[ -f "go.mod" ]] && \
    [[ -f "cmd/githook/main.go" ]] && \
    [[ -d "internal" ]] && \
    [[ -f "src/protected-paths/hooks/protect-sync-hook" ]] && \
    grep -q "module.*assignment-pull-request" go.mod 2>/dev/null
}

# Function to install from local source tree
function install_from_source_tree() {
    echo "   📁 Installing from local source tree..."
    
    # Check if Go is available
    if ! command -v go >/dev/null 2>&1; then
        echo "   ❌ Go is required to build from source"
        echo "      Please install Go or run from a different location"
        exit 1
    fi

    # Build the githook binary
    echo "   🔨 Building githook binary..."
    go build -o "${GOBIN:-$(go env GOPATH)/bin}/$BINARY_NAME" ./cmd/githook
    
    # Install the shared git hook (calls githook binary) from local files
    sudo install -m 0755 src/protected-paths/hooks/protect-sync-hook /etc/git/hooks/protect-sync-hook
    
    echo "   ✅ Installed from local source tree"
}

# Function to install from remote module
function install_from_remote() {
    echo "   🌐 Installing from remote module..."
    
    # Check if Go is available
    if ! command -v go >/dev/null 2>&1; then
        echo "   ❌ Go is required to install from remote module"
        echo "      Please install Go"
        exit 1
    fi

    # Create temporary directory and set up cleanup trap
    TEMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TEMP_DIR"' EXIT
    cd "$TEMP_DIR"
    
    # Clone repository to get hook files
    echo "   📦 Downloading hook files..."
    git clone --depth 1 "https://github.com/$REPO.git" .
    
    # Install the githook binary from remote
    echo "   🔨 Installing githook binary from remote..."
    go install "${MODULE}/cmd/${BINARY_NAME}@latest"
    
    # Install the shared git hook (calls githook binary)
    sudo install -m 0755 src/protected-paths/hooks/protect-sync-hook /etc/git/hooks/protect-sync-hook
    
    # Return to original directory (trap will handle cleanup)
    cd - >/dev/null
    
    echo "   ✅ Installed from remote module"
}

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

# --- Install global hooks path ---
echo "� Configuring global Git hooks path..."
sudo mkdir -p /etc/git/hooks
sudo git config --system core.hooksPath /etc/git/hooks
echo "   Set core.hooksPath to /etc/git/hooks"

# --- Install hooks and binaries ---
echo "📥 Installing hooks and binaries..."

# Check if we're in a complete source tree and install accordingly
if is_complete_source_tree; then
    install_from_source_tree
else
    install_from_remote
fi

# Create symbolic links for all post-* hooks that modify the working tree
echo "🔗 Creating hook symlinks..."
for hook in post-checkout post-merge post-rewrite post-applypatch post-commit post-reset; do
  sudo ln -sf protect-sync-hook "/etc/git/hooks/$hook"
  echo "   Linked $hook -> protect-sync-hook"
done

# --- Grant minimal sudo (NOPASSWD) for running githook as majikmate user ---
echo "🔐 Configuring sudo permissions..."
# This allows the hook to run the githook binary as majikmate user for path protection

GOPATH_DIR="$(go env GOPATH 2>/dev/null || echo "/home/$OWNER_USER/go")"

sudo tee /etc/sudoers.d/githook-protect > /dev/null <<EOF
# Allow $OWNER_USER to run githook as majikmate user for path protection
$OWNER_USER ALL=(majikmate) NOPASSWD: $GOPATH_DIR/bin/githook
EOF

sudo chmod 440 /etc/sudoers.d/githook-protect
echo "   Configured sudo permissions for $OWNER_USER"

echo "✅ Git hooks installed successfully!"
echo ""
echo "🎯 Features:"
echo "   - Assignment-based sparse checkout (post-checkout branch changes)"
echo "   - Protected path synchronization (all working tree modifications)"
echo "   - Automatic configuration from workflow YAML files"
echo "   - Go-based implementation"
echo "   - Secure: hooks run as dedicated majikmate user, not root"
echo ""
echo "👤 Users configured:"
echo "   - Dev user: $OWNER_USER (member of majikmate group)"
echo "   - Protection user: majikmate (system user for file ownership)"
echo ""
echo "🔧 Hook configuration:"
echo "   - Global hooks path: /etc/git/hooks"
echo "   - Active hooks: post-checkout, post-merge, post-rewrite, post-applypatch, post-commit, post-reset"
echo ""
echo "🗑️  To uninstall:"
echo "   sudo rm -f /etc/git/hooks/post-* /etc/git/hooks/protect-sync-hook"
echo "   sudo rm -f /etc/sudoers.d/githook-protect"
echo "   sudo git config --system --unset core.hooksPath"