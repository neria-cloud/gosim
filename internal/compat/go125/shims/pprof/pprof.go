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
	labels, _ := ctx.Value(labelKey{}).(LabelSet)
	value, ok := labels.values[key]
	return value, ok
}

func ForLabels(ctx context.Context, f func(key, value string) bool) {
	labels, _ := ctx.Value(labelKey{}).(LabelSet)
	for k, v := range labels.values {
		if !f(k, v) {
			break
		}
	}
}

func SetGoroutineLabels(ctx context.Context) {
	labels, _ := ctx.Value(labelKey{}).(LabelSet)
	data, err := json.Marshal(labels.values)
	if err != nil {
		panic(err)
	}
	gosimruntime.SetProfileLabels(string(data))
}

func Do(ctx context.Context, labels LabelSet, f func(context.Context)) {
	previous := gosimruntime.ProfileLabels()
	defer gosimruntime.SetProfileLabels(previous)
	labeledContext := WithLabels(ctx, labels)
	SetGoroutineLabels(labeledContext)
	f(labeledContext)
}

func unsupported() {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "runtime/pprof", Message: "native profiling is not available in simulation"})
}

type Profile struct{}

func NewProfile(name string) *Profile                   { unsupported(); return nil }
func Lookup(name string) *Profile                       { unsupported(); return nil }
func Profiles() []*Profile                              { unsupported(); return nil }
func (p *Profile) Name() string                         { unsupported(); return "" }
func (p *Profile) Count() int                           { unsupported(); return 0 }
func (p *Profile) Add(value any, skip int)              { unsupported() }
func (p *Profile) Remove(value any)                     { unsupported() }
func (p *Profile) WriteTo(w io.Writer, debug int) error { unsupported(); return nil }
func StartCPUProfile(w io.Writer) error                 { unsupported(); return nil }
func StopCPUProfile()                                   { unsupported() }
func WriteHeapProfile(w io.Writer) error                { unsupported(); return nil }
