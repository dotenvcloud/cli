package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// fileLock is a minimal cross-platform advisory lock implemented as a
// sibling `.lock` file created with O_EXCL. Two CLI processes saving the
// same config concurrently would otherwise race the atomic-rename step and
// silently drop one writer's edits.
type fileLock struct {
	path string
}

// acquire blocks until the lock file is created or the timeout fires. Stale
// locks older than staleAfter are reclaimed so a crashed process can't wedge
// future runs.
func acquireFileLock(path string, timeout, staleAfter time.Duration) (*fileLock, error) {
	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			f.Close()
			return &fileLock{path: path}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("failed to create lock %s: %w", path, err)
		}

		// Reclaim stale lock if older than threshold.
		if info, statErr := os.Stat(path); statErr == nil {
			if time.Since(info.ModTime()) > staleAfter {
				_ = os.Remove(path)
				continue
			}
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for config lock %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *fileLock) release() {
	if l == nil {
		return
	}
	_ = os.Remove(l.path)
}
