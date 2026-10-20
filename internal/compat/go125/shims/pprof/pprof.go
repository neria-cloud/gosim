//go:build go1.25 && !go1.26

// Package pprof models diagnostic labels. Native profiling operations abort.
package pprof

import (
	"context"
	"encoding/json"
	"io"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type (
	LabelSet struct{ values map[string]string }
	labelKey struct{}
)

func Labels(args ...string) LabelSet {
	if len(args)%2 != 0 {
		panic("uneven number of label arguments")
	}
	labels := LabelSet{values: make(map[string]string)}
	for i := 0; i < len(args); i += 2 {
		labels.values[args[i]] = args[i+1]
	}
	return labels
}

func WithLabels(ctx context.Context, labels LabelSet) context.Context {
	values := make(map[string]string)
	ForLabels(ctx, func(k, v string) bool { values[k] = v; return true })
	for k, v := range labels.values {
		values[k] = v
	}
	return context.WithValue(ctx, labelKey{}, LabelSet{values: values})
}

func Label(ctx context.Context, key string) (string, bool) {
	labels := labelsForContext(ctx)
	value, ok := labels.values[key]
	return value, ok
}

func ForLabels(ctx context.Context, f func(key, value string) bool) {
	labels := labelsForContext(ctx)
	for k, v := range labels.values {
		if !f(k, v) {
			break
		}
	}
}

func SetGoroutineLabels(ctx context.Context) {
	labels := labelsForContext(ctx)
	data, err := json.Marshal(labels.values)
	if err != nil {
		panic(err)
	}
	gosimruntime.SetProfileLabels(string(data))
}

func labelsForContext(ctx context.Context) LabelSet {
	labels, ok := ctx.Value(labelKey{}).(LabelSet)
	if !ok {
		return LabelSet{}
	}
	return labels
}

func Do(ctx context.Context, labels LabelSet, f func(context.Context)) {
	previous := gosimruntime.ProfileLabels()
	defer gosimruntime.SetProfileLabels(previous)
	labeledCtx := WithLabels(ctx, labels)
	SetGoroutineLabels(labeledCtx)
	f(labeledCtx)
}

func unsupported() {
	gosimruntime.Abort(abi.Diagnostic{
		Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime/pprof",
		Message: "native profiling is not available in simulation",
	})
}

type Profile struct{}

func NewProfile(_ string) *Profile                  { unsupported(); return nil }
func Lookup(_ string) *Profile                      { unsupported(); return nil }
func Profiles() []*Profile                          { unsupported(); return nil }
func (_ *Profile) Name() string                     { unsupported(); return "" }
func (_ *Profile) Count() int                       { unsupported(); return 0 }
func (_ *Profile) Add(_ any, _ int)                 { unsupported() }
func (_ *Profile) Remove(_ any)                     { unsupported() }
func (_ *Profile) WriteTo(_ io.Writer, _ int) error { unsupported(); return nil }
func StartCPUProfile(_ io.Writer) error             { unsupported(); return nil }
func StopCPUProfile()                               { unsupported() }
func WriteHeapProfile(_ io.Writer) error            { unsupported(); return nil }
