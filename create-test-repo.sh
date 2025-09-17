#!/bin/bash
set -e

# Configuration
REPO_NAME="assignment-pull-request-test"

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

# Step 2: Fork the repository
echo "🍴 Creating fork of majikmate/assignment-pull-request..."
gh repo fork majikmate/assignment-pull-request --fork-name "${REPO_NAME}" --clone=false

# Wait for fork to be ready
echo "⏳ Waiting for fork to be ready..."
MAX_FORK_ATTEMPTS=12
FORK_ATTEMPT=1
while [ $FORK_ATTEMPT -le $MAX_FORK_ATTEMPTS ]; do
  echo "Checking fork availability (attempt $FORK_ATTEMPT/$MAX_FORK_ATTEMPTS)..."
  if gh repo view "${FULL_REPO_NAME}" >/dev/null 2>&1; then
    echo "✅ Fork is ready!"
    break
  fi
  sleep 5
  FORK_ATTEMPT=$((FORK_ATTEMPT+1))
done

if [ $FORK_ATTEMPT -gt $MAX_FORK_ATTEMPTS ]; then
  echo "❌ Fork not ready after waiting. Exiting."
  exit 1
fi

# Prepare a clean working directory
if [ -d "${REPO_NAME}" ]; then
  rm -rf "${REPO_NAME}"
fi

# Clone the fork
echo "📥 Cloning the fork..."
git clone "https://github.com/${FULL_REPO_NAME}.git" "${REPO_NAME}"
cd "${REPO_NAME}"

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
          assignment-regex: >
            ^test/fixtures/assignments/(assignment-[\d]+)$,
            ^test/fixtures/homework/(hw-[\d]+)$,
            ^test/fixtures/labs/(lab-[\d]+)$,
            ^test/fixtures/bootcamp/(.+/assignment-[\w\-]+)$,
            ^test/fixtures/courses/(.+/assignment-[\w\-]+)$
          default-branch: main
          dry-run: "no"
EOF

# Commit and push the changes
echo "📝 Committing and pushing test workflow..."
git add .
git commit -m "Update workflow for testing"
git push origin main
echo "✅ Test workflow pushed"
