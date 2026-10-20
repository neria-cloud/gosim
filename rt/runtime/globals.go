package gosimruntime

import (
	"cmp"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"slices"
	"time"
	"unsafe"
)

// An atomicValue is a value that can be read or written without triggering the
// race detector.
type atomicValue[T any] struct {
	value T
}

// get returns the current value. It is not marked go:norace to allow inlining.
func (v *atomicValue[T]) get() T {
	return v.value
}

// set writes the current value. It is marked go:norace to allow reads without
// triggering the race detector.
//
//go:norace
func (v *atomicValue[T]) set(t T) {
	v.value = t
}

var globalPtr atomicValue[unsafe.Pointer]

func allocGlobals() unsafe.Pointer {
	return reflect.New(mergedGlobalTyp).UnsafePointer()
}

type Global[T any] struct {
	offset uintptr
	info   *Globals
}

func (g *Global[T]) typ() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

//go:norace
func (g *Global[T]) setOffset(x uintptr) {
	g.offset = x
}

type Globaler interface {
	setOffset(uintptr)
	setInfo(*Globals)
	typ() reflect.Type
}

func (g *Global[T]) setInfo(info *Globals) { g.info = info }

func (g Global[T]) Get() *T {
	if e := infrastructureEngine(); e != nil {
		e.initializeInfrastructure(g.info)
	}
	return (*T)(unsafe.Pointer((uintptr(globalPtr.get()) + g.offset)))
}

// Only native scheduler metadata is inspected here. Application accesses through
// the returned global pointer remain instrumented by the race detector.
//
//go:norace
func infrastructureEngine() *Engine {
	if s := gs.get(); s != nil && s.engine != nil && s.engine.supervisor != nil &&
		globalPtr.get() == s.engine.supervisor.globals {
		return s.engine
	}
	return nil
}

// Supervisor storage is independent of every application machine. Initialize
// owned composition dependencies on first access, including transitive globals
// used by their initializers. Application and test initializers never run here.
func (e *Engine) initializeInfrastructure(info *Globals) {
	if info == nil || !info.Infrastructure {
		panic("application globals accessed on supervisor")
	}
	if e.initializedGlobals[info] {
		return
	}
	if e.initializedGlobals == nil {
		e.initializedGlobals = make(map[*Globals]bool)
	}
	e.initializedGlobals[info] = true // initializer writes reenter Global.Get
	if err := e.BeginTransition(); err != nil {
		panic(err)
	}
	defer e.EndTransition()
	info.Initializers(false)
}

var mergedGlobalTyp reflect.Type

var (
	packages       []*PackageInfo
	packagesByPath = make(map[string]*PackageInfo)
)

//go:norace
func InitGlobals(benchmark, initializeShared bool) {
	// now := time.Now()
	// defer func() {
	// totalInitTime += time.Since(now)
	// }()
	if benchmark { // XXX: optional debug package init time
		type stat struct {
			path     string
			duration time.Duration
			allocs   int64
			size     int64
		}

		var stats []stat

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		prev := memStats.TotalAlloc

		outerStart := time.Now()
		outerPrev := memStats.TotalAlloc

		for _, pkg := range packages {
			if pkg.Globals != nil {
				start := time.Now()

				pkg.Globals.Initializers(initializeShared)

				duration := time.Since(start)
				runtime.ReadMemStats(&memStats)
				stats = append(stats, stat{
					path:     pkg.Path,
					duration: duration,
					allocs:   int64(memStats.TotalAlloc - prev),
					size:     int64(pkg.Globals.Globals.typ().Size()),
				})
				prev = memStats.TotalAlloc
			}
			if pkg.ForTest != nil {
				start := time.Now()
				pkg.ForTest.Initializers(initializeShared)
				duration := time.Since(start)
				runtime.ReadMemStats(&memStats)
				stats = append(stats, stat{
					path:     pkg.Path + " (for test)",
					duration: duration,
					allocs:   int64(memStats.TotalAlloc - prev),
					size:     int64(pkg.Globals.Globals.typ().Size()),
				})
				prev = memStats.TotalAlloc
			}
		}

		slices.SortFunc(stats, func(a, b stat) int {
			return -cmp.Compare(a.duration, b.duration)
		})

		slog.Info(
			"total initialization",
			"duration",
			time.Since(outerStart),
			"allocs",
			memStats.TotalAlloc-outerPrev,
			"size",
			mergedGlobalTyp.Size(),
		)

		for _, pkg := range stats {
			slog.Info(
				"initialization",
				"pkg",
				pkg.path,
				"duration",
				pkg.duration,
				"allocs",
				pkg.allocs,
				"size",
				pkg.size,
			)
		}
	} else {
		for _, pkg := range packages {
			if pkg.Globals != nil {
				pkg.Globals.Initializers(initializeShared)
			}
			if pkg.ForTest != nil {
				pkg.ForTest.Initializers(initializeShared)
			}
		}
	}
	// log.Println(totalInitTime) // XXX find a place to log this, from TestMain?
	// XXX: and make it race safe
}

//go:norace
func prepareGlobals() {
	var fields []reflect.StructField
	offset := 0
	for _, pkg := range packages {
		if pkg.Globals != nil {
			fields = append(fields, reflect.StructField{
				Name: "Globals" + fmt.Sprint(offset),
				Type: pkg.Globals.Globals.typ(),
			})
			offset++
		}
		if pkg.ForTest != nil {
			fields = append(fields, reflect.StructField{
				Name: "Globals" + fmt.Sprint(offset),
				Type: pkg.ForTest.Globals.typ(),
			})
			offset++
		}
	}
	mergedGlobalTyp = reflect.StructOf(fields)
	offset = 0
	for _, pkg := range packages {
		if pkg.Globals != nil {
			pkg.Globals.Globals.setOffset(mergedGlobalTyp.Field(offset).Offset)
			pkg.Globals.Globals.setInfo(pkg.Globals)
			offset++
		}
		if pkg.ForTest != nil {
			pkg.ForTest.Globals.setOffset(mergedGlobalTyp.Field(offset).Offset)
			pkg.ForTest.Globals.setInfo(pkg.ForTest)
			offset++
		}
	}
}

type Globals struct {
	Globals        Globaler
	Initializers   func(initializeShared bool)
	Infrastructure bool
}

type PackageInfo struct {
	Path    string
	Globals *Globals
	ForTest *Globals
}

func RegisterPackage(path string) *PackageInfo {
	if runtimeInitialized {
		panic("calling register package after already initialized")
	}
	if p, ok := packagesByPath[path]; ok {
		return p
	}
	p := &PackageInfo{
		Path: path,
	}
	packages = append(packages, p)
	packagesByPath[path] = p
	return p
}
