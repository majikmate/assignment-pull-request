#!/usr/bin/env bash
set -euo pipefail

# Source common installation functions
source "$(dirname "$0")/scripts/common-install.sh"

# --- Resolve feature options / defaults ---
# Use the devcontainer remote user (always available in devcontainer features)
OWNER_USER="${_REMOTE_USER:-vscode}"

# --- Install dependencies ---
install_dependencies

# --- Create majikmate user/group ---
create_majikmate_user "$OWNER_USER"

# --- Setup git hooks path ---
setup_git_hooks_path

# --- Install hooks and wrapper ---
install_hooks_and_wrapper "hooks" "scripts"

# --- Create hook symlinks ---
create_hook_symlinks

# --- Configure sudo permissions ---
configure_sudo_permissions "$OWNER_USER"

# --- Print summary ---
print_installation_summary "$OWNER_USER" "devcontainer"