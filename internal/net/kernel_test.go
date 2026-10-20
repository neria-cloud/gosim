package net

import (
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
	fsprovider "github.com/neria-cloud/gosim/internal/fs"
	"github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type captureProvider struct {
	Provider
	session *Session
}

func (p *captureProvider) Open(ports contracts.Ports) (contracts.ProviderSession, error) {
	s, err := p.Provider.Open(ports)
	if err == nil {
		p.session = s.(*Session)
	}
	return s, err
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func withKernel(t *testing.T, extra []contracts.Provider, body func(*kernel.Kernel, *Session, *simrt.Engine)) {
	t.Helper()
	e, err := simrt.NewEngine(simrt.EngineConfig{Seed: 42, TimeoutNS: int64(10 * time.Second), ReplayRevision: simrt.ReplayRevision})
	must(t, err)
	defer e.Close()
	must(t, e.Run(func() {
		provider := &captureProvider{}
		features := contracts.FeatureSet(contracts.RT | contracts.Net)
		if len(extra) > 0 {
			features |= contracts.FeatureSet(contracts.FS)
		}
		k, err := kernel.New(kernel.Config{Features: features, TimeoutNS: int64(10 * time.Second), BuildDigest: strings.Repeat("a", 64)}, e, append(extra, provider))
		must(t, err)
		defer func() {
			must(t, k.Close())
			if !provider.session.closed || provider.session.network != nil || len(provider.session.machines) != 0 {
				t.Fatal("session retained state")
			}
		}()
		body(k, provider.session, e)
	}))
}

func create(t *testing.T, k *kernel.Kernel) contracts.MachineRef {
	t.Helper()
	ref, err := k.Create("net-test", func() { simrt.NewChan[int](0).Recv() })
	must(t, err)
	return ref
}

func netControl(t *testing.T, k *kernel.Kernel, ref contracts.MachineRef) contracts.NetControl {
	t.Helper()
	c, err := k.NetworkControl(contracts.CallContext{}, ref.RunID)
	must(t, err)
	return c
}
func contextOf(i *Instance) contracts.CallContext { return contracts.CallContext{Epoch: i.ref} }
func start(t *testing.T, k *kernel.Kernel, s *Session) *Instance {
	ref := create(t, k)
	must(t, k.Start(ref))
	return s.machines[ref.ID].active
}

func newSocket(t *testing.T, i *Instance) int {
	t.Helper()
	fd, err := i.Socket(contextOf(i), addressIPv4, socketFlags, 0)
	must(t, err)
	return fd
}

func listen(t *testing.T, i *Instance, port uint16) int {
	t.Helper()
	fd := newSocket(t, i)
	must(t, i.Bind(contextOf(i), fd, contracts.Address{Family: 2, Port: port}))
	must(t, i.Listen(contextOf(i), fd, 128))
	return fd
}

func zeroDelay(t *testing.T, s *Session) {
	must(t, s.Configure(contracts.NetConfig{Lambda: 5, Repair: 1}))
}

func establish(t *testing.T, a, b *Instance, listener int) (int, int) {
	t.Helper()
	fd := newSocket(t, a)
	local, err := b.LocalAddress(contextOf(b), listener)
	must(t, err)
	local.IPv4 = b.machine.address
	if err := a.Connect(contextOf(a), fd, local); err != syscall.EINPROGRESS {
		t.Fatal(err)
	}
	simrt.Sleep(1)
	accepted, peer, err := b.Accept(contextOf(b), listener, socketNonblock|socketCloseExec)
	must(t, err)
	if peer.IPv4 != a.machine.address {
		t.Fatal("wrong peer")
	}
	return fd, accepted
}

func TestNetworkControlsLifecycleAndReservations(t *testing.T) {
	var stale contracts.NetControl
	var old contracts.MachineRef
	withKernel(t, nil, func(k *kernel.Kernel, s *Session, _ *simrt.Engine) {
		ref := create(t, k)
		old = ref
		c := netControl(t, k, ref)
		stale = c
		if _, err := c.Address(ref); !errors.Is(err, contracts.ErrNotStarted) {
			t.Fatal(err)
		}
		for _, config := range []contracts.NetConfig{{}, {MinNS: -1, Lambda: 1}, {Lambda: math.NaN()}, {Lambda: 1, Fail: math.Inf(1)}, {Lambda: 1, Repair: -1}, {Lambda: 1, TCPBufferSize: -1}, {Lambda: 1, TCPBufferSize: 16*1024*1024 + 1}, {Lambda: 1, ConnectionTimeoutNS: -1}} {
			if err := c.Configure(config); !errors.Is(err, contracts.ErrInvalidConfig) {
				t.Fatal("invalid config", err)
			}
		}
		explicit := [4]byte{10, 0, 0, 1}
		must(t, c.ConfigureMachine(ref, explicit, false))
		if err := c.ConfigureMachine(ref, [4]byte{}, false); !errors.Is(err, contracts.ErrInvalidConfig) {
			t.Fatal(err)
		}
		tx, err := k.BeginStart(ref, false)
		must(t, err)
		if _, err := c.Address(ref); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("address during prepare", err)
		}
		if err := c.ConfigureMachine(ref, [4]byte{}, true); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal(err)
		}
		if _, err := s.machines[ref.ID].prepared.Bindings().Sockets.Socket(contracts.CallContext{Epoch: contracts.EpochRef{Machine: ref, Epoch: 1}}, 2, socketFlags, 0); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("staged service callable", err)
		}
		tx.Abort()
		if s.machines[ref.ID].prepared != nil || s.machines[ref.ID].started {
			t.Fatal("abort published")
		}
		must(t, k.Start(ref))
		addr, err := c.Address(ref)
		must(t, err)
		if addr != explicit {
			t.Fatal(addr)
		}
		instance := s.machines[ref.ID].active
		epochControl, err := k.NetworkControl(contextOf(instance), ref.RunID)
		must(t, err)
		other := create(t, k)
		must(t, c.ConfigureMachine(other, explicit, false))
		if err := k.Start(other); err != syscall.EADDRINUSE {
			t.Fatal("collision", err)
		}
		must(t, k.Stop(ref, contracts.StopCrash))
		if err := epochControl.Configure(contracts.NetConfig{Lambda: 1}); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("stale epoch controls", err)
		}
		if err := c.ConfigureMachine(ref, [4]byte{}, true); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("changed restart identity", err)
		}
		if err := k.Start(other); err != syscall.EADDRINUSE {
			t.Fatal("stopped machine lost address", err)
		}
		must(t, k.Restart(ref))
		addr, err = c.Address(ref)
		must(t, err)
		if addr != explicit || s.machines[ref.ID].active.ref.Epoch <= instance.ref.Epoch {
			t.Fatal("restart identity")
		}
		must(t, k.Stop(ref, contracts.StopGraceful))
		must(t, k.Destroy(ref))
		must(t, k.Start(other))
		if _, err := c.Address(ref); !errors.Is(err, contracts.ErrMachineNotFound) {
			t.Fatal("destroyed address", err)
		}
		if _, err := instance.Socket(contextOf(instance), 2, socketFlags, 0); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("stale instance", err)
		}
	})
	if err := stale.Configure(contracts.NetConfig{Lambda: 1}); !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatal("closed controls", err)
	}
	withKernel(t, nil, func(k *kernel.Kernel, s *Session, _ *simrt.Engine) {
		ref := create(t, k)
		c := netControl(t, k, ref)
		if _, err := c.Address(old); !errors.Is(err, contracts.ErrWrongRun) {
			t.Fatal("wrong run", err)
		}
		if _, err := k.NetworkControl(contracts.CallContext{}, old.RunID); !errors.Is(err, contracts.ErrWrongRun) {
			t.Fatal(err)
		}
		if err := s.ConfigureMachine(ref, [4]byte{224, 0, 0, 1}, false); !errors.Is(err, contracts.ErrInvalidConfig) {
			t.Fatal("multicast", err)
		}
	})
}

