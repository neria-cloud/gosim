package behavior_test

import (
	byteutil "bytes"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/unix"

	"github.com/neria-cloud/gosim"
)

const (
	diskFileMode      = 0o644
	diskDirMode       = 0o755
	diskHello         = "hello"
	diskGoodbye       = "goodbye"
	diskFoo           = "foo"
	diskBar           = "bar"
	diskErrorField    = "err"
	diskBadRead       = "bad read"
	diskHelloLength   = 5
	diskGoodbyeLength = 7
	diskSeekPastEnd   = 10
	diskWorldOffset   = 8
	diskShortRead     = 3
	diskWorldEnd      = 13
	diskSeekGapEnd    = 15
	diskAfterGoodbye  = 22
	diskBackTwo       = 20
	diskBackFive      = 17
	diskShortLength   = 3
	mmapPageSize      = 4096
	mmapHalfPage      = 2048
	mmapSmallSize     = 1024
	mmapInitialLength = 10
	mmapStraddleSize  = 8
	mmapStraddleGap   = 3
	mmapTailLength    = 96
	mmapExtendedWrite = 200
	mmapNearEnd       = 4000
	mmapWriteLength   = 40
	mmapReadLength    = 128
	mmapWideLength    = 256
	mmapEndOffset     = 20
)

func setupRealDisk(t *testing.T) {
	if gosim.IsSim() {
		return
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return
	}

	tmp, err := os.MkdirTemp("", t.Name())
	if err != nil {
		t.Error(err)
		return
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(tmp); err != nil {
			panic(err)
		}
	})

	if err := os.Chdir(tmp); err != nil {
		t.Error(err)
		return
	}

	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			panic(err)
		}
	})
}

