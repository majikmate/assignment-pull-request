package protect

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/majikmate/assignment-pull-request/internal/git"
)

// Lock represents a file-based lock for protect operations
type Lock struct {
	lockFile string
	file     *os.File
}

// acquireLock attempts to acquire an exclusive lock for protect operations
// This prevents concurrent protect-sync operations on the same repository
func acquireLock(repositoryRoot string) (*Lock, error) {
	// Use Git operations to find the actual git directory (handles worktrees, submodules, etc.)
	gitOps := git.NewOperationsWithDir(false, repositoryRoot)
	gitDir, err := gitOps.FindGitDir()
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
