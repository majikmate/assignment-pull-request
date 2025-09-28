#!/usr/bin/env bash

# Assignment Pull Request Git Hooks Cleanup Script
# This script removes all components installed by hooks-install.sh or the devcontainer feature
# to revert the system to a clean state.

set -euo pipefail

echo "🧹 Cleaning up Assignment Pull Request git hooks installation..."

# --- Remove git hooks and related files ---
echo "🗑️  Removing git hooks..."
sudo rm -f /etc/git/hooks/post-* /etc/git/hooks/protect-sync-hook /etc/git/hooks/githook-rsync 2>/dev/null || true
echo "   Removed all hook files"

# --- Remove git hooks directories if empty ---
echo "📁 Removing git directories..."
sudo rmdir /etc/git/hooks /etc/git 2>/dev/null || true
echo "   Removed empty git directories"

# --- Unset global git configuration ---
echo "⚙️  Removing git configuration..."
sudo git config --system --unset core.hooksPath 2>/dev/null || true
echo "   Unset global hooks path"

# --- Remove sudo permissions ---
echo "🔐 Removing sudo permissions..."
sudo rm -f /etc/sudoers.d/githook-protect 2>/dev/null || true
echo "   Removed sudo permissions file"

# --- Remove majikmate user and group ---
echo "👤 Removing majikmate user and group..."
sudo userdel majikmate 2>/dev/null || true
echo "   Removed majikmate user"

sudo groupdel majikmate 2>/dev/null || true
echo "   Removed majikmate group"

# --- Remove githook binary ---
echo "🔨 Removing githook binary..."
if command -v githook >/dev/null 2>&1; then
    GITHOOK_PATH=$(which githook)
    rm -f "$GITHOOK_PATH" 2>/dev/null || true
    echo "   Removed githook binary from $GITHOOK_PATH"
else
    echo "   No githook binary found"
fi

# --- Verification ---
echo "🔍 Verifying cleanup..."

# Check git directory
if [ -d "/etc/git" ]; then
    echo "   ⚠️  /etc/git directory still exists"
else
    echo "   ✅ /etc/git directory removed"
fi

# Check majikmate user
if getent passwd majikmate >/dev/null 2>&1; then
    echo "   ⚠️  majikmate user still exists"
else
    echo "   ✅ majikmate user removed"
fi

# Check majikmate group
if getent group majikmate >/dev/null 2>&1; then
    echo "   ⚠️  majikmate group still exists"
else
    echo "   ✅ majikmate group removed"
fi

# Check githook binary
if command -v githook >/dev/null 2>&1; then
    echo "   ⚠️  githook binary still in PATH"
else
    echo "   ✅ githook binary removed"
fi

# Check git hooks path
if git config --system --get core.hooksPath >/dev/null 2>&1; then
    echo "   ⚠️  Global git hooks path still configured"
else
    echo "   ✅ Global git hooks path unset"
fi

echo ""
echo "✅ Cleanup completed successfully!"
echo ""
echo "🔄 To reinstall, run:"
echo "   ./hooks-install.sh"