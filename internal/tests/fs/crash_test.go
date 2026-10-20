//go:build sim

package behavior_test

import (
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	"github.com/neria-cloud/gosim"
	simfs "github.com/neria-cloud/gosim/fs"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func TestCrashRestartPendingDisk(t *testing.T) {
	var first atomic.Bool
	gosimruntime.TestRaceToken.Release()
	m := testsupport.Start(t, func() {
		gosimruntime.TestRaceToken.Acquire()
		a, err := os.OpenFile("a", os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Write([]byte("a")); err != nil {
			t.Fatal(err)
		}
		if err := a.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := a.Close(); err != nil {
			t.Fatal(err)
		}
		d, err := os.OpenFile(".", os.O_RDONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("b", []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}
		first.Store(true)
		gosim.CrashSelf()
	})
	testsupport.Wait(t, m)
	if !first.Load() {
		t.Error("expected first")
	}
	var second atomic.Bool
	gosimruntime.TestRaceToken.Release()
	testsupport.Check(t, m.SetMain(func() {
		gosimruntime.TestRaceToken.Acquire()
		contents, err := os.ReadFile("a")
		if err != nil {
			t.Fatal(err)
		}
		if string(contents) != "a" {
			t.Fatal("bad contents")
		}
		_, err = os.ReadFile("b")
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %v, got %v", os.ErrNotExist, err)
		}
		second.Store(true)
	}))
	testsupport.Check(t, m.Restart())
	testsupport.Wait(t, m)
	if !second.Load() {
		t.Error("expected second")
	}
}

func TestCrashRestartFilesystemPartialInner(t *testing.T) {
	m := testsupport.Start(t, func() {
		testsupport.Check(t, simfs.SetCrashMode(gosim.CurrentMachine(), simfs.SamplePending))
		if err := os.WriteFile("foo", []byte("hello"), 0o644); err != nil {
			// XXX: could this be log.Fatal?
			t.Fatal(err)
		}
		gosim.CrashSelf()
	})
	testsupport.Wait(t, m)
	// now see what we wrote to disk. see details below on scenarios.
	testsupport.Check(t, m.SetMain(func() {
		contents, err := os.ReadFile("foo")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				contents = []byte("<missing>")
			} else {
				t.Fatal(err)
			}
		}
		slog.Info("read", "contents", string(contents))
	}))
	testsupport.Check(t, m.Restart())
	testsupport.Wait(t, m)
}
