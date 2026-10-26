package fs

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	"github.com/neria-cloud/gosim/internal/kernel"
)

const exclusiveNB = lockExclusive | lockNonblock

func closeFile(t *testing.T, table *kernel.FDTable, fd int) *File {
	t.Helper()
	r, err := table.Remove(fd)
	must(t, err)
	must(t, r.Release(contracts.CloseExplicit))
	return r.(*File)
}

func busy(t *testing.T, i *Instance, ctx contracts.CallContext, fd int) {
	t.Helper()
	if err := i.Flock(ctx, fd, exclusiveNB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("contender acquired lock: %v", err)
	}
}

func TestFlockIndependentOpens(t *testing.T) {
	i, ctx, table := ioFixture(t)
	a := open(t, i, ctx, "lock", openCreate|2)
	b := open(t, i, ctx, "lock", 0)
	c := open(t, i, ctx, "lock", 0)
	must(t, i.Flock(ctx, a, exclusiveNB))
	must(t, i.Flock(ctx, a, exclusiveNB))
	busy(t, i, ctx, b)
	must(t, i.Flock(ctx, b, lockUnlock))
	closeFile(t, table, b)
	busy(t, i, ctx, c)
	must(t, i.Flock(ctx, a, lockUnlock|lockNonblock))
	must(t, i.Flock(ctx, c, exclusiveNB))
	old := closeFile(t, table, a)
	must(t, old.Release(contracts.CloseCrash))
	d := open(t, i, ctx, "lock", 0)
	busy(t, i, ctx, d)
	closeFile(t, table, c)
	must(t, i.Flock(ctx, d, exclusiveNB))
}

func TestFlockFlagsAndErrors(t *testing.T) {
	i, ctx, table := ioFixture(t)
	fd := open(t, i, ctx, "file", openCreate|2)
	for _, how := range []int{0, lockNonblock, 123, -1, lockExclusive | lockShared} {
		if err := i.Flock(ctx, fd, how); !errors.Is(err, syscall.EINVAL) {
			t.Fatal(how, err)
		}
	}
	for _, how := range []int{lockShared, lockShared | lockNonblock} {
		var failure *contracts.Failure
		if err := i.Flock(ctx, fd, how); !errors.As(err, &failure) || failure.Code != contracts.UnsupportedOperation {
			t.Fatal(how, err)
		}
	}
	i.machine.readOnly = true
	must(t, i.Flock(ctx, fd, exclusiveNB))
	closeFile(t, table, fd)
	if err := i.Flock(ctx, fd, exclusiveNB); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
	dir := open(t, i, ctx, ".", 0)
	must(t, i.Flock(ctx, dir, exclusiveNB))
	wrong := ctx
	wrong.Epoch.Machine.RunID++
	if err := i.Flock(wrong, dir, exclusiveNB); !errors.Is(err, contracts.ErrWrongRun) {
		t.Fatal(err)
	}
	wrong = ctx
	wrong.Epoch.Epoch++
	if err := i.Flock(wrong, dir, exclusiveNB); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal(err)
	}
}

func TestFlockBlockingUncontended(t *testing.T) {
	i, ctx, table := ioFixture(t)
	a := open(t, i, ctx, "lock", openCreate|2)
	b := open(t, i, ctx, "lock", 0)
	must(t, i.Flock(ctx, a, lockExclusive))
	must(t, i.Flock(ctx, a, lockExclusive))
	var failure *contracts.Failure
	if err := i.Flock(ctx, b, lockExclusive); !errors.As(err, &failure) || failure.Code != contracts.UnsupportedOperation {
		t.Fatal("contended blocking request did not fail explicitly", err)
	}
	busy(t, i, ctx, b)
	closeFile(t, table, a)
	must(t, i.Flock(ctx, b, lockExclusive))
}

func TestFlockRecordLocksUnsupported(t *testing.T) {
	i, ctx, _ := ioFixture(t)
	fd := open(t, i, ctx, "file", openCreate|2)
	before := contracts.Lock{Type: 1, Start: 17, Len: 32, PID: 9}
	for _, command := range []int{5, 6, 7} {
		lock := before
		var failure *contracts.Failure
		if err := i.FcntlLock(ctx, fd, command, &lock); !errors.As(err, &failure) || failure.Code != contracts.UnsupportedOperation {
			t.Fatal(command, err)
		}
		if lock != before {
			t.Fatal("rejected command mutated caller lock")
		}
	}
	for _, command := range []int{36, 37, 38, 999} {
		if err := i.FcntlLock(ctx, fd, command, &before); !errors.Is(err, syscall.EINVAL) {
			t.Fatal(command, err)
		}
	}
}