func TestDiskBasic(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	if err := f.Sync(); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte(diskHello)) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskTruncateGrow(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	if err := f.Truncate(diskSeekPastEnd); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte("hello\x00\x00\x00\x00\x00")) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskTruncateShrink(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	if err := f.Truncate(diskShortLength); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte("hel")) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskOpenTruncate(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}
	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_WRONLY|os.O_TRUNC, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	if _, err := f.Write([]byte(diskGoodbye)); err != nil {
		t.Error(err)
		return
	}
	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte(diskGoodbye)) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskSyncDir(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(".", os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	// XXX: you canc sync a O_RDONLY dir?
	if err := f.Sync(); err != nil {
		t.Error(err)
		return
	}
}

func TestDiskAfterClose(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	checkClosedError := func(err error, op string) {
		t.Helper()
		pathErr, ok := err.(*os.PathError)
		if !ok {
			t.Error("not a path error")
			return
		}
		if pathErr.Op != op {
			t.Errorf("expected op %q, got %q", op, pathErr.Op)
		}
		if pathErr.Err != os.ErrClosed {
			t.Errorf("expected os.ErrClosed, got %v", pathErr.Err)
		}
		if pathErr.Path != diskHello {
			t.Errorf("expected \"hello\", got %q", pathErr.Path)
		}
	}

	err = f.Close()
	checkClosedError(err, "close")
	slog.Info("return value of close after close", diskErrorField, err.Error())

	n, err := f.Read(nil)
	checkClosedError(err, "read")
	slog.Info("return value of read after close", "n", n, diskErrorField, err.Error())

	n, err = f.Write([]byte(diskHello))
	checkClosedError(err, "write")
	slog.Info("return value of write after close", "n", n, diskErrorField, err.Error())

	err = f.Sync()
	checkClosedError(err, "sync")
	slog.Info("return value of sync after close", diskErrorField, err.Error())
}

func TestDiskWriteAtEndOfFile(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.WriteAt([]byte(diskHello), 0); err != nil {
		t.Error(err)
		return
	}

	if _, err := f.WriteAt([]byte(diskHello), diskShortLength); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte("helhello")) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskWriteAtAfterEndOfFile(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.WriteAt([]byte(diskHello), diskShortLength); err != nil {
		t.Error(err)
		return
	}

	if _, err := f.WriteAt([]byte(diskHello), 3+5+3); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte("\x00\x00\x00hello\x00\x00\x00hello")) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskReadAtBasic(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.WriteAt([]byte(diskHello), 0); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	data := make([]byte, diskHelloLength)
	n, err := f.ReadAt(data, 0)
	if err != nil {
		t.Error(err)
	}
	if n != diskHelloLength || !byteutil.Equal(data, []byte(diskHello)) {
		t.Error(diskBadRead, n, data)
	}

	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskReadAtEndOfFile(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.WriteAt([]byte(diskHello), 0); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	{
		data := make([]byte, diskHelloLength)
		n, err := f.ReadAt(data, diskShortLength)
		if err != io.EOF {
			t.Error(err)
		}
		if n != 2 || !byteutil.Equal(data, []byte("lo\x00\x00\x00")) {
			t.Error(diskBadRead, n, data)
		}
	}

	{
		data := make([]byte, diskHelloLength)
		n, err := f.ReadAt(data, diskHelloLength)
		if err != io.EOF {
			t.Error(err)
		}
		if n != 0 || !byteutil.Equal(data, []byte("\x00\x00\x00\x00\x00")) {
			t.Error(diskBadRead, n, data)
		}
	}

	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskReadAtAfterEndOfFile(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.WriteAt([]byte(diskHello), 0); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskHello, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	data := make([]byte, diskHelloLength)
	n, err := f.ReadAt(data, diskSeekPastEnd)
	if err != io.EOF {
		t.Error(err)
	}
	if n != 0 || !byteutil.Equal(data, []byte("\x00\x00\x00\x00\x00")) {
		t.Error(diskBadRead, n, data)
	}

	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskSeekBasic(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_RDWR, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	ok := func(offset int64, whence int, expected int64) {
		t.Helper()

		got, err := f.Seek(offset, whence)
		if err != nil {
			t.Errorf("seek %d %d: %v", offset, whence, err)
		}
		if got != expected {
			t.Errorf("seek %d %d: got %d, expected %d", offset, whence, got, expected)
		}
	}

	bad := func(offset int64, whence int, expected error) {
		t.Helper()

		got, err := f.Seek(offset, whence)
		if err == nil || !errors.Is(err, expected) {
			t.Errorf("seek %d %d: got err %v, expected %v", offset, whence, err, expected)
		}
		if got != 0 {
			t.Errorf("seek %d %d: expected 0 after failure, got %d", offset, whence, got)
		}
	}

	// get current pos
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Error(err)
	}
	if pos != 0 {
		t.Error(pos)
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	// get current pos
	ok(0, io.SeekCurrent, diskHelloLength)

	// seek to start
	ok(0, io.SeekStart, 0)
	ok(0, io.SeekCurrent, 0)

	// read from current pos
	buf := make([]byte, diskHelloLength)
	if n, err := f.Read(buf); err != nil || n != diskHelloLength {
		t.Error(n, err)
	}
	if string(buf) != diskHello {
		t.Error(string(buf))
	}
	ok(0, io.SeekCurrent, diskHelloLength)

	// seek to start
	ok(0, io.SeekStart, 0)
	ok(0, io.SeekCurrent, 0)

	// seek to end
	ok(0, io.SeekEnd, diskHelloLength)
	ok(0, io.SeekCurrent, diskHelloLength)

	// seek after end
	ok(diskHelloLength, io.SeekEnd, diskSeekPastEnd)
	ok(0, io.SeekCurrent, diskSeekPastEnd)

	// read in nowhere and stay
	if n, err := f.Read(buf); err != io.EOF || n != 0 {
		t.Error(n, err)
	}
	ok(0, io.SeekCurrent, diskSeekPastEnd)

	// writeat over current and read
	if n, err := f.WriteAt([]byte("world"), diskWorldOffset); err != nil || n != diskHelloLength {
		t.Error(n, err)
	}
	if n, err := f.Read(buf); err != nil || n != diskShortRead {
		t.Error(n, err)
	}
	if string(buf[0:diskShortRead]) != "rld" {
		t.Error(buf[0:diskShortRead])
	}
	ok(0, io.SeekCurrent, diskWorldEnd)

	// seek from start and read
	ok(diskWorldOffset, io.SeekStart, diskWorldOffset)
	if n, err := f.Read(buf); err != nil || n != diskHelloLength {
		t.Error(n, err)
	}
	if string(buf) != "world" {
		t.Error(buf)
	}
	ok(0, io.SeekCurrent, diskWorldEnd)

	// seek after end and write
	ok(2, io.SeekEnd, diskSeekGapEnd)
	ok(0, io.SeekCurrent, diskSeekGapEnd)
	if n, err := f.Write([]byte(diskGoodbye)); err != nil || n != diskGoodbyeLength {
		t.Error(err)
	}
	ok(0, io.SeekCurrent, diskAfterGoodbye)

	// invalid seek from start
	bad(-2, io.SeekStart, syscall.EINVAL)
	ok(0, io.SeekCurrent, diskAfterGoodbye)

	// invalid seek from current
	bad(-23, io.SeekCurrent, syscall.EINVAL)
	ok(0, io.SeekCurrent, diskAfterGoodbye)

	// invalid seek from end
	pos, err = f.Seek(-23, io.SeekEnd)
	bad(-23, io.SeekEnd, syscall.EINVAL)
	ok(0, io.SeekCurrent, diskAfterGoodbye)

	// ok negative seek
	ok(-2, io.SeekCurrent, diskBackTwo)
	ok(0, io.SeekCurrent, diskBackTwo)

	// ok negative seek
	ok(-5, io.SeekEnd, diskBackFive)
	ok(0, io.SeekCurrent, diskBackFive)
}

func TestDiskOpenMissingFile(t *testing.T) {
	setupRealDisk(t)

	_, err := os.OpenFile(diskFoo, os.O_RDONLY, diskFileMode)
	if err == nil {
		t.Error(err)
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Error("should be not exist")
	}
	slog.Info("error opening a non-existing file", diskErrorField, err.Error())
}

func TestDiskStatMissingFile(t *testing.T) {
	setupRealDisk(t)

	_, err := os.Stat(diskFoo)
	if err == nil {
		t.Error(err)
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Error("should be not exist")
	}
	slog.Info("error stat a non-existing file", diskErrorField, err.Error())
}

func TestDiskStatFile(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskFoo, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	fi, err := os.Stat(diskFoo)
	if err != nil {
		t.Error(err)
		return
	}

	if fi.IsDir() {
		t.Error("should not be dir, not", fi.IsDir())
	}
	if fi.Name() != diskFoo {
		t.Error("should be foo, not ", fi.Name())
	}
	expectedSize := int64(len(diskHello))
	if gotSize := fi.Size(); gotSize != expectedSize {
		t.Errorf("bad size: got %d, expected %d", gotSize, expectedSize)
	}
}

func TestDiskStatFd(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskFoo, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	fi, err := f.Stat()
	if err != nil {
		t.Error(err)
		return
	}

	if fi.IsDir() {
		t.Error("should not be dir, not", fi.IsDir())
	}
	if fi.Name() != diskFoo {
		t.Error("should be foo, not ", fi.Name())
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	fi2, err := os.Stat(diskFoo)
	if err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(fi, fi2) {
		t.Error("different")
	}
}

func TestDiskDeleteMissingFile(t *testing.T) {
	setupRealDisk(t)

	err := os.Remove(diskFoo)
	if err == nil {
		t.Error(err)
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Error("should be not exist", err)
	}
	slog.Info("error deleting a non-existing file", diskErrorField, err.Error())
}

func TestDiskDeleteFile(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskFoo, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	if _, err := os.Stat(diskFoo); err != nil {
		t.Error("missing file before delete", err)
		return
	}

	if err := os.Remove(diskFoo); err != nil {
		t.Error("remove failed", err)
		return
	}

	if _, err := os.Stat(diskFoo); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Error("delete did not work", err)
		return
	}
}

func TestDiskRenameFile(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskFoo, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	if _, err := os.Stat(diskFoo); err != nil {
		t.Error("missing file before rename", err)
		return
	}

	if err := os.Rename(diskFoo, diskBar); err != nil {
		t.Error("rename failed", err)
		return
	}

	if _, err := os.Stat(diskFoo); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Error("file still there after rename", err)
		return
	}

	f, err = os.OpenFile(diskBar, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte(diskHello)) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

func TestDiskRenameMissingFile(t *testing.T) {
	setupRealDisk(t)

	err := os.Rename(diskFoo, diskBar)
	if err == nil {
		t.Error(err)
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Error("should be not exist")
	}
	slog.Info("error renaming a non-existing file", diskErrorField, err.Error())
}

func TestDiskRenameFileReplace(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskFoo, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	f, err = os.OpenFile(diskBar, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}

	if _, err := f.Write([]byte("old")); err != nil {
		t.Error(err)
		return
	}

	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	if _, err := os.Stat(diskFoo); err != nil {
		t.Error("missing file before rename", err)
		return
	}

	if _, err := os.Stat(diskBar); err != nil {
		t.Error("missing file before rename", err)
		return
	}

	if err := os.Rename(diskFoo, diskBar); err != nil {
		t.Error("rename failed", err)
		return
	}

	if _, err := os.Stat(diskFoo); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Error("file still there after rename", err)
		return
	}

	f, err = os.OpenFile(diskBar, os.O_RDONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Error(err)
	}
	if !byteutil.Equal(data, []byte(diskHello)) {
		t.Error(diskBadRead, data)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
}

type dirEntry struct {
	Name  string
	IsDir bool
}

func checkReadDir(t *testing.T, dir string, expected []dirEntry) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Error(err)
		return
	}

	var converted []dirEntry
	for _, entry := range entries {
		converted = append(converted, dirEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}

	if diff := cmp.Diff(expected, converted); diff != "" {
		t.Error(diff)
	}
}

func TestDiskReadDir(t *testing.T) {
	setupRealDisk(t)

	checkReadDir(t, ".", nil)

	f, err := os.OpenFile(diskFoo, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}
	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	checkReadDir(t, ".", []dirEntry{
		{
			Name:  diskFoo,
			IsDir: false,
		},
	})

	f, err = os.OpenFile(diskBar, os.O_CREATE|os.O_WRONLY, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	if _, err := f.Write([]byte(diskHello)); err != nil {
		t.Error(err)
		return
	}
	if err := f.Close(); err != nil {
		t.Error(err)
		return
	}

	checkReadDir(t, ".", []dirEntry{
		{
			Name:  diskBar,
			IsDir: false,
		},
		{
			Name:  diskFoo,
			IsDir: false,
		},
	})

	if err := os.Rename(diskBar, "zz"); err != nil {
		t.Error(err)
		return
	}

	checkReadDir(t, ".", []dirEntry{
		{
			Name:  diskFoo,
			IsDir: false,
		},
		{
			Name:  "zz",
			IsDir: false,
		},
	})

	if err := os.Remove(diskFoo); err != nil {
		t.Error(err)
		return
	}

	checkReadDir(t, ".", []dirEntry{
		{
			Name:  "zz",
			IsDir: false,
		},
	})
}

func TestMmapBasic(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_RDWR, diskFileMode)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()

	if _, err := f.WriteAt([]byte(diskHello), 0); err != nil {
		t.Error(err)
	}

	data, err := unix.Mmap(int(f.Fd()), 0, mmapPageSize, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unix.Munmap(data); err != nil {
			t.Error(err)
		}
	}()

	if got := string(data[0:diskHelloLength]); got != diskHello {
		t.Errorf("expected hello, got %q", got)
	}

	if _, err := f.WriteAt([]byte(diskGoodbye), 0); err != nil {
		t.Error(err)
	}

	if got := string(data[0:diskGoodbyeLength]); got != diskGoodbye {
		t.Errorf("expected goodbye, got %q", got)
	}

	if err := f.Truncate(mmapHalfPage); err != nil {
		t.Error(err)
	}
	n, err := f.WriteAt([]byte("straddle"), mmapHalfPage-mmapStraddleGap)
	if n != mmapStraddleSize || err != nil {
		t.Errorf("tried to write, got %d %v", n, err)
	}
	if got := string(data[mmapHalfPage-mmapStraddleGap : mmapHalfPage-mmapStraddleGap+mmapStraddleSize]); got != "straddle" {
		t.Errorf("expected straddle, got %q", got)
	}

	if err := f.Truncate(mmapSmallSize); err != nil {
		t.Error(err)
	}

	if got := string(data[mmapHalfPage-mmapStraddleGap : mmapHalfPage-mmapStraddleGap+mmapStraddleSize]); got != "\x00\x00\x00\x00\x00\x00\x00\x00" {
		t.Errorf("expected zeroes, got %q", got)
	}
}

func TestDiskDirBasic(t *testing.T) {
	setupRealDisk(t)

	if err := os.Mkdir(diskFoo, diskDirMode); err != nil {
		t.Error(err)
	}

	if err := os.WriteFile("foo/bar", []byte("baz"), diskFileMode); err != nil {
		t.Error(err)
	}

	bytes, err := os.ReadFile("foo/bar")
	if err != nil {
		t.Error(err)
	}
	if string(bytes) != "baz" {
		t.Errorf("unexpected read %q", string(bytes))
	}

	if _, err := os.ReadFile("foo/bar/baz"); !errors.Is(err, syscall.ENOTDIR) {
		t.Error(err)
	}
	if _, err := os.ReadFile("foo/bar/baz/bah"); !errors.Is(err, syscall.ENOTDIR) {
		t.Error(err)
	}

	checkReadDir(t, ".", []dirEntry{
		{
			Name:  diskFoo,
			IsDir: true,
		},
	})

	checkReadDir(t, diskFoo, []dirEntry{
		{
			Name:  diskBar,
			IsDir: false,
		},
	})

	before, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}

	if err := os.Chdir(diskFoo); err != nil {
		t.Error(err)
	}

	after, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}

	if filepath.Join(before, diskFoo) != after {
		t.Error(before, filepath.Join(before, diskFoo), after)
	}

	if err := os.Rename("../foo", "../huh"); err != nil {
		t.Error(err)
	}

	after2, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}

	if filepath.Join(before, "huh") != after2 {
		t.Error(before, filepath.Join(before, "huh"), after2)
	}

	bytes, err = os.ReadFile(diskBar)
	if err != nil {
		t.Error(err)
	}
	if string(bytes) != "baz" {
		t.Errorf("unexpected read %q", string(bytes))
	}

	checkReadDir(t, ".", []dirEntry{
		{
			Name:  diskBar,
			IsDir: false,
		},
	})

	if err := os.Chdir(".."); err != nil {
		t.Error(err)
	}

	if err := os.Remove("huh"); !errors.Is(err, syscall.ENOTEMPTY) {
		t.Error(err)
	}

	if err := os.Remove("huh/bar"); err != nil {
		t.Error(err)
	}

	if err := os.Remove("huh"); err != nil {
		t.Error(err)
	}
}

