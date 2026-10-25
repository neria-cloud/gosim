package behavior_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestGS01DarwinNativeDescriptorsMountAndSync(t *testing.T) {
	base := t.TempDir()
	file, err := os.Create(filepath.Join(base, "file"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("native"); err != nil {
		t.Fatal(err)
	}
	flags, err := unix.FcntlInt(file.Fd(), unix.F_GETFL, 0)
	if err != nil || flags&unix.O_ACCMODE != unix.O_RDWR {
		t.Fatal(flags, err)
	}
	if _, err := unix.FcntlInt(file.Fd(), unix.F_FULLFSYNC, 0); err != nil {
		t.Fatal(err)
	}
	var mount unix.Statfs_t
	if err := unix.Fstatfs(int(file.Fd()), &mount); err != nil {
		t.Fatal(err)
	}
	if unix.ByteSliceToString(mount.Mntonname[:]) == "" {
		t.Fatal("native mount identity missing")
	}
	for n := 0; n < 140; n++ {
		name := fmt.Sprintf("%041d", n)
		if err := os.WriteFile(filepath.Join(base, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(base)
	if err != nil || len(entries) != 141 {
		t.Fatal("native listing inherited simulator bound", len(entries), err)
	}
}
