package simulation

import (
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"github.com/neria-cloud/gosim/internal/contracts"
	fs "github.com/neria-cloud/gosim/internal/fs/model"
	network "github.com/neria-cloud/gosim/internal/net/model"
	"github.com/neria-cloud/gosim/internal/simulation/syscallabi"
	"github.com/neria-cloud/gosim/rt/runtime"
)

type Machine struct {
	id          int
	netEpoch    contracts.EpochRef
	label       string
	addr        netip.Addr
	bootProgram func()

	// TODO: decide how this is shared with LinuxOS exactly and how to lock it
	filesystem *fs.Filesystem
	// TODO(migration): provider-owned stream replaces this field in Task 22.
	filesystemRandom *gosimruntime.Random
	netstack         *network.Stack

	gosimOS *GosimOS
	linuxOS *LinuxOS

	runtimeMachine *gosimruntime.Machine

	waiters []*syscallabi.Syscall

	sometimesCrashOnSyncMu sync.Mutex
	sometimesCrashOnSync   bool
}

type Simulation struct {
	dispatcher syscallabi.Dispatcher

	mu sync.Mutex

	nextMachineID int
	machinesById  map[int]*Machine
	main          *Machine

	network       *network.Network
	networkPorts  contracts.Ports
	networkEvents *legacyNetworkEvents
	networkRunID  uint64

	timeoutTimer *time.Timer

	// rawLogger does not include goroutine or machine
	rawLogger *slog.Logger
}

func (s *Simulation) newMachine(label string, addr netip.Addr, filesystem *fs.Filesystem, bootProgram func()) *Machine {
	id := s.nextMachineID
	s.nextMachineID++

	if label == "" {
		label = fmt.Sprintf("machine-%d", id)
	}

	m := &Machine{
		id:          id,
		label:       label,
		addr:        addr,
		bootProgram: bootProgram,

		filesystem:       filesystem,
		filesystemRandom: gosimruntime.NewDomainRandom(gosimruntime.Seed(), "fs", uint64(id)),
		netstack:         nil,

		runtimeMachine: nil,
	}

	s.machinesById[id] = m

	return m
}

func (s *Simulation) startMachine(machine *Machine) {
	machine.runtimeMachine = gosimruntime.NewMachine(machine.label)

	machine.netEpoch.Machine = contracts.MachineRef{RunID: s.networkRunID, ID: uint64(machine.id)}
	machine.netEpoch.Epoch++
	var raw [4]byte
	if machine.addr.IsValid() {
		raw = machine.addr.As4()
	}
	reservation, err := s.network.ReserveAddress(machine.netEpoch.Machine, raw, machine.addr.IsValid())
	if err != nil {
		panic(err)
	}
	machine.addr = reservation.Address()
	machine.netstack = network.NewStack(machine.netEpoch, s.networkPorts)
	s.networkEvents.live[machine.netEpoch] = true
	s.network.Attach(reservation, machine.netEpoch, machine.netstack)

	machine.linuxOS = NewLinuxOS(s, machine, s.dispatcher)
	machine.gosimOS = NewGosimOS(s, s.dispatcher)

	linuxOS := machine.linuxOS
	gosimOS := machine.gosimOS
	bootProgram := machine.bootProgram

	gosimruntime.GoWithMachine(func() {
		setupUserspace(gosimOS, linuxOS, machine.id, machine.label)
		bootProgram()
		// not using defer because we want an unhandled panic to bubble up; Stop
		// prevents all code from running afterwards
		SyscallMachineStop(machine.id, true)
	}, machine.runtimeMachine)
}

func (s *Simulation) stopMachine(machine *Machine, graceful bool) {
	if machine.runtimeMachine == nil {
		// XXX: already stopped
		return
	}

	// first, stop all the work the machine is currently doing. this atomically
	// stops all goroutines, unhooks all selects, and prevents any future timers
	// from firing.
	gosimruntime.StopMachine(machine.runtimeMachine)

	// then, wait for current syscalls to finish and prevent all work on any
	// pending future syscalls.
	machine.linuxOS.doShutdown()
	machine.linuxOS = nil

	for _, close := range machine.gosimOS.crashIterCloseByID {
		close()
	}
	machine.gosimOS = nil

	// make sure the network stack won't do anything in the future
	// graceful shutdown also sends close packets
	kind := contracts.StopCrash
	if graceful {
		kind = contracts.StopGraceful
	}
	machine.netstack.Quiesce()
	machine.netstack.Shutdown(kind)

	// stop sending packets to this netstack
	s.network.Detach(machine.netEpoch)
	s.networkEvents.CancelEpoch(machine.netEpoch)
	delete(s.networkEvents.live, machine.netEpoch)
	machine.netstack = nil

	if graceful {
		// XXX: flush all writes to disk
		machine.filesystem.FlushEverything()
	}

	// XXX: what about polldescs? any other comms between machine and the world?

	// XXX: shut down any background flusher in the filesystem (tbd, does not exist yet)
	// XXX: keep a snapshot of the filesystem?
	// XXX: mark machine as bad? don't want to run code in it again.
	machine.runtimeMachine = nil

	for _, waiter := range machine.waiters {
		waiter.Complete()
	}
	machine.waiters = nil

	if machine == s.main {
		// XXX: should we distinguish a test ending with a clean return vs
		// shutdown being called on the machine?
		gosimruntime.SetAbortError(gosimruntime.ErrMainReturned)
	}
}

func Runtime(fun func()) {
	setupSlog("os")

	timeoutTimer := time.AfterFunc(defaultTimeout, func() {
		slog.Error("test timeout")
		gosimruntime.SetAbortError(gosimruntime.ErrTimeout)
	})

	dispatcher := syscallabi.NewDispatcher()

	s := &Simulation{
		dispatcher: dispatcher,

		nextMachineID: 1,
		machinesById:  make(map[int]*Machine),

		timeoutTimer: timeoutTimer,

		rawLogger: slog.New(makeBaseSlogHandler()),
	}
	s.setupNetworkPorts()
	s.main = s.newMachine("main", netip.Addr{}, fs.NewLinuxFilesystem(), fun)
	s.startMachine(s.main)

	dispatcher.Run()
}