func TestSocketServicesOwnershipAndUnsupportedBoundaries(t *testing.T) {
	withKernel(t, nil, func(k *kernel.Kernel, s *Session, _ *simrt.Engine) {
		zeroDelay(t, s)
		a, b := start(t, k, s), start(t, k, s)
		actx, bctx := contextOf(a), contextOf(b)
		for _, args := range [][3]int{{10, socketFlags, 0}, {2, 2 | socketNonblock | socketCloseExec, 0}, {2, 1, 0}, {2, socketFlags, 17}} {
			if _, err := a.Socket(actx, args[0], args[1], args[2]); err == nil {
				t.Fatal("unsupported socket accepted", args)
			}
		}
		listener := listen(t, b, 8080)
		fd, accepted := establish(t, a, b, listener)
		if _, _, err := b.Accept(bctx, listener, socketNonblock|socketCloseExec); err != syscall.EWOULDBLOCK {
			t.Fatal("empty accept", err)
		}
		local, err := b.LocalAddress(bctx, listener)
		must(t, err)
		if local.Port != 8080 {
			t.Fatal("dummy listen address")
		}
		peer, err := a.PeerAddress(actx, fd)
		must(t, err)
		if peer.Port != 8080 || peer.IPv4 != b.machine.address {
			t.Fatal(peer)
		}
		client, err := a.socket(actx, fd)
		must(t, err)
		server, err := b.socket(bctx, accepted)
		must(t, err)
		if _, err := client.Read(bctx, abi.ByteView{}); !errors.Is(err, contracts.ErrInvalidState) {
			t.Fatal("foreign epoch read", err)
		}
		n, err := client.Write(actx, abi.ByteView{Ptr: []byte("hello")})
		must(t, err)
		if n != 5 {
			t.Fatal(n)
		}
		simrt.Sleep(1)
		output := make([]byte, 5)
		n, err = server.Read(bctx, abi.ByteView{Ptr: output})
		must(t, err)
		if n != 5 || string(output) != "hello" {
			t.Fatal(n, string(output))
		}
		status := make([]byte, 4)
		n, err = a.GetOption(actx, fd, 1, 4, abi.ByteView{Ptr: status})
		must(t, err)
		if n != 4 || binary.LittleEndian.Uint32(status) != 0 {
			t.Fatal("connect status")
		}
		for _, option := range [][3]int{{1, 2, 4}, {1, 7, 4}, {1, 8, 4}, {1, 9, 4}, {1, 13, 8}, {6, 1, 4}, {6, 4, 4}, {6, 5, 4}, {6, 6, 4}} {
			must(t, a.SetOption(actx, fd, option[0], option[1], abi.ByteView{Ptr: make([]byte, option[2])}))
		}
		if err := a.SetOption(actx, fd, 1, 999, abi.ByteView{Ptr: status}); err != syscall.ENOPROTOOPT {
			t.Fatal(err)
		}
		if err := a.SetOption(actx, fd, 6, 1, abi.ByteView{Ptr: status[:3]}); err != syscall.EINVAL {
			t.Fatal(err)
		}
		if _, err := a.GetOption(actx, fd, 1, 999, abi.ByteView{Ptr: status}); err != syscall.ENOPROTOOPT {
			t.Fatal(err)
		}
		if _, err := a.FcntlInt(actx, fd, 3, 0); err != syscall.ENOSYS {
			t.Fatal(err)
		}
		if err := a.Connect(actx, fd, peer); err != syscall.EISCONN {
			t.Fatal("reconnect leaked stream", err)
		}
		if err := a.Bind(actx, newSocket(t, a), contracts.Address{Family: 10, Port: 1}); err != syscall.EAFNOSUPPORT {
			t.Fatal("IPv6 truncated", err)
		}
		if err := a.Bind(actx, newSocket(t, a), contracts.Address{Family: 2, Port: 1, IPv4: b.machine.address}); err != syscall.EADDRNOTAVAIL {
			t.Fatal(err)
		}
		must(t, k.CloseFD(actx, fd))
		must(t, client.Release(contracts.CloseCrash))
		simrt.Sleep(1)
		if client.closeReason != contracts.CloseExplicit || !client.released {
			t.Fatal("release repeated")
		}
		if _, err := server.Read(bctx, abi.ByteView{Ptr: output}); err != syscall.EPIPE {
			t.Fatal("peer close", err)
		}
		if _, err := a.socket(actx, fd); err != syscall.EBADF {
			t.Fatal("closed fd reused", err)
		}
	})
}

