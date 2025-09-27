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

# --- Grant minimal sudo (NOPASSWD) for the githook binary and related commands to the dev user ---
# This allows the githook to run with root privileges for path protection
sudo bash -c "cat > /etc/sudoers.d/githook-protect <<EOF
# Allow $OWNER_USER to run githook with root privileges for path protection
$OWNER_USER ALL=(root) NOPASSWD: $(go env GOPATH 2>/dev/null || echo "/home/$OWNER_USER/go")/bin/githook
$OWNER_USER ALL=(root) NOPASSWD: /usr/local/bin/githook
# Allow backup protect-sync script (legacy support)
$OWNER_USER ALL=(root) NOPASSWD: /usr/local/bin/protect-sync
# Allow rsync and chown commands needed for path protection
$OWNER_USER ALL=(root) NOPASSWD: /usr/bin/rsync
$OWNER_USER ALL=(root) NOPASSWD: /bin/chown
$OWNER_USER ALL=(root) NOPASSWD: /bin/chmod
EOF"
sudo chmod 440 /etc/sudoers.d/githook-protect

echo "[protected-paths] Git hooks installed. Dev user: $OWNER_USER."
echo "Features:"
echo "  - Assignment-based sparse checkout (post-checkout branch changes)"
echo "  - Protected path synchronization (all working tree modifications)"
echo "  - Automatic configuration from workflow YAML files"
echo "  - Go-based implementation with bash backup script"