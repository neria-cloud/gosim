//go:build go1.25 && !go1.26

// Package metrics provides declarations for dormant runtime metric collectors.
// The simulation advertises no host metrics; sampling is unsupported.
package metrics

import (
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type ValueKind int

const (
	KindBad ValueKind = iota
	KindUint64
	KindFloat64
	KindFloat64Histogram
)

type Description struct {
	Name, Description string
	Kind              ValueKind
	Cumulative        bool
}
type Float64Histogram struct {
	Counts  []uint64
	Buckets []float64
}
type (
	Value  struct{}
	Sample struct {
		Name  string
		Value Value
	}
)

func All() []Description { return nil }
func Read(_ []Sample) {
	simrt.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "rt",
			Operation: "runtime/metrics.Read",
			Message:   "host heap and scheduler metrics are not modeled",
		},
	)
}
func (Value) Kind() ValueKind  { return KindBad }
func (Value) Uint64() uint64   { panic("called Uint64 on non-uint64 metric value") }
func (Value) Float64() float64 { panic("called Float64 on non-float64 metric value") }
func (Value) Float64Histogram() *Float64Histogram {
	panic("called Float64Histogram on non-Float64Histogram metric value")
}