func TestReadinessLifecycleAndPeerStopModes(t *testing.T) {
	for _, kind := range []contracts.StopKind{contracts.StopGraceful, contracts.StopCrash} {
		withKernel(t, nil, func(k *kernel.Kernel, s *Session, _ *simrt.Engine) {
			zeroDelay(t, s)
			a, b := start(t, k, s), start(t, k, s)
			actx, bctx := contextOf(a), contextOf(b)
			listener := listen(t, b, 80)
			fd, accepted := establish(t, a, b, listener)
			client, err := a.socket(actx, fd)
			must(t, err)
			server, err := b.socket(bctx, accepted)
			must(t, err)
			id, err := k.PollOpen(actx, fd)
			must(t, err)
			must(t, k.PollDeadline(actx, id, 100, 'r'))
			must(t, k.PollDeadline(actx, id, 200, 'r'))
			simrt.Sleep(150)
			if err := k.PollWait(actx, id, 'r'); errors.Is(err, kernel.ErrPollTimeout) {
				t.Fatal("old deadline fired")
			}
			simrt.Sleep(60)
			if err := k.PollWait(actx, id, 'r'); !errors.Is(err, kernel.ErrPollTimeout) {
				t.Fatal(err)
			}
			must(t, k.PollDeadline(actx, id, 0, 'r'))
			old := client.subscriptions[0]
			must(t, k.PollClose(actx, id))
			newID, err := k.PollOpen(actx, fd)
			must(t, err)
			old.Notify(a.ref, fd, contracts.ReadReady)
			if err := k.PollWait(actx, newID, 'r'); err == nil {
				t.Fatal("stale poll callback admitted")
			}
			var staleServer *socketReadiness
			serverPoll, err := k.PollOpen(bctx, accepted)
			must(t, err)
			_ = serverPoll
			staleServer = server.subscriptions[0]
			oldRef := b.ref
			must(t, k.Stop(b.machine.ref, kind))
			staleServer.Notify(oldRef, accepted, contracts.ReadReady)
			if !server.released || server.closeReason != map[contracts.StopKind]contracts.CloseReason{contracts.StopGraceful: contracts.CloseGraceful, contracts.StopCrash: contracts.CloseCrash}[kind] || len(server.subscriptions) != 0 {
				t.Fatal("stop did not release owned socket")
			}
			simrt.Sleep(1)
			if a.stack.StreamStatus(client.stream).Closed != (kind == contracts.StopGraceful) {
				t.Fatal("wrong peer stop behavior")
			}
			must(t, k.Restart(b.machine.ref))
			fresh := s.machines[b.machine.ref.ID].active
			if fresh.ref.Epoch == oldRef.Epoch || fresh.machine.address != b.machine.address {
				t.Fatal("bad restart")
			}
			if _, err := b.Socket(bctx, 2, socketFlags, 0); !errors.Is(err, contracts.ErrInvalidState) {
				t.Fatal(err)
			}
		})
	}
}

