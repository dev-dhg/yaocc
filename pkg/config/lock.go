package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
)

const lockFileName = "yaocc.lock"

// lockGracePeriod is how long a lock remains valid after being written.
// This must be longer than the watcher poll interval (3s) to ensure
// the watcher sees the lock before it expires.
const lockGracePeriod = 5 * time.Second

// getLockFilePath returns the full path to the lock file.
func getLockFilePath() string {
	// Load .env to ensure YAOCC_CONFIG_DIR is available
	_ = godotenv.Load()
	configDir := ResolveConfigDir()
	return filepath.Join(configDir, lockFileName)
}

// AcquireConfigLock creates a lock file to signal that a config update is in progress.
// This is used to prevent the watcher from triggering a reload during self-initiated updates.
func AcquireConfigLock() error {
	lockPath := getLockFilePath()
	f, err := os.Create(lockPath)
	if err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	f.Close()
	return nil
}

// ReleaseConfigLock is a no-op. The lock file is left in place and expires
// based on its modification time (see IsConfigLocked). This avoids a race
// condition where the lock is deleted before the file watcher has a chance
// to poll and see it.
func ReleaseConfigLock() error {
	// Intentionally left as no-op.
	// The lock expires automatically via the grace period checked in IsConfigLocked.
	return nil
}

// IsConfigLocked checks if a config update is in progress by looking at the
// lock file's modification time. The lock is considered active if the file
// exists and was modified within the last lockGracePeriod.
func IsConfigLocked() bool {
	lockPath := getLockFilePath()
	info, err := os.Stat(lockPath)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < lockGracePeriod
}
