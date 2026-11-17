package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Lock files: the server and the watcher each keep their own token, so each needs one process.
const (
	serveLock = "serve.lock"
	watchLock = "watch.lock"
)

// lockStateDir makes sure only one process of a kind (server or watcher) uses the state
// directory: two would share one cached token, and a refresh by one would invalidate the other's.
func lockStateDir(dir, name string) (func(), error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("state directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // a directory needs x; 0700 is owner only
		return nil, fmt.Errorf("state directory: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // the state directory is configuration
	if err != nil {
		return nil, fmt.Errorf("state directory lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another bytovekonto-mcp %s uses %s", strings.TrimSuffix(name, ".lock"), dir)
		}
		return nil, fmt.Errorf("state directory lock: %w", err)
	}
	return func() { f.Close() }, nil
}
