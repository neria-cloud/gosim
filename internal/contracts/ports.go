package contracts

type (
	Clock  interface{ NowNanos() int64 }
	Random interface {
		Uint64() uint64
		Uint64N(uint64) uint64
		Float64() float64
		ExpFloat64() float64
	}
)

type Events interface {
	Schedule(EpochRef, int64, func()) (EventID, error)
	Cancel(EventID)
	CancelEpoch(EpochRef)
}
type ReadinessSink interface {
	Notify(EpochRef, int, Readiness)
}
type Trace interface {
	Emit(EpochRef, string, []Field)
}
type (
	Field struct{ Key, Value string }
	Ports struct {
		Clock     Clock
		Events    Events
		Trace     Trace
		RandomFor func(Feature, MachineRef) Random
	}
)
