# Git Hooks Documentation

This document describes all supported Git hooks and their parameters in the
assignment-pull-request system.

## Overview

The assignment-pull-request system uses Git hooks to:

- Configure sparse checkout for assignment-based development
- Protect paths from unauthorized modifications
- Automatically manage repository state based on workflow configurations

## Supported Hook Types

The system supports the following Git hooks that modify the working tree:

### 1. post-checkout

**Triggered**: After a successful `git checkout` or `git switch`

**Parameters**:

- `$1` (previous HEAD): The ref of the previous HEAD
- `$2` (new HEAD): The ref of the new HEAD
- `$3` (branch checkout flag): `1` if checking out a branch, `0` if checking out
  a file

**Behavior**:

- **Sparse Checkout**: Only processed when `$3 = 1` (branch checkout)
- **Protected Paths**: Always processed
- **Use Cases**: Switch between assignment branches, initialize assignment
  workspace

**Example Usage**:

```bash
# Branch checkout - triggers both sparse checkout and path protection
git checkout assignment-1

# File checkout - only triggers path protection
git checkout HEAD -- somefile.txt
```

### 2. post-merge

**Triggered**: After a successful `git merge`

**Parameters**:

- `$1` (squash flag): `1` if the merge was a squash merge, `0` otherwise

**Behavior**:

- **Sparse Checkout**: Not processed
- **Protected Paths**: Always processed
- **Use Cases**: Merge changes while maintaining path protections

**Example Usage**:

```bash
# Regular merge - triggers path protection
git merge feature-branch

# Squash merge - triggers path protection
git merge --squash feature-branch
```

### 3. post-rewrite

**Triggered**: After commands that rewrite commits (`git rebase`,
`git commit --amend`)

**Parameters**:

- `$1` (command): The command that triggered the rewrite (`rebase` or `amend`)

**Behavior**:

- **Sparse Checkout**: Not processed
- **Protected Paths**: Always processed
- **Use Cases**: Maintain protections after commit rewrites

**Example Usage**:

```bash
# Interactive rebase - triggers path protection
git rebase -i HEAD~3

# Amend commit - triggers path protection
git commit --amend
```

### 4. post-applypatch

**Triggered**: After `git am` applies a patch

**Parameters**:

- No parameters

**Behavior**:

- **Sparse Checkout**: Not processed
- **Protected Paths**: Always processed
- **Use Cases**: Apply patches while maintaining protections

**Example Usage**:

```bash
# Apply patch series - triggers path protection for each patch
git am patch-series/*.patch
```

### 5. post-commit

**Triggered**: After a successful `git commit`

**Parameters**:

- No parameters

**Behavior**:

- **Sparse Checkout**: Not processed
- **Protected Paths**: Always processed
- **Use Cases**: Maintain protections after commits

**Example Usage**:

```bash
# Regular commit - triggers path protection
git commit -m "Add solution"

# Amend previous commit - triggers path protection
git commit --amend --no-edit
```

### 6. post-reset

**Triggered**: After `git reset`

**Parameters**:

- No parameters

**Behavior**:

- **Sparse Checkout**: Not processed
- **Protected Paths**: Always processed
- **Use Cases**: Maintain protections after reset operations

**Example Usage**:

```bash
# Soft reset - triggers path protection
git reset --soft HEAD~1

# Hard reset - triggers path protection
git reset --hard origin/main
```

## Hook Processing Logic

### Sparse Checkout Processing

Only triggered for `post-checkout` with branch checkout (`$3 = 1`):

1. Parse workflow files for `assignment-regex` patterns
2. Match current branch against assignment patterns
3. Configure sparse-checkout to show only matching assignment files
4. Hide other assignments and protected paths

### Protected Paths Processing

Triggered for all supported hooks:

1. Parse workflow files for `protected-paths-regex` patterns
2. Find all paths matching the patterns
3. Set ownership to `root:root` with restricted permissions
4. Prevent unauthorized modifications

## Workflow Configuration

Hooks read configuration from workflow YAML files (`.github/workflows/*.yml`):

```yaml
jobs:
    example:
        steps:
            - uses: majikmate/assignment-pull-request@v1
              with:
                  assignment-regex: |
                      ^assignments/(assignment-\d+)$
                      ^homework/(hw-\d+)$
                  protected-paths-regex: |
                      ^.devcontainer$
                      ^.github$
                      ^tutorials$
```

## Hook Installation

### Global Installation (DevContainer Feature)

Automatically installs all hooks via the `protected-paths` DevContainer feature:

```json
{
    "features": {
        "ghcr.io/majikmate/assignment-pull-request/protected-paths:1": {}
    }
}
```

### Manual Installation

Install specific hooks manually:

```bash
# Install post-checkout hook
curl -fsSL https://raw.githubusercontent.com/majikmate/assignment-pull-request/main/install-hook.sh | bash
```

## Hook Execution Flow

1. **Hook Trigger**: Git operation triggers one of the supported hooks
2. **Context Detection**: Determine hook type and repository root
3. **Workflow Parsing**: Parse `.github/workflows/*.yml` files for patterns
4. **Conditional Processing**:
   - Sparse checkout (post-checkout with branch only)
   - Protected paths (all supported hooks)
5. **Error Handling**: Log errors but don't fail Git operations

## Environment Variables

Hooks respect these environment variables:

- `GITHUB_REPOSITORY`: Repository identifier for GitHub operations
- `GITHUB_TOKEN`: Authentication token for GitHub API
- `GOBIN`: Custom Go binary installation path
- `GOPATH`: Go workspace path

## Debugging

Enable verbose logging:

```bash
# Set debug mode
export GIT_TRACE=1

# Run git operation to trigger hooks
git checkout assignment-1
```

Hook logs are written to stderr and can be viewed in the terminal output.

## Error Handling

Hooks are designed to be non-disruptive:

- **Installation Failures**: Skip hook execution, don't fail Git operation
- **Network Issues**: Use cached binaries, continue with degraded functionality
- **Permission Errors**: Log warnings, don't fail Git operation
- **Configuration Errors**: Use defaults, continue processing

## Security Considerations

- Hooks run as regular user but escalate to dedicated `prot` user for path
  protection
- Binary updates are verified against the official GitHub repository
- Protected paths are owned by `prot:prot` and cannot be modified by regular
  users
- Development user is added to `prot` group for read access to protected files
- Sparse checkout restrictions prevent access to hidden assignments
- No root privileges required - uses dedicated system user for security
  isolation

## Troubleshooting

### Common Issues

1. **Hook not executing**: Check if hooks are installed and executable
2. **Permission denied**: Ensure proper sudo configuration for path protection
3. **Binary not found**: Check Go installation and PATH configuration
4. **Network timeouts**: Hooks will use cached binaries

### Manual Verification

```bash
# Check installed hooks
ls -la .git/hooks/

# Test hook manually
.git/hooks/post-checkout prev_head new_head 1

# Verify sparse-checkout configuration
git sparse-checkout list
```