type rejectingTable struct{ contracts.DescriptorTable }

func (rejectingTable) Allocate(contracts.Resource) (int, error) { return 0, syscall.EMFILE }
func TestFailedDescriptorTransferAndControlCopies(t *testing.T) {
	withKernel(t, nil, func(k *kernel.Kernel, s *Session, _ *simrt.Engine) {
		zeroDelay(t, s)
		a, b := start(t, k, s), start(t, k, s)
		listener := listen(t, b, 80)
		original := a.fds
		a.fds = rejectingTable{original}
		if _, err := a.Socket(contextOf(a), 2, socketFlags, 0); err != syscall.EMFILE || len(a.sockets) != 0 {
			t.Fatal("failed allocation leaked", err)
		}
		a.fds = original
		fd := newSocket(t, a)
		if err := a.Connect(contextOf(a), fd, contracts.Address{Family: 2, IPv4: b.machine.address, Port: 80}); err != syscall.EINPROGRESS {
			t.Fatal(err)
		}
		simrt.Sleep(1)
		original = b.fds
		b.fds = rejectingTable{original}
		if _, _, err := b.Accept(contextOf(b), listener, socketNonblock|socketCloseExec); err != syscall.EMFILE || len(b.sockets) != 1 {
			t.Fatal("failed accept leaked", err)
		}
		b.fds = original
		// Abort is silent; failed fd transfer must not enqueue an explicit close.
		client, err := a.socket(contextOf(a), fd)
		must(t, err)
		simrt.Sleep(1)
		if a.stack.StreamStatus(client.stream).Closed {
			t.Fatal("abort sent close")
		}
		c := netControl(t, k, a.machine.ref)
		link := contracts.NetLink{From: a.machine.address, To: b.machine.address, Direction: 1}
		policy := contracts.NetLinkPolicy{SetLatency: true, MinNS: 10, MaxNS: 10, Lambda: 2, SetLoss: true, Repair: 1}
		must(t, c.SetLinkPolicy(link, policy))
		policy.Lambda = math.NaN()
		if err := c.SetLinkPolicy(link, policy); !errors.Is(err, contracts.ErrInvalidConfig) {
			t.Fatal(err)
		}
		must(t, c.SetConnectivity(link, false))
		must(t, c.SetConnectivity(link, true))
		must(t, c.ResetLinkPolicy(link))
		if err := c.SetLinkPolicy(contracts.NetLink{Direction: 2}, contracts.NetLinkPolicy{}); !errors.Is(err, contracts.ErrInvalidConfig) {
			t.Fatal(err)
		}
	})
}

