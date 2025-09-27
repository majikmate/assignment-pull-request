package protect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Lock represents a file-based lock for protect operations
type Lock struct {
	lockFile string
	file     *os.File
}

// acquireLock attempts to acquire an exclusive lock for protect operations
// This prevents concurrent protect-sync operations on the same repository
func acquireLock(repositoryRoot string) (*Lock, error) {
	// Use Git to find the actual git directory (handles worktrees, submodules, etc.)
	gitDir, err := findGitDir(repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to find git directory: %w", err)
	}
	
	lockFile := filepath.Join(gitDir, "protect-paths.lock")
	
	// Try to acquire lock with timeout
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err == nil {
			// Successfully created lock file, write our PID
			pid := os.Getpid()
			_, writeErr := file.WriteString(strconv.Itoa(pid) + "\n")
			if writeErr != nil {
				file.Close()
				os.Remove(lockFile)
				return nil, fmt.Errorf("failed to write PID to lock file: %w", writeErr)
			}
			
			return &Lock{lockFile: lockFile, file: file}, nil
		}
		
		// If file exists, check if the process is still alive
		if os.IsExist(err) {
			if pid, err := readLockPID(lockFile); err == nil {
				if !isProcessRunning(pid) {
					// Stale lock, remove it
					os.Remove(lockFile)
					continue
				}
			}
		}
		
		// Wait a bit and try again
		time.Sleep(100 * time.Millisecond)
	}
	
	return nil, fmt.Errorf("timeout waiting for protect-paths lock (another operation may be in progress)")
}

// Release releases the lock
func (l *Lock) Release() error {
	if l.file != nil {
		l.file.Close()
	}
	return os.Remove(l.lockFile)
}

// readLockPID reads the PID from a lock file
func readLockPID(lockFile string) (int, error) {
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return 0, err
	}
	
	pid, err := strconv.Atoi(string(data[:len(data)-1])) // Remove newline
	return pid, err
}

// isProcessRunning checks if a process with given PID is still running
func isProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// findGitDir locates the actual git directory for the repository
// This handles git worktrees, submodules, and other Git configurations
// where .git might not be a directory in the repository root
func findGitDir(repositoryRoot string) (string, error) {
	// Use git rev-parse --git-dir to find the actual git directory
	// This handles all Git configurations correctly
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repositoryRoot
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to find git directory: %w", err)
	}
	
	gitDir := strings.TrimSpace(string(output))
	
	// If gitDir is relative, make it absolute relative to repositoryRoot
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repositoryRoot, gitDir)
	}
	
	// Clean the path to remove any redundant elements
	gitDir = filepath.Clean(gitDir)
	
	// Verify the git directory exists and is accessible
	if _, err := os.Stat(gitDir); err != nil {
		return "", fmt.Errorf("git directory not accessible: %w", err)
	}
	
	return gitDir, nil
}