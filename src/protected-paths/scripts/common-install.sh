#!/usr/bin/env bash

# Common installation functions shared between install.sh and install-hook.sh

# --- Detect distro package manager and install dependencies ---
function install_dependencies() {
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
}

# --- Create dedicated majikmate:majikmate user/group for protected path ownership ---
function create_majikmate_user() {
    local owner_user="$1"
    
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
    sudo usermod -a -G majikmate "$owner_user"
    echo "   Added $owner_user to majikmate group"
}

# --- Install global hooks path and git configuration ---
function setup_git_hooks_path() {
    echo "🔧 Configuring global Git hooks path..."
    sudo mkdir -p /etc/git/hooks
    sudo git config --system core.hooksPath /etc/git/hooks
    echo "   Set core.hooksPath to /etc/git/hooks"
}

# --- Install the shared git hook and secure rsync wrapper ---
function install_hooks_and_wrapper() {
    local hooks_dir="$1"
    local scripts_dir="$2"
    
    # Install the shared git hook (calls githook binary)
    sudo install -m 0755 "${hooks_dir}/protect-sync-hook" /etc/git/hooks/protect-sync-hook
    
    # Install the secure rsync wrapper
    sudo install -m 755 "${scripts_dir}/githook-rsync" /etc/git/hooks/githook-rsync
}

# --- Create symbolic links for all post-* hooks ---
function create_hook_symlinks() {
    echo "🔗 Creating hook symlinks..."
    for hook in post-checkout post-merge post-rewrite post-applypatch post-commit post-reset; do
        sudo ln -sf protect-sync-hook "/etc/git/hooks/$hook"
        echo "   Linked $hook -> protect-sync-hook"
    done
}

# --- Configure sudo permissions for protected path operations ---
function configure_sudo_permissions() {
    local owner_user="$1"
    
    echo "🔐 Configuring sudo permissions..."
    
    # Configure sudoers to allow only our secure wrapper
    sudo tee /etc/sudoers.d/githook-protect > /dev/null <<EOF
# Allow $owner_user to run secure githook-rsync wrapper for file ownership operations
$owner_user ALL=(root) NOPASSWD: /etc/git/hooks/githook-rsync
EOF
    
    sudo chmod 440 /etc/sudoers.d/githook-protect
    echo "   Configured sudo permissions for $owner_user"
}

# --- Print installation summary ---
function print_installation_summary() {
    local owner_user="$1"
    local context="$2"  # "devcontainer" or "standalone"
    
    if [ "$context" = "devcontainer" ]; then
        echo "[protected-paths] Git hooks installed. Dev user: $owner_user, Protection user: majikmate."
    else
        echo "✅ Git hooks installed successfully!"
        echo ""
        echo "👤 Users configured:"
        echo "   - Dev user: $owner_user (member of majikmate group)"
        echo "   - Protection user: majikmate (system user for file ownership)"
        echo ""
        echo "🔧 Hook configuration:"
        echo "   - Global hooks path: /etc/git/hooks"
        echo "   - Active hooks: post-checkout, post-merge, post-rewrite, post-applypatch, post-commit, post-reset"
        echo ""
        echo "🗑️  To uninstall:"
        echo "   sudo rm -f /etc/git/hooks/post-* /etc/git/hooks/protect-sync-hook /etc/git/hooks/githook-rsync"
        echo "   sudo rm -f /etc/sudoers.d/githook-protect"
        echo "   sudo git config --system --unset core.hooksPath"
    fi
    
    echo "🎯 Features:"
    echo "   - Assignment-based sparse checkout (post-checkout branch changes)"
    echo "   - Protected path synchronization (all working tree modifications)"
    echo "   - Automatic configuration from workflow YAML files"
    echo "   - Go-based implementation"
    echo "   - Secure: hooks run as dedicated majikmate user, not root"
}