#!/usr/bin/env bash
set -euo pipefail

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

# --- Resolve feature options / defaults ---
# Use the devcontainer remote user (always available in devcontainer features)
OWNER_USER="${_REMOTE_USER:-vscode}"

# --- Create dedicated majikmate:majikmate user/group for protected path ownership ---
echo "Creating majikmate user/group for protected path ownership..."

# Create majikmate group if it doesn't exist
if ! getent group majikmate >/dev/null 2>&1; then
    sudo groupadd --system majikmate
    echo "Created majikmate group"
fi

# Create majikmate user if it doesn't exist  
if ! getent passwd majikmate >/dev/null 2>&1; then
    sudo useradd --system --gid majikmate --home-dir /nonexistent --shell /usr/sbin/nologin majikmate
    echo "Created majikmate user"
fi

# Add dev user to majikmate group for read access to protected files
sudo usermod -a -G majikmate "$OWNER_USER"
echo "Added $OWNER_USER to majikmate group"

# --- Install global hooks path ---
sudo mkdir -p /etc/git/hooks
sudo git config --system core.hooksPath /etc/git/hooks

# --- Install the shared git hook (calls githook binary) ---
sudo install -m 0755 hooks/protect-sync-hook /etc/git/hooks/protect-sync-hook

# Create symbolic links for all post-* hooks that modify the working tree
for hook in post-checkout post-merge post-rewrite post-applypatch post-commit post-reset; do
  sudo ln -sf protect-sync-hook "/etc/git/hooks/$hook"
done

# --- Install backup protect-sync script ---
sudo install -m 0755 scripts/protect-sync /usr/local/bin/protect-sync

# --- Grant minimal sudo (NOPASSWD) for running githook as majikmate user ---
# This allows the hook to run the githook binary as majikmate user for path protection
sudo bash -c "cat > /etc/sudoers.d/githook-protect <<EOF
# Allow $OWNER_USER to run githook as majikmate user for path protection
$OWNER_USER ALL=(majikmate) NOPASSWD: $(go env GOPATH 2>/dev/null || echo "/home/$OWNER_USER/go")/bin/githook
$OWNER_USER ALL=(majikmate) NOPASSWD: /usr/local/bin/githook
# Allow backup protect-sync script (legacy support)
$OWNER_USER ALL=(majikmate) NOPASSWD: /usr/local/bin/protect-sync
EOF"
sudo chmod 440 /etc/sudoers.d/githook-protect

echo "[protected-paths] Git hooks installed. Dev user: $OWNER_USER, Protection user: majikmate."
echo "Features:"
echo "  - Assignment-based sparse checkout (post-checkout branch changes)"
echo "  - Protected path synchronization (all working tree modifications)"
echo "  - Automatic configuration from workflow YAML files"
echo "  - Go-based implementation with bash backup script"
echo "  - Secure: hooks run as dedicated majikmate user, not root"