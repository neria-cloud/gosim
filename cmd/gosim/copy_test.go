package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyPublishesFreshInodeAndPreservesOnFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "source"), filepath.Join(dir, "worker")
	if err := os.WriteFile(src, []byte("new executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatal(err)
	}
	oldBytes, err := io.ReadAll(old)
	if err != nil || string(oldBytes) != "old executable" {
		t.Fatal("copy overwrote an open executable inode", string(oldBytes), err)
	}
	newBytes, err := os.ReadFile(dst)
	if err != nil || string(newBytes) != "new executable" {
		t.Fatal("copy did not publish", err)
	}
	before, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode().Perm() != 0o755 {
		t.Fatal("lost executable permissions")
	}
	if err := copyFileIfChanged(src, dst, 0o755); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("identical file rewritten")
	}
	if err := copyFile(dir, dst, 0o755); err == nil {
		t.Fatal("directory read unexpectedly succeeded")
	}
	newBytes, err = os.ReadFile(dst)
	if err != nil || string(newBytes) != "new executable" {
		t.Fatal("failed copy damaged prior artifact", err)
	}
	temporary, err := filepath.Glob(filepath.Join(dir, ".gosim-copy-*"))
	if err != nil || len(temporary) != 0 {
		t.Fatal("leaked copy staging", temporary, err)
	}
}
