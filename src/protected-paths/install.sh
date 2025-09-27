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

# Add dev user to majikmate group for read access to protected files
sudo usermod -a -G majikmate "$OWNER_USER"
echo "   Added $OWNER_USER to majikmate group"

# --- Install global hooks path and git configuration ---
echo "🔧 Configuring global Git hooks path..."
sudo mkdir -p /etc/git/hooks
sudo git config --system core.hooksPath /etc/git/hooks
echo "   Set core.hooksPath to /etc/git/hooks"

# --- Install the shared git hook and secure rsync wrapper ---
# Install the shared git hook (calls githook binary)
sudo install -m 0755 hooks/protect-sync-hook /etc/git/hooks/protect-sync-hook

# Install the secure rsync wrapper
sudo install -m 755 scripts/githook-rsync /etc/git/hooks/githook-rsync

# --- Create symbolic links for all post-* hooks ---
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