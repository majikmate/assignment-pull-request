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

# --- Install the shared git hook ---
# Install the shared git hook (calls githook binary which auto-installs itself and githook-rsync)
sudo install -m 0755 hooks/protect-sync-hook /etc/git/hooks/protect-sync-hook

# --- Create symbolic links for all post-* hooks ---
echo "🔗 Creating hook symlinks..."
for hook in post-checkout post-merge post-rewrite post-applypatch post-commit post-reset; do
    sudo ln -sf protect-sync-hook "/etc/git/hooks/$hook"
    echo "   Linked $hook -> protect-sync-hook"
done

# --- No sudo permissions needed - Go binaries handle security internally ---
echo "✅ Security handled internally by Go binaries (no sudo configuration needed)"

# --- Print installation summary ---
echo "[protected-paths] Git hooks installed. Dev user: $OWNER_USER, Protection user: majikmate."
echo "🎯 Features:"
echo "   - Assignment-based sparse checkout (post-checkout branch changes)"
echo "   - Protected path synchronization (all working tree modifications)"
echo "   - Automatic configuration from workflow YAML files"
echo "   - Go-based implementation with auto-updating binaries"
echo "   - Secure: binaries handle validation internally, no root privileges needed"