#!/usr/bin/env bash
set -euo pipefail

# --- Resolve feature options / defaults ---
# Use the devcontainer remote user (always available in devcontainer features)
OWNER_USER="${_REMOTE_USER:-vscode}"

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

# --- Install global hooks path and git configuration ---
echo "🔧 Configuring global Git hooks path..."
sudo mkdir -p /etc/git/hooks
sudo git config --system core.hooksPath /etc/git/hooks
echo "   Set core.hooksPath to /etc/git/hooks"

# --- Install the shared git hook and secure githook-rsync binary ---
# Install the shared git hook (calls githook binary which auto-installs itself)
sudo install -m 0755 hooks/protect-sync-hook /etc/git/hooks/protect-sync-hook

# Build and install secure githook-rsync binary (requires Go and source code)
if command -v go >/dev/null 2>&1; then
    echo "🔒 Building and installing secure githook-rsync binary..."
    
    # Clone the repository to build githook-rsync
    TEMP_BUILD_DIR=$(mktemp -d)
    trap 'rm -rf "$TEMP_BUILD_DIR"' EXIT
    
    cd "$TEMP_BUILD_DIR"
    git clone --depth 1 https://github.com/majikmate/assignment-pull-request.git .
    
    # Build and install githook-rsync
    go build -o githook-rsync ./cmd/githook-rsync
    sudo install -m 755 githook-rsync /etc/git/hooks/githook-rsync
    
    cd - >/dev/null
    echo "   📦 githook-rsync installed to /etc/git/hooks/ (root-owned, secure)"
else
    echo "   ⚠️  Go not available - githook-rsync will be installed on first hook run"
    # Create a placeholder that will trigger installation on first run
    sudo tee /etc/git/hooks/githook-rsync > /dev/null <<'EOF'
#!/bin/bash
# Placeholder - will be replaced with Go binary on first hook execution
echo "githook-rsync not yet installed - install Go and run a git hook" >&2
exit 1
EOF
    sudo chmod 755 /etc/git/hooks/githook-rsync
fi

# --- Create symbolic links for all post-* hooks ---
echo "🔗 Creating hook symlinks..."
for hook in post-checkout post-merge post-rewrite post-applypatch post-commit post-reset; do
    sudo ln -sf protect-sync-hook "/etc/git/hooks/$hook"
    echo "   Linked $hook -> protect-sync-hook"
done

# --- Configure sudo permissions for githook-rsync binary operations ---
echo "🔐 Configuring sudo permissions..."

# Configure sudoers to allow the specific githook-rsync binary in /etc/git/hooks/
sudo tee /etc/sudoers.d/githook-protect > /dev/null <<EOF
# Allow $OWNER_USER to run secure githook-rsync binary for file ownership operations
# Binary is installed by root to /etc/git/hooks/ and cannot be tampered with by users
$OWNER_USER ALL=(root) NOPASSWD: /etc/git/hooks/githook-rsync
EOF

sudo chmod 440 /etc/sudoers.d/githook-protect
echo "   Configured sudo permissions for $OWNER_USER to run /etc/git/hooks/githook-rsync"

# --- Print installation summary ---
echo "[protected-paths] Git hooks installed. Dev user: $OWNER_USER, Protection user: majikmate."
echo "🎯 Features:"
echo "   - Assignment-based sparse checkout (post-checkout branch changes)"
echo "   - Protected path synchronization (all working tree modifications)"
echo "   - Automatic configuration from workflow YAML files"
echo "   - Go-based implementation: githook (user-managed), githook-rsync (root-owned)"
echo "   - Secure: githook-rsync installed by root, runs with sudo for ownership operations"