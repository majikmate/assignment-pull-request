#!/bin/bash

# GitHub Post-Checkout Hook Installer
# This script downloads and installs the assignment-pull-request post-checkout hook

set -e

REPO="majikmate/assignment-pull-request"
HOOK_NAME="post-checkout"
GITHOOKS_DIR="$HOME/.githooks"

echo "🔧 Installing Assignment Pull Request post-checkout hook..."

# Function to build from source
function build_from_source() {
    echo "   Building from source..."
    
    # Check if Go is available
    if ! command -v go >/dev/null 2>&1; then
        echo "   ❌ Go is required to build from source"
        echo "      Please install Go or download a pre-built binary"
        exit 1
    fi
    
    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # Clone repository
    echo "   📦 Cloning repository..."
    git clone --depth 1 "https://github.com/$REPO.git" .
    
    # Build the hook
    echo "   🔨 Building post-checkout hook..."
    go build -o "$GITHOOKS_DIR/$HOOK_NAME" ./cmd/githook
    
    # Cleanup
    cd - >/dev/null
    rm -rf "$TEMP_DIR"
    
    echo "   ✅ Built from source"
}

# Create githooks directory if it doesn't exist
mkdir -p "$GITHOOKS_DIR"

# Set global hooks path if not already set
current_hooks_path=$(git config --global --get core.hooksPath 2>/dev/null || echo "")
if [ "$current_hooks_path" != "$GITHOOKS_DIR" ]; then
    echo "📁 Configuring global Git hooks path..."
    git config --global core.hooksPath "$GITHOOKS_DIR"
    echo "   Set core.hooksPath to $GITHOOKS_DIR"
fi

# Download the latest release or build from source
echo "📥 Downloading post-checkout hook..."

# Try to download from GitHub releases first
if command -v curl >/dev/null 2>&1; then
    # Get latest release download URL
    LATEST_RELEASE=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -n "$LATEST_RELEASE" ]; then
        echo "   Found release: $LATEST_RELEASE"
        DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/post-checkout-linux"
        
        if curl -L --fail "$DOWNLOAD_URL" -o "$GITHOOKS_DIR/$HOOK_NAME" 2>/dev/null; then
            echo "   ✅ Downloaded from release"
        else
            echo "   ⚠️  Release download failed, building from source..."
            build_from_source
        fi
    else
        echo "   ⚠️  No releases found, building from source..."
        build_from_source
    fi
else
    echo "   ⚠️  curl not available, building from source..."
    build_from_source
fi

# Make executable
chmod +x "$GITHOOKS_DIR/$HOOK_NAME"

echo "✅ Post-checkout hook installed successfully!"
echo ""
echo "📖 Usage:"
echo "   The hook will automatically run when you checkout branches."
echo "   It will:"
echo "   1. Scan .github/workflows/ for assignment-pull-request action usage"
echo "   2. Extract regex patterns from the action configuration"
echo "   3. Check if current branch matches any assignment folder"
echo "   4. Setup sparse-checkout to show only matching assignments"
echo ""
echo "🔧 Manual hook path configuration (if needed):"
echo "   git config --global core.hooksPath $GITHOOKS_DIR"
echo ""
echo "🗑️  To uninstall:"
echo "   rm $GITHOOKS_DIR/$HOOK_NAME"
echo "   git config --global --unset core.hooksPath  # if you want to disable global hooks"