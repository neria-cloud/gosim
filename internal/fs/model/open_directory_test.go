package model

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestOpenResolvedDirectory(t *testing.T) {
	fs := NewLinuxFilesystem()
	defer fs.Release()
	app, err := fs.Getdirinode(RootInode, "/app")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "///", "/app/", "/app///", "./"} {
		inode, err := fs.OpenFile(app, path, os.O_RDONLY)
		if err != nil {
			t.Fatalf("open %q: %v", path, err)
		}
		want := app
		if path == "/" || path == "///" {
			want = RootInode
		}
		if inode != want {
			t.Fatalf("open %q = inode %d, want %d", path, inode, want)
		}
		if err := fs.CloseFile(inode); err != nil {
			t.Fatal(err)
		}
	}
	for _, flags := range []int{os.O_RDONLY, os.O_CREATE | os.O_RDWR} {
		if _, err := fs.OpenFile(app, "", flags); !errors.Is(err, syscall.ENOENT) {
			t.Fatalf("empty pathname: %v", err)
		}
	}
	inode, err := fs.OpenFile(app, "file", os.O_CREATE|os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.CloseFile(inode); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.OpenFile(app, "file/", os.O_RDONLY); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("file with trailing slash: %v", err)
	}
}