func TestFlockHostMappingLifetime(t *testing.T) {
	for _, policy := range []FlockSemantics{FlockLinux, FlockDarwin} {
		for _, unlock := range []bool{false, true} {
			t.Run(fmt.Sprintf("policy%d/unlock%v", policy, unlock), func(t *testing.T) {
				i, ctx, table := ioFixture(t)
				i.session.flockSemantics = policy
				a := open(t, i, ctx, "mapped", openCreate|2)
				b := open(t, i, ctx, "mapped", 0)
				_, err := i.Pwrite(ctx, a, bytesView("data"), 0)
				must(t, err)
				first, err := i.Map(ctx, 0, 4, protRead, mapShared, a, 0)
				must(t, err)
				must(t, i.Flock(ctx, a, exclusiveNB)) // mapping predates locking
				second, err := i.Map(ctx, 0, 4, protRead, mapShared, a, 0)
				must(t, err)
				if unlock {
					must(t, i.Flock(ctx, a, lockUnlock))
				}
				owner := closeFile(t, table, a)
				if policy == FlockLinux && !unlock {
					busy(t, i, ctx, b)
				} else {
					must(t, i.Flock(ctx, b, exclusiveNB))
				}
				must(t, i.Unmap(ctx, first, 4))
				if policy == FlockLinux && !unlock {
					busy(t, i, ctx, b)
				}
				must(t, i.Unmap(ctx, second, 4))
				must(t, i.Flock(ctx, b, exclusiveNB))
				if !owner.modelRefReleased || owner.mappingRefs != 0 {
					t.Fatal("open description leaked")
				}
				c := open(t, i, ctx, "mapped", 0)
				busy(t, i, ctx, c) // old owner cleanup cannot remove replacement owner
			})
		}
	}
}

func TestFlockOtherOpenMappingAndUnmapBeforeClose(t *testing.T) {
	i, ctx, table := ioFixture(t)
	a := open(t, i, ctx, "file", openCreate|2)
	b := open(t, i, ctx, "file", 0)
	_, err := i.Pwrite(ctx, a, bytesView("x"), 0)
	must(t, err)
	address, err := i.Map(ctx, 0, 1, protRead, mapShared, b, 0)
	must(t, err)
	must(t, i.Flock(ctx, a, exclusiveNB))
	closeFile(t, table, a)
	must(t, i.Flock(ctx, b, exclusiveNB))
	must(t, i.Unmap(ctx, address, 1))
	c := open(t, i, ctx, "file", 0)
	busy(t, i, ctx, c)
	closeFile(t, table, b)
	must(t, i.Flock(ctx, c, exclusiveNB))
}

func TestFlockReleaseReasonsAndLateRelease(t *testing.T) {
	for _, reason := range []contracts.CloseReason{contracts.CloseExplicit, contracts.CloseGraceful, contracts.CloseCrash, contracts.CloseAbort} {
		t.Run(fmt.Sprint(reason), func(t *testing.T) {
			i, ctx, table := ioFixture(t)
			a := open(t, i, ctx, "file", openCreate|2)
			b := open(t, i, ctx, "file", 0)
			must(t, i.Flock(ctx, a, exclusiveNB))
			owner, err := table.Remove(a)
			must(t, err)
			must(t, owner.Release(reason))
			must(t, owner.Release(reason))
			must(t, i.Flock(ctx, b, exclusiveNB))
			i.Detach()
			must(t, table.Finalize(contracts.CloseCrash))
			if i.flocks != nil || i.model != nil {
				t.Fatal("detached state retained")
			}
		})
	}
}

func TestFlockSessionCleanupContinuesAfterMappingPanic(t *testing.T) {
	i, ctx, table := ioFixture(t)
	fd := open(t, i, ctx, "file", openCreate|2)
	_, err := i.Pwrite(ctx, fd, bytesView("x"), 0)
	must(t, err)
	first, err := i.Map(ctx, 0, 1, protRead, mapShared, fd, 0)
	must(t, err)
	_, err = i.Map(ctx, 0, 1, protRead, mapShared, fd, 0)
	must(t, err)
	must(t, i.Flock(ctx, fd, exclusiveNB))
	owner := closeFile(t, table, fd)
	// Corrupt just one model mapping to exercise defensive invariant cleanup.
	i.mappings[first].model.Inode = -1
	other := start(t, i.session, contracts.MachineRef{RunID: 1, ID: 2}, 1, false)
	var recovered any
	func() { defer func() { recovered = recover() }(); i.session.Close() }()
	if recovered == nil {
		t.Fatal("cleanup invariant was hidden")
	}
	if !i.session.closed || i.session.machines != nil || i.session.handles != nil || i.model != nil || other.model != nil || i.flocks != nil || i.mappings != nil || owner.mappingRefs != 0 {
		t.Fatal("one failed unmap interrupted remaining cleanup")
	}
	i.session.Close()
}

func TestFlockProviderRequiresHostPolicy(t *testing.T) {
	_, ports := newSession(t)
	for _, policy := range []FlockSemantics{0, 99} {
		if _, err := NewProvider(policy).Open(ports.ports()); !errors.Is(err, contracts.ErrInvalidConfig) {
			t.Fatal(policy, err)
		}
	}
}