// The real filesystem provider participates before the real network provider.
// Address collision is a genuine second-provider preparation failure.
func TestRealNetworkFailureRollsBackFilesystem(t *testing.T) {
	withKernel(t, []contracts.Provider{fsprovider.NewProvider()}, func(k *kernel.Kernel, s *Session, e *simrt.Engine) {
		a := start(t, k, s)
		ref := create(t, k)
		c := netControl(t, k, ref)
		must(t, c.ConfigureMachine(ref, a.machine.address, false))
		fsControl, err := k.FilesystemControl(contracts.CallContext{}, ref.RunID)
		must(t, err)
		must(t, fsControl.ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/etc"}))
		if err := k.Start(ref); err != syscall.EADDRINUSE {
			t.Fatal(err)
		}
		if s.machines[ref.ID].prepared != nil || s.machines[ref.ID].active != nil {
			t.Fatal("network preparation leaked")
		}
		// A failed start did not publish the filesystem's before-first-start config.
		must(t, fsControl.ConfigureMachine(ref, contracts.FSConfig{WorkingDir: "/app"}))
		must(t, c.ConfigureMachine(ref, [4]byte{}, true))
		must(t, k.Start(ref))
		instance := s.machines[ref.ID].active
		files, err := k.Filesystem(contextOf(instance))
		must(t, err)
		info, err := files.Statat(contextOf(instance), -100, ".", 0)
		must(t, err)
		if info.Mode != 0o040777 {
			t.Fatal("filesystem binding missing")
		}
		expected := simrt.NewDomainRandom(42, "fs", ref.ID).Uint64()
		if e.RandomFor("fs", ref.ID).Uint64() != expected {
			t.Fatal("network failure consumed filesystem stream")
		}
	})
}

func TestSocketPendingAndFailedConnectStatus(t *testing.T) {
	withKernel(t, nil, func(k *kernel.Kernel, s *Session, _ *simrt.Engine) {
		zeroDelay(t, s)
		a := start(t, k, s)
		ctx := contextOf(a)
		fd := newSocket(t, a)
		target := contracts.Address{Family: 2, IPv4: [4]byte{10, 99, 0, 1}, Port: 80}
		if err := a.Connect(ctx, fd, target); err != syscall.EINPROGRESS {
			t.Fatal(err)
		}
		if err := a.Connect(ctx, fd, target); err != syscall.EALREADY {
			t.Fatal(err)
		}
		status := make([]byte, 4)
		_, err := a.GetOption(ctx, fd, 1, 4, abi.ByteView{Ptr: status})
		must(t, err)
		if binary.LittleEndian.Uint32(status) != 115 {
			t.Fatal("pending status is not Linux EINPROGRESS")
		}
		simrt.Sleep(int64(time.Second) + 1)
		_, err = a.GetOption(ctx, fd, 1, 4, abi.ByteView{Ptr: status})
		must(t, err)
		if binary.LittleEndian.Uint32(status) != 110 {
			t.Fatal("failed status is not Linux ETIMEDOUT")
		}
	})
}

func TestFailedPreparationDoesNotConsumeRandomStreams(t *testing.T) {
	run := func(fail bool) (draws [3]uint64) {
		withKernel(t, []contracts.Provider{fsprovider.NewProvider()}, func(k *kernel.Kernel, s *Session, e *simrt.Engine) {
			a := start(t, k, s)
			ref := create(t, k)
			c := netControl(t, k, ref)
			must(t, c.ConfigureMachine(ref, a.machine.address, false))
			if fail {
				if err := k.Start(ref); err != syscall.EADDRINUSE {
					t.Fatal(err)
				}
			}
			// Use the native stream API, never inspect its scheduler-owned memory.
			draws = [3]uint64{e.RandomFor("fs", ref.ID).Uint64(), e.RandomFor("net", 0).Uint64(), e.RandomFor("scheduler", 0).Uint64()}
		})
		return
	}
	if run(false) != run(true) {
		t.Fatal("failed preparation changed filesystem, network, or scheduler randomness")
	}
}

func TestRestartReusesEndpointWithoutReusingConnection(t *testing.T) {
	withKernel(t, nil, func(k *kernel.Kernel, s *Session, _ *simrt.Engine) {
		zeroDelay(t, s)
		a, b := start(t, k, s), start(t, k, s)
		listener := listen(t, b, 80)
		fd, accepted := establish(t, a, b, listener)
		client, err := a.socket(contextOf(a), fd)
		must(t, err)
		oldID := client.stream.ID()
		oldServer, err := b.socket(contextOf(b), accepted)
		must(t, err)
		ref := a.machine.ref
		must(t, k.Stop(ref, contracts.StopCrash))
		must(t, k.Restart(ref))
		fresh := s.machines[ref.ID].active
		newFD, newAccepted := establish(t, fresh, b, listener)
		newClient, err := fresh.socket(contextOf(fresh), newFD)
		must(t, err)
		if newClient.stream.ID() != oldID {
			t.Fatal("fixture did not recycle endpoint tuple")
		}
		if _, err := oldServer.Write(contextOf(b), abi.ByteView{Ptr: []byte("stale")}); err != syscall.EPIPE {
			t.Fatal("old connection remained writable", err)
		}
		must(t, k.CloseFD(contextOf(b), accepted)) // Must not remove the replacement stream.
		_, err = newClient.Write(contextOf(fresh), abi.ByteView{Ptr: []byte("fresh")})
		must(t, err)
		simrt.Sleep(1)
		newServer, err := b.socket(contextOf(b), newAccepted)
		must(t, err)
		buffer := make([]byte, 5)
		n, err := newServer.Read(contextOf(b), abi.ByteView{Ptr: buffer})
		must(t, err)
		if n != 5 || string(buffer) != "fresh" {
			t.Fatal("recycled connection received stale data")
		}
	})
}
