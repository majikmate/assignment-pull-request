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
sudo install -m 0755 protect-sync-hook /etc/git/hooks/protect-sync-hook

# Create symbolic links for all post-* hooks that modify the working tree
for hook in post-checkout post-merge post-rewrite post-applypatch post-commit post-reset; do
  sudo ln -sf protect-sync-hook "/etc/git/hooks/$hook"
done

# --- Grant minimal sudo (NOPASSWD) for the githook binary to the dev user ---
# This allows the githook to run with root privileges for path protection
sudo bash -c "cat > /etc/sudoers.d/githook-protect <<EOF
# Allow $OWNER_USER to run githook with root privileges for path protection
$OWNER_USER ALL=(root) NOPASSWD: $(go env GOPATH)/bin/githook
$OWNER_USER ALL=(root) NOPASSWD: /usr/local/bin/githook
# Allow rsync and chown commands needed for path protection
$OWNER_USER ALL=(root) NOPASSWD: /usr/bin/rsync
$OWNER_USER ALL=(root) NOPASSWD: /bin/chown
$OWNER_USER ALL=(root) NOPASSWD: /bin/chmod
EOF"
sudo chmod 440 /etc/sudoers.d/githook-protect

echo "[assignment-pull-request] Git hooks installed. Dev user: $OWNER_USER."
echo "Hooks will handle:"
echo "  - Sparse checkout configuration (post-checkout branch changes)" 
echo "  - Protected path synchronization (all working tree modifications)"