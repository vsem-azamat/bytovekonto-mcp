package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPasswordFileWins(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "password")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BYTOVEKONTO_USER", "resident@example.com")
	t.Setenv("BYTOVEKONTO_PASSWORD", "from-env")
	t.Setenv("BYTOVEKONTO_PASSWORD_FILE", file)
	t.Setenv("BYTOVEKONTO_STATE_DIR", dir)
	c, err := loadConfig(true)
	if err != nil {
		t.Fatal(err)
	}
	if c.password != "from-file" {
		t.Errorf("password = %q, want the file's", c.password)
	}
	if _, ok := os.LookupEnv("BYTOVEKONTO_PASSWORD"); ok {
		t.Error("the password stayed in the environment")
	}
}

func TestMissingUser(t *testing.T) {
	t.Setenv("BYTOVEKONTO_USER", "")
	if _, err := loadConfig(true); err == nil || !strings.Contains(err.Error(), "BYTOVEKONTO_USER") {
		t.Fatalf("error = %v", err)
	}
}

func TestSecondServerIsLockedOut(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lockStateDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := lockStateDir(dir); err == nil {
		t.Fatal("a second lock on the same state directory succeeded")
	}
}
