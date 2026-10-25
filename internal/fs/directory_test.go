package fs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func TestDirectoryBatchIsAtomicAndRewindable(t *testing.T) {
	i, ctx, table := ioFixture(t)
	ctx.PC = 0x1234
	must(t, i.Mkdirat(ctx, atFDCWD, "listing", 0o700))
	fd := open(t, i, ctx, "listing", openDirectory)
	_, err := i.Openat(ctx, fd, "a", openCreate|2, 0o600)
	must(t, err)
	must(t, i.Mkdirat(ctx, fd, "dir", 0o700))
	must(t, i.Symlinkat(ctx, "a", fd, "ln"))
	resource, err := table.Lookup(fd)
	must(t, err)
	f := resource.(*File)
	// 21 + 23 + 22 bytes in the documented Linux dirent64 encoding.
	const total = 66
	buf := bytes.Repeat([]byte{0xa5}, total)
	for _, size := range []int{0, 21, total - 1} {
		before := bytes.Clone(buf)
		n, err := i.Getdents64(ctx, fd, abi.ByteView{Ptr: buf[:size]})
		var failure *contracts.Failure
		if n != 0 || !errors.As(err, &failure) || failure.Code != contracts.UnsupportedOperation {
			t.Fatalf("size %d: %d, %v", size, n, err)
		}
		if failure.Diagnostic.Feature != "fs" || failure.Diagnostic.Operation != "getdents64" || failure.Diagnostic.Epoch != ctx.Epoch || failure.Diagnostic.PC != ctx.PC {
			t.Fatal(failure.Diagnostic)
		}
		if !bytes.Equal(buf, before) || f.didReaddir || f.pos != 0 {
			t.Fatal("overflow changed buffer or cursor")
		}
	}
	n, err := i.Getdents64(ctx, fd, abi.ByteView{Ptr: buf})
	must(t, err)
	if n != total || !f.didReaddir {
		t.Fatal(n, f.didReaddir)
	}
	names, types := []string{}, []byte{}
	for offset := 0; offset < n; {
		length := int(binary.LittleEndian.Uint16(buf[offset+16 : offset+18]))
		if length < 20 || offset+length > n {
			t.Fatal("invalid dirent length", length)
		}
		names = append(names, string(bytes.TrimRight(buf[offset+19:offset+length], "\x00")))
		types = append(types, buf[offset+18])
		offset += length
	}
	if !reflect.DeepEqual(names, []string{"a", "dir", "ln"}) || !bytes.Equal(types, []byte{8, 4, 10}) {
		t.Fatal(names, types)
	}
	first := bytes.Clone(buf)
	if n, err := i.Getdents64(ctx, fd, abi.ByteView{Ptr: buf}); err != nil || n != 0 {
		t.Fatal("missing EOF", n, err)
	}
	for _, test := range []struct {
		offset int64
		whence int
	}{{0, io.SeekCurrent}, {0, io.SeekEnd}, {1, io.SeekStart}, {-1, io.SeekStart}} {
		if _, err := i.Seek(ctx, fd, test.offset, test.whence); !errors.Is(err, syscall.EINVAL) {
			t.Fatal(err)
		}
		if !f.didReaddir {
			t.Fatal("invalid seek reset cursor")
		}
	}
	pos, err := i.Seek(ctx, fd, 0, io.SeekStart)
	must(t, err)
	if pos != 0 || f.didReaddir {
		t.Fatal(pos, f.didReaddir)
	}
	n, err = i.Getdents64(ctx, fd, abi.ByteView{Ptr: buf})
	must(t, err)
	if n != total || !bytes.Equal(buf, first) {
		t.Fatal("rewind did not reproduce listing")
	}
	second := open(t, i, ctx, "listing", openDirectory)
	n, err = i.Getdents64(ctx, second, abi.ByteView{Ptr: buf})
	must(t, err)
	if n != total || !bytes.Equal(buf, first) {
		t.Fatal("handles share a cursor")
	}
}

func TestDirectoryBatchResourceValidationAndEmptyEOF(t *testing.T) {
	i, ctx, table := ioFixture(t)
	must(t, i.Mkdirat(ctx, atFDCWD, "empty", 0o700))
	dir := open(t, i, ctx, "empty", openDirectory)
	n, err := i.Getdents64(ctx, dir, abi.ByteView{})
	must(t, err)
	if n != 0 {
		t.Fatal(n)
	}
	resource, err := table.Lookup(dir)
	must(t, err)
	file := resource.(*File)
	file.read = false
	if _, err := i.Getdents64(ctx, dir, abi.ByteView{}); !errors.Is(err, syscall.EBADF) {
		t.Fatal("EOF bypassed read access", err)
	}
	file.read = true
	_, err = table.Remove(dir)
	must(t, err)
	must(t, resource.Release(contracts.CloseExplicit))
	if _, err := i.Getdents64(ctx, dir, abi.ByteView{}); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
	regular := open(t, i, ctx, "file", openCreate|2)
	if _, err := i.Getdents64(ctx, regular, abi.ByteView{}); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatal(err)
	}
}
