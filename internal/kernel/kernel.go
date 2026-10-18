// Package kernel owns translated machine lifecycle and service routing. Only
// composition constructs concrete providers; the kernel sees their contracts.
package kernel

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type Config struct {
	// Composition supplies the normalized runtime identity and the digest of
	// the complete host build identity; host compiler/config packages stay out
	// of the translated dependency graph.
	Identity    abi.HostIdentity
	BuildDigest string
	Features    contracts.FeatureSet
	TimeoutNS   int64
}

type Kernel struct {
	features             contracts.FeatureSet
	machineByID          map[uint64]*Machine
	providers            []contracts.ProviderSession
	controls             contracts.ControlBindings
	engine               *simrt.Engine
	dispatcher           *Dispatcher
	runID, nextMachineID uint64

	config          Config
	owners          []contracts.Feature
	transactions    map[uint64]*StartTxn
	waits           map[contracts.WaitID]*waitRecord
	nextWaitID      contracts.WaitID
	events          map[contracts.EventID]contracts.EpochRef
	polls           map[uint64]*pollRegistration
	nextPollID      uint64
	traces          []traceRecord
	caller          contracts.CallContext
	root            uint64
	exits           simrt.Chan[[3]uint64]
	running, closed bool
	closeKind       contracts.StopKind
	closeErr        error
}

func New(config Config, engine *simrt.Engine, providers []contracts.Provider) (*Kernel, error) {
	ordered, err := contracts.OrderProviders(config.Features, providers)
	if err != nil {
		return nil, err
	}
	if len(config.BuildDigest) != 64 || strings.Trim(config.BuildDigest, "0123456789abcdef") != "" || config.TimeoutNS <= 0 || engine == nil {
		return nil, fmt.Errorf("%w: invalid kernel identity, timeout or engine", contracts.ErrInvalidConfig)
	}
	if err := engine.BeginTransition(); err != nil {
		return nil, err
	}
	engine.EndTransition()
	runID, err := simrt.NewRunID()
	if err != nil {
		return nil, err
	}
	k := &Kernel{
		features: config.Features, engine: engine, runID: runID, nextMachineID: 1,
		config: config, machineByID: make(map[uint64]*Machine), transactions: make(map[uint64]*StartTxn),
		waits: make(map[contracts.WaitID]*waitRecord), nextWaitID: 1, events: make(map[contracts.EventID]contracts.EpochRef),
		polls: make(map[uint64]*pollRegistration), nextPollID: 1,
		exits: simrt.NewChan[[3]uint64](0), closeKind: contracts.StopCrash,
	}
	k.dispatcher = NewDispatcher(k)
	for _, provider := range ordered {
		var session contracts.ProviderSession
		err := k.protected(func() error { var err error; session, err = provider.Open(k.Ports()); return err })
		if err == nil && session == nil {
			err = errors.New("provider returned a nil session")
		}
		if session != nil {
			k.providers = append(k.providers, session)
			k.owners = append(k.owners, provider.Feature())
		}
		if err == nil {
			err = k.protected(func() error {
				controls := session.Controls()
				if err := contracts.ValidateControls(provider.Feature(), controls); err != nil {
					return err
				}
				if controls.FSControl != nil {
					k.controls.FSControl = controls.FSControl
				}
				if controls.NetControl != nil {
					k.controls.NetControl = controls.NetControl
				}
				return nil
			})
		}
		if err != nil {
			failures := []error{err}
			for i := len(k.providers) - 1; i >= 0; i-- {
				failures = append(failures, k.protected(func() error { k.providers[i].Close(); return nil }))
			}
			return nil, errors.Join(failures...)
		}
	}
	return k, nil
}

// Run executes inside Engine.Run's surviving supervisor. It never creates a
// second scheduler. Driver owns Engine.Close after this cleanup has returned.
func (k *Kernel) Run(main func()) (result error) {
	if k.running || k.closed || k.root != 0 || main == nil {
		return contracts.ErrInvalidState
	}
	k.running = true
	defer func() {
		result = errors.Join(result, k.Close())
		k.running = false
		if result != nil {
			k.cleanupFailure("run cleanup", result)
		}
	}()
	ref, err := k.Create("root", main)
	if err != nil {
		return err
	}
	k.root = ref.ID
	if err := k.Start(ref); err != nil {
		return err
	}
	for {
		index, payload, _ := simrt.Select(k.exits.RecvSelector(), k.dispatcher.incoming.RecvSelector())
		if index == 1 {
			k.dispatcher.DispatchOne(payload.(*Invocation))
			continue
		}
		request := payload.([3]uint64)
		id, epoch, kind := request[0], request[1], contracts.StopKind(request[2])
		m := k.machineByID[id]
		if m == nil || m.state != StateRunning || m.current.ref.Epoch != epoch || (kind != contracts.StopGraceful && kind != contracts.StopCrash) {
			k.invariant("invalid self-exit request")
			return contracts.ErrInvalidState
		}
		if id == k.root {
			k.closeKind = kind
			return nil
		}
		if err := k.stop(m, kind); err != nil {
			return err
		}
	}
}

func (k *Kernel) Close() error {
	if k.closed {
		return k.closeErr
	}
	k.closed = true
	var ids []uint64
	for id := range k.machineByID {
		if id != k.root {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	if k.root != 0 {
		ids = append(ids, k.root)
	}
	var failures []error
	for _, id := range ids {
		m := k.machineByID[id]
		if tx := k.transactions[id]; tx != nil {
			failures = append(failures, tx.abort())
		}
		if m.state == StateRunning {
			failures = append(failures, k.stop(m, k.closeKind))
		}
		for i := len(k.providers) - 1; i >= 0; i-- {
			failures = append(failures, k.protected(func() error { k.providers[i].Destroy(m.ref); return nil }))
		}
		m.state, m.main = StateDestroyed, nil
	}
	failures = append(failures, k.dispatcher.Drain())
	for i := len(k.providers) - 1; i >= 0; i-- {
		failures = append(failures, k.protected(func() error { k.providers[i].Close(); return nil }))
	}
	k.controls = contracts.ControlBindings{}
	k.providers, k.owners, k.waits, k.events = nil, nil, nil, nil
	k.closeErr = errors.Join(failures...)
	return k.closeErr
}

// A provider contract violation is retained while best-effort cleanup continues.
// This wrapper never changes the stop mode or retries a provider operation.
func protected(f func() error) (result error) {
	defer func() {
		if failure := recover(); failure != nil {
			result = fmt.Errorf("provider invariant: %v", failure)
		}
	}()
	return f()
}

func (k *Kernel) protected(f func() error) error {
	return protected(func() error {
		if err := k.engine.BeginTransition(); err != nil {
			return err
		}
		defer k.engine.EndTransition()
		return f()
	})
}