type mmapTester struct {
	file     *os.File
	expected []byte
	mmaps    map[string][]byte
}

func (f *mmapTester) truncate(t *testing.T, n int) {
	t.Logf("truncating %d", n)

	if err := f.file.Truncate(int64(n)); err != nil {
		t.Fatal(n)
	}
	if n > len(f.expected) {
		f.expected = append(f.expected, make([]byte, n-len(f.expected))...)
	}
	if n < len(f.expected) {
		f.expected = f.expected[:n]
	}
}

func (f *mmapTester) readAt(t *testing.T, pos, n int) {
	t.Logf("reading %d %d", pos, n)

	expected := make([]byte, n)
	if pos <= len(f.expected) {
		copy(expected, f.expected[pos:])
	}

	for name, mmap := range f.mmaps {
		t.Logf("checking mmap %s", name)

		if pos >= len(mmap) {
			t.Logf("out of range, skipping")
			continue
		}

		got := mmap[pos:min(pos+n, len(mmap))]
		if !byteutil.Equal(expected[:len(got)], got) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	}
}

func (f *mmapTester) writeAt(t *testing.T, pos, n int) {
	t.Logf("writing %d %d", pos, n)

	b := make([]byte, n)
	rand.Read(b)

	m, err := f.file.WriteAt(b, int64(pos))
	if err != nil || m != n {
		t.Errorf("expected ok write, got %d %v", m, err)
	}

	if pos+n > len(f.expected) {
		// append zeroes
		f.expected = append(f.expected, make([]byte, pos+n-len(f.expected))...)
	}
	copy(f.expected[pos:], b)
}

