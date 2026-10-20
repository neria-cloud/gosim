package kernel

import (
	"errors"
	"fmt"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

type TxnState uint8

const (
	Prepared TxnState = iota
	Committed
	Aborted
)

var ErrTxnState error = contracts.Sentinel("invalid start transaction state")

type RuntimePrepared struct {
	RuntimeID uint64
	Epoch     contracts.EpochRef
}

func (r *RuntimePrepared) Token() simrt.MachineToken {
	return simrt.MachineToken{ID: r.RuntimeID, Epoch: r.Epoch.Epoch}
}

type StartTxn struct {
	machine       *Machine
	previous      State
	run           *Run
	staged        []contracts.PreparedMachine
	runtimeStaged *RuntimePrepared
	state         TxnState
	kernel        *Kernel
}

func (k *Kernel) BeginStart(ref contracts.MachineRef, restart bool) (*StartTxn, error) {
	m, err := k.Lookup(ref)
	if err != nil {
		return nil, err
	}
	if k.closed {
		return nil, contracts.ErrInvalidState
	}
	if restart {
		if err := k.returningTransition(m); err != nil {
			return nil, err
		}
		if m.state != StateStopped && m.state != StateCrashed {
			return nil, contracts.ErrInvalidState
		}
	} else if m.state != StateNew {
		return nil, contracts.ErrInvalidState
	}
	epoch, err := consumeEpoch(m)
	if err != nil {
		k.invariant(err.Error())
		return nil, err
	}
	tx := &StartTxn{
		kernel: k, machine: m, previous: m.state,
		run: &Run{ref: contracts.EpochRef{Machine: ref, Epoch: epoch}, runtimeID: ref.ID},
	}
	tx.run.descriptors = NewFDTable(tx.run.ref)
	m.state = StateStarting
	k.transactions[ref.ID] = tx
	err = k.protected(func() error {
		token, err := k.engine.PrepareMachine(ref.ID, epoch, m.label)
		if err != nil {
			return err
		}
		tx.runtimeStaged = &RuntimePrepared{RuntimeID: token.ID, Epoch: tx.run.ref}
		if err := tx.run.descriptors.installDiagnostics(k); err != nil {
			return err
		}
		for _, session := range k.providers {
			staged, err := session.Prepare(
				contracts.StartSpec{Ref: tx.run.ref, Label: m.label, Restart: restart},
				tx.run.descriptors,
			)
			if err != nil {
				return err
			}
			if staged == nil {
				return errors.New("provider returned a nil prepared machine")
			}
			tx.staged = append(tx.staged, staged)
		}
		bindings, err := k.mergeBindings(tx.staged)
		if err != nil {
			return err
		}
		tx.run.bindings = bindings
		return nil
	})
	if err != nil {
		if cleanupErr := tx.abort(); cleanupErr != nil {
			k.cleanupFailure("startup rollback", errors.Join(err, cleanupErr))
		}
		return nil, err
	}
	return tx, nil
}

func (k *Kernel) mergeBindings(staged []contracts.PreparedMachine) (contracts.Bindings, error) {
	if len(staged) != len(k.owners) {
		return contracts.Bindings{}, errors.New("incomplete provider preparation")
	}
	groups := make([]contracts.Bindings, 0, len(staged))
	for i, prepared := range staged {
		bindings := prepared.Bindings()
		if err := contracts.ValidateBindings(k.owners[i], bindings); err != nil {
			return contracts.Bindings{}, err
		}
		groups = append(groups, bindings)
	}
	return contracts.MergeBindings(groups)
}

func (tx *StartTxn) Commit() error {
	if tx.state != Prepared {
		return ErrTxnState
	}
	k := tx.kernel
	// The supervisor owns all transitions. Reject voluntary scheduling and
	// blocking while publication is atomic, preserving native boot parking.
	attempted := 0
	err := k.protected(func() error {
		for _, staged := range tx.staged {
			attempted++
			instance := staged.Publish()
			if instance == nil {
				panic("provider published a nil instance")
			}
			tx.run.instances = append(tx.run.instances, instance)
		}
		tx.machine.current = tx.run
		tx.run.accepting = true
		tx.machine.state = StateRunning
		tx.state = Committed
		delete(k.transactions, tx.machine.ref.ID)
		main, runtimeToken, exits := tx.machine.main, tx.runtimeStaged.Token(), k.exits
		k.engine.PublishMachine(tx.runtimeStaged.Token(), func() {
			// Engine's composition initializer runs before this boot callback.
			// No application code runs before publication.
			main()
			requestSelfExit(exits, runtimeToken, contracts.StopGraceful)
		})
		return nil
	})
	if err != nil {
		// Publication is irreversible. Retain committed instances for crash
		// cleanup; abort only providers whose Publish has not been attempted.
		tx.run.unpublished = tx.staged[attempted:]
		tx.machine.current, tx.machine.state = tx.run, StateRunning
		tx.run.accepting, tx.state = false, Committed
		delete(k.transactions, tx.machine.ref.ID)
		tx.staged = nil
		k.cleanupFailure("provider publication", err)
		return err
	}
	tx.staged, tx.runtimeStaged = nil, nil
	return nil
}

func (tx *StartTxn) abort() error {
	if tx.state == Aborted {
		return nil
	}
	if tx.state == Committed {
		return ErrTxnState
	}
	var failures []error
	failures = append(failures, tx.kernel.protected(func() error {
		return tx.run.descriptors.Finalize(contracts.CloseAbort)
	}))
	for i := len(tx.staged) - 1; i >= 0; i-- {
		failures = append(failures, tx.kernel.protected(func() error { tx.staged[i].Abort(); return nil }))
	}
	if tx.runtimeStaged != nil {
		tx.kernel.engine.AbortMachine(tx.runtimeStaged.Token())
	}
	tx.machine.state = tx.previous
	tx.state = Aborted
	delete(tx.kernel.transactions, tx.machine.ref.ID)
	tx.staged, tx.runtimeStaged, tx.run = nil, nil, nil
	return errors.Join(failures...)
}

func (tx *StartTxn) Abort() {
	if err := tx.abort(); err != nil {
		tx.kernel.cleanupFailure("startup rollback", fmt.Errorf("%w", err))
	}
}
