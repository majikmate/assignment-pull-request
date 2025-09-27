#!/bin/bash
set -e

# Configuration
REPO_NAME="module-html-css-test"

# Determine the repo owner for the fork:
# 1) Allow override via TEST_REPO_OWNER env var
# 2) Else, use the currently authenticated GitHub user via gh
if [ -n "${TEST_REPO_OWNER}" ]; then
  REPO_OWNER="${TEST_REPO_OWNER}"
else
  REPO_OWNER=$(gh api user -q .login 2>/dev/null || echo "")
fi

if [ -z "${REPO_OWNER}" ]; then
  echo "❌ Error: Could not determine GitHub username. Set TEST_REPO_OWNER env var or ensure 'gh auth status' is logged in."
  exit 1
fi

FULL_REPO_NAME="${REPO_OWNER}/${REPO_NAME}"

echo "🚀 Starting real scenario test for ${FULL_REPO_NAME}..."

# Pre-flight checks
echo "🔍 Running pre-flight checks..."

# Check if /workspaces directory exists
if [ -d "/workspaces" ]; then
    echo "📂 /workspaces directory found, using dev container environment..."
    cd /workspaces
    
    # Remove existing repo directory if it exists
    if [ -d "${REPO_NAME}" ]; then
        echo "🗑️  Removing existing local ${REPO_NAME} directory..."
        rm -rf "${REPO_NAME}"
    fi
else
    echo "📂 /workspaces directory not found, checking current environment..."
    
    # Check if we're currently in a git repository
    if git rev-parse --git-dir >/dev/null 2>&1; then
        echo "❌ Error: Currently inside a git repository and not in /workspaces environment."
        echo "Current directory: $(pwd)"
        echo "Git root: $(git rev-parse --show-toplevel 2>/dev/null || echo 'unknown')"
        echo "Please run this script from outside any git repository."
        exit 1
    fi
    
    echo "✅ Not in a git repository, proceeding in current directory: $(pwd)"
fi

echo "🧹 Cleaning .github folder and creating test workflow..."

# Step 1: Delete existing test repo if it exists
echo "📋 Checking for existing test repository..."
if gh repo view "${FULL_REPO_NAME}" >/dev/null 2>&1; then
  echo "🗑️  Deleting existing test repository..."
  gh repo delete "${FULL_REPO_NAME}" --yes
else
  echo "ℹ️  No existing test repository found"
fi

# Step 2: Create repository and clone module-html-css content
echo "📦 Creating test repository with module-html-css content..."
gh repo create "${FULL_REPO_NAME}" --public --description "Test repo for assignment PR actions with module-html-css content"

# Wait for repo to be ready
echo "⏳ Waiting for repository to be ready..."
MAX_REPO_ATTEMPTS=12
REPO_ATTEMPT=1
while [ $REPO_ATTEMPT -le $MAX_REPO_ATTEMPTS ]; do
  echo "Checking repository availability (attempt $REPO_ATTEMPT/$MAX_REPO_ATTEMPTS)..."
  if gh repo view "${FULL_REPO_NAME}" >/dev/null 2>&1; then
    echo "✅ Repository is ready!"
    break
  fi
  sleep 5
  REPO_ATTEMPT=$((REPO_ATTEMPT+1))
done

if [ $REPO_ATTEMPT -gt $MAX_REPO_ATTEMPTS ]; then
  echo "❌ Repository not ready after waiting. Exiting."
  exit 1
fi

# Prepare a clean working directory
if [ -d "${REPO_NAME}" ]; then
  rm -rf "${REPO_NAME}"
fi

# Clone the module-html-css repository content
echo "📥 Cloning module-html-css content..."
git clone "https://github.com/majikmate/module-html-css.git" "${REPO_NAME}"
cd "${REPO_NAME}"

# Update remote to point to our test repository
git remote set-url origin "https://github.com/${FULL_REPO_NAME}.git"

echo "🧹 Setting up test workflow..."

# Remove entire .github folder for clean slate
rm -rf .github

# Create .github/workflows directory
mkdir -p .github/workflows

# Create test workflow
cat > .github/workflows/test-action.yml << 'EOF'
name: Test Assignment PR Action

on:
  workflow_dispatch:
  push:
    branches: [main]

permissions:
  contents: write
  pull-requests: write

concurrency:
  group: ${{ github.repository }}
  cancel-in-progress: false

jobs:
  assignment-pull-request:
    name: Test Real Assignment Processing
    runs-on: ubuntu-latest

    steps:
      - name: Checkout repository
        uses: actions/checkout@v5
        with:
          fetch-depth: 0

      - name: Run assignment PR creator
        uses: majikmate/assignment-pull-request@main
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          assignment-regex: |
            ^none$
          protected-paths-regex: |
            ^(.devcontainer)$
            ^(.github)$
            ^(10-tutorials)$
            ^(reference-material)$
          default-branch: main
          dry-run: "no"
EOF

# Commit and push the changes
echo "📝 Committing and pushing test workflow and module content..."
git add .
git commit -m "Add test workflow and module-html-css content"
git push origin main
echo "✅ Test workflow and content pushed"
