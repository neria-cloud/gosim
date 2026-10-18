package simulation

import (
	"fmt"
	"net/netip"
	"syscall"
	"time"

	"github.com/neria-cloud/gosim/rt/runtime/abi"

	fs "github.com/neria-cloud/gosim/internal/fs/model"
	network "github.com/neria-cloud/gosim/internal/net/model"
	"github.com/neria-cloud/gosim/internal/simulation/syscallabi"
)

const defaultTimeout = 10 * time.Minute

// GosimOS implements all simulation-level system calls, such as creating
// machines, messing with the network, etc.
type GosimOS struct {
	dispatcher syscallabi.Dispatcher

	simulation *Simulation

	// TODO: this is currently protected by simulation.mu.

	// TODO: have a common by-ID scheme because I don't want infinite maps, please
	nextCrashIterId    int
	crashItersById     map[int]func() (int, bool)
	crashIterCloseByID map[int]func()
}

func NewGosimOS(s *Simulation, d syscallabi.Dispatcher) *GosimOS {
	return &GosimOS{
		dispatcher: d,

		simulation: s,

		crashItersById:     make(map[int]func() (int, bool)),
		crashIterCloseByID: make(map[int]func()),
	}
}

func (g *GosimOS) SetSimulationTimeout(d time.Duration, invocation *syscallabi.Syscall) error {
	g.simulation.timeoutTimer.Reset(d)
	return nil
}

func (g *GosimOS) MachineNew(label string, addrStr string, program any, invocation *syscallabi.Syscall) (machineId int) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	// TODO: handle failure but do allow an empty addrStr to indicate unset addr
	addr, _ := netip.ParseAddr(addrStr)

	bootProgram := program.(func())

	machine := g.simulation.newMachine(label, addr, fs.NewLinuxFilesystem(), bootProgram)

	// TODO: don't start machine???
	g.simulation.startMachine(machine)

	return machine.id
}

func (g *GosimOS) MachineRecoverInit(machineID int, program any, invocation *syscallabi.Syscall) int {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]
	fs := machine.filesystem

	// TODO(migration): remove this legacy adapter in Task 22.
	image := fs.FreezeCrash()
	iter := image.IterCrashes()
	image.Release()
	counter := 0

	id := g.nextCrashIterId
	g.nextCrashIterId++
	g.crashIterCloseByID[id] = iter.Close

	bootProgram := program.(func())

	g.crashItersById[id] = func() (int, bool) {
		fs, ok := iter.Next()
		if !ok {
			return 0, false
		}

		label := fmt.Sprintf("%s-iter-recover-%d", machine.label, counter)
		counter++

		var addr netip.Addr
		newMachine := g.simulation.newMachine(label, addr, fs, bootProgram)
		g.simulation.startMachine(newMachine)

		return newMachine.id, true
	}
	return id
}

func (g *GosimOS) MachineRecoverNext(crashIterId int, invocation *syscallabi.Syscall) (int, bool) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	// XXX: jank
	iter := g.crashItersById[crashIterId]
	return iter()
}

func (g *GosimOS) MachineRecoverRelease(crashIterId int, invocation *syscallabi.Syscall) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	if close := g.crashIterCloseByID[crashIterId]; close != nil {
		close()
	}
	delete(g.crashIterCloseByID, crashIterId)
	delete(g.crashItersById, crashIterId)
}

func (g *GosimOS) MachineInodeInfo(machineID int, inode int, info abi.ValueView[fs.InodeInfo], invocation *syscallabi.Syscall) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]
	info.Set(machine.filesystem.GetInodeInfo(inode))
}

func (g *GosimOS) MachineWait(machineID int, invocation *syscallabi.Syscall) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]
	if machine.runtimeMachine == nil {
		// XXX: improve how we mark stopped please
		invocation.Complete()
	} else {
		machine.waiters = append(machine.waiters, invocation)
	}
}

func (g *GosimOS) MachineStop(machineID int, graceful bool, invocation *syscallabi.Syscall) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]
	g.simulation.stopMachine(machine, graceful)
}

func (g *GosimOS) MachineRestart(machineID int, partialDisk bool, invocation *syscallabi.Syscall) (err error) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]

	if machine.runtimeMachine != nil {
		return syscall.EALREADY
	}

	// TODO(migration): session lifecycle replaces this adapter in Task 22.
	image := machine.filesystem.FreezeCrash()
	machine.filesystem.Release()
	machine.filesystem = image.Clone(partialDisk, machine.filesystemRandom)
	image.Release()
	g.simulation.startMachine(machine)

	// XXX: return a new machine ID?
	// what if a machine gets restarted multiple times?
	// do we track some kind of status?
	return nil
}

func (g *GosimOS) MachineGetLabel(machineID int, invocation *syscallabi.Syscall) (label string, err error) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]

	return machine.label, nil
}

func (g *GosimOS) MachineSetBootProgram(machineID int, program any, invocation *syscallabi.Syscall) (err error) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]
	machine.bootProgram = program.(func())

	return nil
}

func (g *GosimOS) MachineSetSometimesCrashOnSync(machineID int, value bool, invocation *syscallabi.Syscall) (err error) {
	g.simulation.mu.Lock()
	defer g.simulation.mu.Unlock()

	machine := g.simulation.machinesById[machineID]
	// XXX: on restart how do we copy these?
	machine.sometimesCrashOnSyncMu.Lock()
	machine.sometimesCrashOnSync = true
	machine.sometimesCrashOnSyncMu.Unlock()
	return nil
}

func (g *GosimOS) ConfigureNetwork(request abi.ValueView[network.ControlRequest], invocation *syscallabi.Syscall) error {
	return g.simulation.network.ApplyControl(request.Get())
}

func (g *GosimOS) SetConnected(a, b string, connected bool, invocation *syscallabi.Syscall) error {
	pair, err := network.ParseHostPair(a, b)
	if err != nil {
		return err
	}
	return g.simulation.network.SetConnected(pair, connected)
}

func (g *GosimOS) SetDelay(a, b string, delay time.Duration, invocation *syscallabi.Syscall) error {
	pair, err := network.ParseHostPair(a, b)
	if err != nil {
		return err
	}
	return g.simulation.network.SetDelay(pair, delay)
}