func (f *mmapTester) mmap(t *testing.T, name string, n int) {
	mmap, err := unix.Mmap(int(f.file.Fd()), 0, n, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		t.Fatal(err)
	}
	f.mmaps[name] = mmap
}

func (f *mmapTester) munmap(t *testing.T, name string) {
	if err := unix.Munmap(f.mmaps[name]); err != nil {
		t.Fatal(err)
	}
	delete(f.mmaps, name)
}

func TestMmapComplex(t *testing.T) {
	setupRealDisk(t)

	f, err := os.OpenFile(diskHello, os.O_CREATE|os.O_RDWR, diskFileMode)
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()

	mt := &mmapTester{
		file:     f,
		expected: []byte{},
		mmaps:    make(map[string][]byte),
	}

	mt.readAt(t, 0, mmapInitialLength)
	mt.writeAt(t, 0, mmapInitialLength)
	mt.readAt(t, 0, mmapInitialLength)

	mt.mmap(t, "big", mmapPageSize)
	mt.mmap(t, "small", mmapSmallSize)

	mt.readAt(t, 0, mmapInitialLength)
	mt.readAt(t, 0, mmapPageSize)

	mt.writeAt(t, 0, mmapSmallSize)
	mt.readAt(t, 0, mmapSmallSize)

	mt.munmap(t, "small")
	mt.mmap(t, "small", mmapSmallSize)

	mt.writeAt(t, 0, mmapPageSize)
	mt.readAt(t, 0, mmapPageSize)

	mt.writeAt(t, mmapSmallSize-mmapEndOffset, mmapWriteLength)
	mt.readAt(t, mmapSmallSize-mmapReadLength, mmapReadLength)

	mt.writeAt(t, mmapPageSize-mmapReadLength, mmapWideLength)
	mt.readAt(t, mmapPageSize-mmapReadLength, mmapWideLength)

	mt.truncate(t, mmapNearEnd)
	mt.readAt(t, 0, mmapPageSize)

	mt.truncate(t, mmapPageSize)
	mt.readAt(t, mmapNearEnd, mmapTailLength)
	mt.writeAt(t, mmapNearEnd, mmapTailLength)

	mt.writeAt(t, mmapNearEnd, mmapExtendedWrite)
	mt.readAt(t, mmapNearEnd, mmapExtendedWrite)
}

// open dir
// rename dir
// delete open file
// stat dir?
// stat size?

// fix duplication of writes in tests?

// things to test:
// - error codes for...
//   closed, eof, wrong mode
// - concurrent reads/writes (same handle, different handle)
// - read/write at end of file (writeat with offset at file, after file end)
// - read ending at file (err = nil or eof or both?)
// - ???
