package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// tokenStore caches the token pair in a file (0600 in a 0700 directory), so a restart needs no
// password login. The file is a secret.
type tokenStore struct {
	path     string // "" keeps the token in memory only
	readOnly bool
	mu       sync.Mutex
	mem      token
}

func (s *tokenStore) load(user string) token {
	empty := token{User: user}
	if s.path == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.mem.User == user {
			return s.mem
		}
		return empty
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		return empty
	}
	var t token
	if json.Unmarshal(b, &t) != nil || t.User != user {
		return empty
	}
	return t
}

func (s *tokenStore) save(t token) error {
	if s.readOnly {
		return nil
	}
	if s.path == "" {
		s.mu.Lock()
		s.mem = t
		s.mu.Unlock()
		return nil
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("token directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // a directory needs x; 0700 is owner only
		return fmt.Errorf("token directory: %w", err)
	}
	b, err := json.Marshal(t) //nolint:gosec // the token cache is meant to hold the token; the file is 0600
	if err != nil {
		return fmt.Errorf("token file: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".token-*")
	if err != nil {
		return fmt.Errorf("token file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("token file: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("token file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("token file: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("token file: %w", err)
	}
	return nil
}

func (s *tokenStore) remove() error {
	if s.path == "" {
		s.mu.Lock()
		s.mem = token{}
		s.mu.Unlock()
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("token file: %w", err)
	}
	return nil
}
