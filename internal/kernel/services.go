package kernel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type traceRecord struct {
	epoch  contracts.EpochRef
	event  string
	fields []contracts.Field
}
type clockPort struct{}

func (clockPort) NowNanos() int64 { return simrt.Nanotime() }

type (
	eventPort struct{ kernel *Kernel }
	tracePort struct{ kernel *Kernel }
)

func featureName(feature contracts.Feature) string {
	switch feature {
	case contracts.RT:
		return "rt"
	case contracts.FS:
		return "fs"
	case contracts.Net:
		return "net"
	}
	return fmt.Sprintf("invalid(%d)", feature)
}

func (k *Kernel) Require(ctx contracts.CallContext, feature contracts.Feature, operation string) {
	if k.features.Has(feature) {
		return
	}
	k.AbortRun(contracts.Diagnostic{
		Kind: string(contracts.FeatureDisabled), Feature: featureName(feature), Operation: operation,
		Epoch: ctx.Epoch, PC: ctx.PC, Message: "operation requires a disabled feature",
	})
}

func (k *Kernel) AbortRun(diagnostic contracts.Diagnostic) {
	k.engine.Abort(abi.Diagnostic{
		Kind: diagnostic.Kind, Feature: diagnostic.Feature, Operation: diagnostic.Operation,
		Message: diagnostic.Message, RunID: diagnostic.Epoch.Machine.RunID, MachineID: diagnostic.Epoch.Machine.ID,
		Epoch: diagnostic.Epoch.Epoch, PC: diagnostic.PC,
	})
}

func (k *Kernel) invariant(message string) {
	k.AbortRun(contracts.Diagnostic{Kind: string(contracts.InvariantFailure), Message: message})
}

func (k *Kernel) cleanupFailure(operation string, err error) {
	k.AbortRun(contracts.Diagnostic{Kind: string(contracts.InvariantFailure), Operation: operation, Message: err.Error()})
}

func (k *Kernel) Ports() contracts.Ports {
	ports := contracts.Ports{
		Clock: clockPort{}, Events: eventPort{k}, Trace: tracePort{k},
		RandomFor: func(feature contracts.Feature, ref contracts.MachineRef) contracts.Random {
			if (feature != contracts.FS && feature != contracts.Net) || !k.features.Has(feature) {
				k.invariant("provider requested an unowned random stream")
				return nil
			}
			if ref.ID == 0 {
				if ref.RunID != 0 && ref.RunID != k.runID {
					k.invariant("wrong-run session random stream")
					return nil
				}
			} else if _, err := k.Lookup(ref); err != nil {
				k.invariant("invalid machine random stream")
				return nil
			}
			return k.engine.RandomFor(featureName(feature), ref.ID)
		},
	}
	return ports
}

func (p eventPort) Schedule(epoch contracts.EpochRef, when int64, callback func()) (contracts.EventID, error) {
	k := p.kernel
	// Closing rejects application admission, but ordered provider cleanup may
	// enqueue a final notification for a still-live destination. Its later
	// teardown cancels the event; callbacks still require normal admission.
	if !k.liveEpoch(epoch) || callback == nil {
		return 0, contracts.ErrInvalidState
	}
	var id uint64
	var err error
	id, err = k.engine.Schedule(simrt.MachineToken{ID: epoch.Machine.ID, Epoch: epoch.Epoch}, when, func() {
		delete(k.events, contracts.EventID(id))
		if !k.Accepts(epoch) {
			p.kernel.Ports().Trace.Emit(epoch, "discard-stale-event", nil)
			return
		}
		callback()
	})
	if err != nil {
		return 0, err
	}
	eventID := contracts.EventID(id)
	k.events[eventID] = epoch
	return eventID, nil
}

func (p eventPort) Cancel(id contracts.EventID) {
	if _, ok := p.kernel.events[id]; ok {
		p.kernel.engine.CancelEvent(uint64(id))
		delete(p.kernel.events, id)
	}
}
func (p eventPort) CancelEpoch(epoch contracts.EpochRef) { p.kernel.cancelEvents(epoch) }
func (k *Kernel) cancelEvents(epoch contracts.EpochRef) {
	var ids []contracts.EventID
	for id, owner := range k.events {
		if owner == epoch {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		k.engine.CancelEvent(uint64(id))
		delete(k.events, id)
	}
}

func (p tracePort) Emit(epoch contracts.EpochRef, event string, fields []contracts.Field) {
	p.kernel.traces = append(p.kernel.traces, traceRecord{epoch: epoch, event: event, fields: slices.Clone(fields)})
	if simrt.TraceNetwork.Enabled() && strings.HasPrefix(event, "network.") {
		values := make([]string, 0, 2+len(fields)*2)
		values = append(values, "traceKind", "network")
		for _, field := range fields {
			values = append(values, field.Key, field.Value)
		}
		ctx := p.kernel.caller
		p.kernel.engine.WriteProviderTrace(tokenFor(ctx.Epoch), ctx.GoroutineID, ctx.PC, event, values)
	}
}
