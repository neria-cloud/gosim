package kernel

import (
	"errors"
	"math"
	"strings"
	"syscall"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

func pollFixture(t *testing.T, k *Kernel) (contracts.CallContext, *dataResource, uint64) {
	t.Helper()
	ref := createFor(t, k, "poll", idleMain)
	if err := k.Start(ref); err != nil {
		t.Fatal(err)
	}
	run := k.machineByID[ref.ID].current
	ctx := contracts.CallContext{Epoch: run.ref}
	r := &dataResource{owner: contracts.RT}
	fd, err := run.descriptors.Allocate(r)
	if err != nil {
		t.Fatal(err)
	}
	r.fd = fd
	id, err := k.PollOpen(ctx, fd)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, r, id
}

func dispatchPollWait(k *Kernel, ctx contracts.CallContext, id uint64, mode uint8) *Invocation {
	inv := &Invocation{Context: ctx, Route: RoutePollWait, Scalars: [6]uintptr{uintptr(id), uintptr(mode)}}
	k.dispatcher.DispatchOne(inv)
	return inv
}

func TestPollReadinessResetUnblockAndRegistrationGeneration(t *testing.T) {
	_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ctx, r, id := pollFixture(t, k)
		if _, err := k.PollOpen(ctx, 0); !errors.Is(err, ErrNotPollable) {
			t.Fatal(err)
		}
		if _, err := k.PollOpen(ctx, 999); !errors.Is(err, syscall.EBADF) {
			t.Fatal(err)
		}
		oldSink := k.polls[id].sink
		inv := dispatchPollWait(k, ctx, id, 'r')
		if len(k.dispatcher.live) != 1 || inv.Sema != 0 {
			t.Fatal("poll did not retain pending invocation")
		}
		r.notify(contracts.ReadReady)
		if len(k.dispatcher.live) != 0 || inv.Sema != 1 || inv.Result.R0 != 0 {
			t.Fatal("readiness did not complete wait")
		}
		if err := k.PollReset(ctx, id, 'r'); err != nil {
			t.Fatal(err)
		}
		inv = dispatchPollWait(k, ctx, id, 'r')
		if err := k.PollUnblock(ctx, id); err != nil {
			t.Fatal(err)
		}
		if inv.Result.R0 != 1 || inv.Sema != 1 {
			t.Fatal("unblock did not wake with closing")
		}
		if err := k.PollReset(ctx, id, 'r'); !errors.Is(err, ErrPollClosing) {
			t.Fatal(err)
		}
		if err := k.PollClose(ctx, id); err != nil {
			t.Fatal(err)
		}
		if len(r.sinks) != 0 {
			t.Fatal("poll close retained subscription")
		}
		r.ready = 0
		newID, err := k.PollOpen(ctx, r.fd)
		if err != nil || newID <= id {
			t.Fatal("poll handle reused", newID, err)
		}
		oldSink.Notify(ctx.Epoch, r.fd, contracts.ReadReady)
		if k.polls[newID].ready != 0 {
			t.Fatal("old sink reached new registration")
		}
		if err := k.CloseFD(ctx, r.fd); err != nil {
			t.Fatal(err)
		}
		if k.polls[newID] != nil || len(r.sinks) != 0 || r.released != 1 {
			t.Fatal("fd close did not finalize polls")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPollDeadlineGenerationAndCanceledEpoch(t *testing.T) {
	_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ctx, r, id := pollFixture(t, k)
		inv := dispatchPollWait(k, ctx, id, 'r')
		if err := k.PollDeadline(ctx, id, 100, 'r'); err != nil {
			t.Fatal(err)
		}
		oldGeneration := k.polls[id].read.generation
		if err := k.PollDeadline(ctx, id, 200, 'r'); err != nil {
			t.Fatal(err)
		}
		k.expirePoll(id, oldGeneration, 'r')
		if inv.Sema != 0 {
			t.Fatal("old deadline completed current wait")
		}
		simrt.Sleep(150)
		if inv.Sema != 0 {
			t.Fatal("canceled deadline fired")
		}
		simrt.Sleep(60)
		if inv.Sema != 1 || inv.Result.R0 != 2 {
			t.Fatal("replacement deadline did not wake")
		}
		if err := k.PollReset(ctx, id, 'r'); !errors.Is(err, ErrPollTimeout) {
			t.Fatal(err)
		}
		if err := k.PollDeadline(ctx, id, 0, 'r'); err != nil {
			t.Fatal(err)
		}
		if err := k.PollReset(ctx, id, 'r'); err != nil {
			t.Fatal(err)
		}
		oldSink := k.polls[id].sink
		pending := dispatchPollWait(k, ctx, id, 'w')
		if err := k.PollDeadline(ctx, id, 100, 'w'); err != nil {
			t.Fatal(err)
		}
		if err := k.Stop(ctx.Epoch.Machine, contracts.StopCrash); err != nil {
			t.Fatal(err)
		}
		if pending.Sema != 0 || pending.Result != (Result{}) || len(k.dispatcher.live) != 0 || len(k.dispatcher.canceled) != 0 || len(k.polls) != 0 {
			t.Fatal("canceled poll retained lease or woke caller")
		}
		if err := k.Restart(ctx.Epoch.Machine); err != nil {
			t.Fatal(err)
		}
		run := k.machineByID[ctx.Epoch.Machine.ID].current
		newCtx := contracts.CallContext{Epoch: run.ref}
		newResource := &dataResource{owner: contracts.RT}
		fd, err := run.descriptors.Allocate(newResource)
		if err != nil {
			t.Fatal(err)
		}
		newResource.fd = fd
		if fd != r.fd {
			t.Fatal("fixture did not reuse epoch-local fd")
		}
		newID, err := k.PollOpen(newCtx, fd)
		if err != nil {
			t.Fatal(err)
		}
		oldSink.Notify(ctx.Epoch, fd, contracts.ReadReady)
		k.expirePoll(id, oldGeneration, 'r')
		simrt.Sleep(200)
		if k.polls[newID].ready != 0 || k.polls[newID].read.timedOut {
			t.Fatal("old epoch affected restarted poll")
		}
		if err := k.PollDeadline(newCtx, newID, -1, 'r'+'w'); err != nil {
			t.Fatal(err)
		}
		if err := k.PollWait(newCtx, newID, 'r'); !errors.Is(err, ErrPollTimeout) {
			t.Fatal(err)
		}
		if err := k.PollWait(newCtx, newID, 'w'); !errors.Is(err, ErrPollTimeout) {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPollSubscriptionFailureAndIDExhaustion(t *testing.T) {
	_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
		ctx, r, id := pollFixture(t, k)
		if err := k.PollClose(ctx, id); err != nil {
			t.Fatal(err)
		}
		r.subscribeErr = errInjected
		if _, err := k.PollOpen(ctx, r.fd); !errors.Is(err, errInjected) {
			t.Fatal(err)
		}
		if len(r.sinks) != 0 || len(k.polls) != 0 {
			t.Fatal("failed subscription retained owner state")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"registration", "deadline"} {
		t.Run(failure, func(t *testing.T) {
			_, err := fixture(t, contracts.FeatureSet(contracts.RT), nil, func(k *Kernel) {
				ctx, r, id := pollFixture(t, k)
				if failure == "registration" {
					k.nextPollID = math.MaxUint64
					if last, err := k.PollOpen(ctx, r.fd); err != nil || last != math.MaxUint64 {
						t.Fatal(last, err)
					}
					_, _ = k.PollOpen(ctx, r.fd)
				} else {
					k.polls[id].read.generation = math.MaxUint64
					_ = k.PollDeadline(ctx, id, 1, 'r')
				}
				t.Error("poll ID overflow returned")
			})
			if err == nil || !strings.Contains(err.Error(), "exhausted") {
				t.Fatal(err)
			}
		})
	}
}
