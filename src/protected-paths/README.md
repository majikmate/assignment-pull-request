# Protected Paths (Git) - DevContainer Feature

A DevContainer feature that enforces root-owned protected paths and
assignment-based sparse checkout via global git hooks. Automatically configures
based on workflow YAML files in the repository.

## Features

- **Assignment-based Sparse Checkout**: Automatically configures git
  sparse-checkout based on assignment patterns when checking out branches
- **Protected Path Synchronization**: Enforces root ownership and permissions on
  specified paths across all git operations that modify the working tree
- **Workflow-driven Configuration**: Automatically reads configuration from
  GitHub Actions workflow files (`.github/workflows/*.yml`)
- **Multi-hook Support**: Handles various git hooks (`post-checkout`,
  `post-merge`, `post-rewrite`, `post-commit`, etc.)
- **Go Implementation**: Modern, efficient implementation with bash backup
  scripts for compatibility

## Installation

Add this feature to your `.devcontainer/devcontainer.json`:

```json
{
    "features": {
        "ghcr.io/majikmate/assignment-pull-request/protected-paths:1": {}
    }
}
```

## How It Works

1. **Global Git Hooks**: Installs hooks at `/etc/git/hooks` that trigger on git
   operations
2. **Dynamic Configuration**: Reads `assignment-regex` and
   `protected-paths-regex` from workflow YAML files
3. **Sparse Checkout**: For `post-checkout` with branch changes, configures
   sparse-checkout to show only relevant assignment folders
4. **Path Protection**: For all working-tree-modifying hooks, synchronizes
   protected paths from HEAD with root ownership
5. **Skip Worktree**: Applies git skip-worktree flags to prevent accidental
   modifications of protected files

## Configuration

No manual configuration required! The feature automatically:

- Reads regex patterns from your repository's workflow YAML files
- Looks for `assignment-regex` patterns to determine assignment folders
- Looks for `protected-paths-regex` patterns to determine protected paths
- Configures appropriate permissions and ownership

## Example Workflow Configuration

```yaml
# .github/workflows/assignments.yml
name: Process Assignments
on: [push]
jobs:
    process:
        uses: majikmate/assignment-pull-request/.github/workflows/process-assignments.yml@main
        with:
            assignment-regex: |
                test/fixtures/(?P<category>[^/]+)/(?P<number>\d+)-(?P<name>[^/]+)/README\.md$
                assignments/(?P<semester>[^/]+)/(?P<module>[^/]+)/(?P<assignment>[^/]+)/README\.md$
            protected-paths-regex: |
                ^test/fixtures/
                ^\.github/
```

## Components

- **Go Binary (`githook`)**: Main implementation that handles all logic
- **Bash Hook (`protect-sync-hook`)**: Lightweight hook that calls the Go binary
- **Backup Script (`protect-sync`)**: Legacy bash implementation kept for
  compatibility
- **Sudo Configuration**: Grants minimal required privileges for path protection

## Security

- Runs with minimal required privileges
- Only grants sudo access to specific binaries and commands
- Preserves git operation integrity
- Non-failing hooks (git operations continue even if hooks fail)

## Compatibility

- **Linux containers** (uses `rsync`, `chown`, `chmod`)
- **Go 1.19+** required
- **Git 2.25+** recommended
- Works with any git repository structure

## Development

The feature is part of the
[assignment-pull-request](https://github.com/majikmate/assignment-pull-request)
project.
