# Per-machine syscall counters: feasibility and proposal

**Date:** 2026-10-21  
**Gosim revision:** `453947f` (`feat/compact-go-versions`)  
**Status:** proposal only; nothing in this report has been implemented

## Summary

The request is to let simulated code read per-machine statistics as `{syscall_name: count}` pairs through a Gosim-specific method.

**This is feasible and moderately small.** Every syscall that a translated application submits to the simulated kernel already passes through one function, `Dispatcher.DispatchOne` in `internal/kernel/dispatcher.go:154`. That function runs on the supervisor once per request and knows both the requesting machine and the operation. Counting there needs a few lines and no locking. Reading the counters needs one new kernel control route, declared in `internal/routegen/routes.json` and generated like the existing `FSStats` and `Info` routes. It also needs a public method such as `gosim.Machine.SyscallStats()`.

Estimated effort: roughly 100–200 hand-written lines across 6–8 files, plus regenerated code, tests and documentation. Most of the cost is the route, regeneration and tests; the counting logic itself is trivial.

The main design decision is what "syscall name" means. Gosim does not execute Linux syscall numbers. It routes typed **kernel operations** such as `Read`, `Openat` and `Statat`. Counting those operations is accurate, cheap and deterministic. A Linux-style name can be attached through a lookup table, but it is approximate where several Go entry points share one route.

## How a syscall reaches the simulated kernel

```text
application code (translated)
  os.ReadFile / net.Dial / ...
    -> translated stdlib wrapper (syscall.read, unix.Fstat, ...)
      -> generated hook adapter          internal/compat/hooks/{kernel,fs,net}/syscalls_<arch>.go
        -> Call(k.ReadRequest{...})       internal/compat/hooks/kernel/transport.go:16
          -> control.Submit               internal/control/dispatch.go:59
            -> Dispatcher.Submit          internal/kernel/dispatcher.go:88  (unbuffered channel to supervisor)
              -> supervisor loop          internal/kernel/kernel.go:136-138
                -> Dispatcher.DispatchOne internal/kernel/dispatcher.go:154  <- proposed counting point
                  -> decode (generated)   internal/kernel/dispatch_gen.go:12
                    -> kernel / fs / net service
```

Facts that make `DispatchOne` the right place:

- **One call per request.** Delayed filesystem completions finish through `Dispatcher.Complete` (`dispatcher.go:307`), not through `DispatchOne` again. A counter in `DispatchOne` never counts one request twice.
- **The caller is known.** `inv.Context.Epoch.Machine` identifies the machine and epoch, and `admit` (`dispatcher.go:124`) has already checked that the epoch is live.
- **The operation is known.** `inv.Route` is a `RouteID`, and `RouteID.String()` (`internal/kernel/routes_gen.go:95`) returns its name.
- **Single-threaded.** Dispatch runs on the supervisor, one request at a time. The function is already `//go:norace`.
- **Precedent.** The existing syscall trace writes `call <Route>` at exactly this point (`dispatcher.go:160`) when `-simtrace syscall` is enabled.

## What would be counted

### Route names and the closest Linux syscall

Routes are declared in `internal/routegen/routes.json` with a `kind`. The `syscall` and `poll` kinds come from application I/O. The `control`, `wait` and `self` kinds are Gosim API calls (`NewMachine`, `Label`, `fs.Stats`, `rt.Info`, `Require` and so on). Those should not be counted.

The Go entry points below come from `adapterOperations` in `internal/routegen/adapters.go:20`, plus hand-written hooks.

| Route | Main Go entry points | Closest Linux syscall |
| --- | --- | --- |
| `Read` | `syscall.read`, `unix.read` | `read` |
| `Write` | `syscall.write`, `unix.write`, `internal/poll.writev` (flattened) | `write`, `writev` |
| `Close` | `Close` | `close` |
| `FcntlInt` | `fcntl`, `FcntlInt` | `fcntl` |
| `FcntlLock` | `FcntlFlock` | `fcntl` (lock commands) |
| `Getrandom` | `Getrandom`, `GetRandom`, `vgetrandom` | `getrandom` |
| `Getpid`, `Uname` | `Getpid`, `Uname` | `getpid`, `uname` |
| `Openat` | `openat`, `Openat` | `openat` |
| `Renameat` | `Renameat` | `renameat` |
| `Getdents64` | `Getdents` | `getdents64` |
| `Fallocate` | `Fallocate` | `fallocate` |
| `Seek` | `Seek` | `lseek` |
| `Pread`, `Pwrite` | `pread`, `pwrite` | `pread64`, `pwrite64` |
| `Sync` | `Fsync`, `Fdatasync` | `fsync`, `fdatasync` |
| `Fstat`, `Statat` | `Fstat`, `fstatat`, `Fstatat` | `fstat`, `newfstatat` |
| `StatFS`, `FstatFS` | `Statfs`, `Fstatfs` | `statfs`, `fstatfs` |
| `Unlinkat`, `Mkdirat` | `unlinkat`, `Unlinkat`, `Mkdirat` | `unlinkat`, `mkdirat` |
| `Truncate` | `Ftruncate` | `ftruncate` |
| `Chdir`, `Getcwd` | `Chdir`, `Getcwd` | `chdir`, `getcwd` |
| `Fchmod`, `Chmodat`, `Fchown`, `Chownat` | same names | `fchmod`, `fchmodat`, `fchown`, `fchownat` |
| `Flock` | `Flock` | `flock` |
| `Map`, `Unmap`, `Advise` | `mmap`, `munmap`, `Madvise` | `mmap`, `munmap`, `madvise` |
| `Socket`, `Bind`, `Listen`, `Connect` | same names | same names |
| `Accept` | `accept4` | `accept4` |
| `LocalAddress`, `PeerAddress` | `getsockname`, `getpeername` | `getsockname`, `getpeername` |
| `SetOption`, `GetOption` | `setsockopt`, `getsockopt` | `setsockopt`, `getsockopt` |
| `PollOpen`, `PollClose`, `PollWait`, `PollReset`, `PollDeadline`, `PollUnblock` | `internal/poll` runtime hooks (`internal/compat/hooks/kernel/poll.go`) | Go netpoller; no 1:1 Linux syscall (`epoll_ctl` / `epoll_pwait` equivalents) |

### Where the counts differ from a Linux `strace`

- **Several calls share one route.**
  - `syscall.Fstat` and `unix.Fstat` both count as `Fstat`.
  - `Fsync` and `Fdatasync` both count as `Sync`, separated only by a flag (`internal/routegen/adapter_body.go:60`).
  - `writev` becomes one `Write` (`internal/compat/hooks/kernel/writev.go:17`).
- **Some calls never reach the kernel**, so they would not be counted:
  - time, sleep and timers, which the engine handles (`internal/compat/hooks/rt/time.go`);
  - futexes, semaphores and scheduling, which are engine primitives, not syscalls in the simulation;
  - `Getrlimit`, which returns `ENOSYS` inside the hook (`internal/compat/hooks/kernel/syscalls_arm64.go:66`);
  - calls that fail validation in the hook before submission, such as a nil `Utsname` returning `EFAULT`;
  - rejected operations such as raw `syscall.Syscall`, which abort the run (`internal/compat/hooks/kernel/syscalls_arm64.go:73-96`).
- **Counts are attempts.** Calls that return an errno are counted.
- **Everything translated on the machine counts.** That includes stdlib internals and test output written to stdout or stderr, not only the application's own calls.

## Proposed design

### 1. Count admitted requests

Add a counter array to the kernel's machine record, which lasts for the machine's whole life (`internal/kernel/machine.go:26`):

```go
type Machine struct {
	ref       contracts.MachineRef
	// ... existing fields ...
	syscalls  [routeLimit]uint64 // admitted application requests by route
}
```

Increment it in `DispatchOne` right after admission:

```go
func (d *Dispatcher) DispatchOne(inv *Invocation) {
	if err := d.admit(inv); err != nil {
		d.kernel.invariant("invocation admission: " + err.Error())
		return
	}
	k := d.kernel
	k.countSyscall(inv.Context.Epoch.Machine, inv.Route)
	// ... unchanged ...
}

// countSyscall records an admitted application request. Counters change only
// in supervisor dispatch order, so identical seeds reproduce them.
//
//go:norace
func (k *Kernel) countSyscall(ref contracts.MachineRef, route RouteID) {
	if m := k.machineByID[ref.ID]; m != nil && route.CountedSyscall() {
		m.syscalls[route]++
	}
}
```

A fixed array is preferred over a map:

- it does not allocate on the hot path;
- iteration order is fixed, so output is deterministic;
- it costs about 640 bytes per machine.

### 2. Generate the route classification

`RenderKernel` (`internal/routegen/render.go`) already knows each route's `kind`. It should emit two small additions to `routes_gen.go`:

```go
const routeLimit = 80 // highest RouteID + 1

// CountedSyscall reports whether a route is an application syscall or netpoll operation.
func (r RouteID) CountedSyscall() bool {
	switch r {
	case RouteRead, RouteWrite, RouteClose, /* every syscall and poll route */:
		return true
	}
	return false
}
```

Generating this keeps the classification in step with `routes.json`. A hand-written list would go stale when someone adds a route.

### 3. Add a query route

Append one route to `internal/routegen/routes.json`:

```json
{"id":79,"name":"SyscallStats","owner":"kernel","kind":"control","inputs":[{"name":"Machine","type":"contracts.MachineRef"}],"results":[{"name":"Counts","type":"[]contracts.SyscallCount"}],"legacy":[]}
```

Supporting changes:

- Define the result element type in `internal/contracts`, next to `MachineRef` (`internal/contracts/identity.go:13`):
  ```go
  SyscallCount struct {
  	Name  string
  	Count uint64
  }
  ```
- Add `"[]contracts.SyscallCount"` to `allowedType` in `internal/routegen/manifest.go:90`.
- No generator special case is needed. The default `control` branch of `renderDispatch` (`internal/routegen/dispatch.go:105-141`) renders:
  ```go
  case RouteSyscallStats:
  	p := inv.Args[0].(SyscallStatsRequest)
  	var out SyscallStatsResult
  	var err error
  	out.Counts, err = k.SyscallStats(p.Machine)
  	return controlRouteResult(out, err)
  ```
  This is the same shape as `Label` and `State`.

The result is a slice sorted by route ID, not a map:

- a route `Result` must carry copied values, never kernel-owned state;
- a slice in route order keeps the payload deterministic;
- the public wrapper converts it to a map.

### 4. Implement the kernel method

Place it next to `Label` and `MachineState` in `internal/kernel/control_runtime.go`:

```go
// SyscallStats copies a machine's nonzero request counters in route order.
func (k *Kernel) SyscallStats(ref contracts.MachineRef) ([]contracts.SyscallCount, error) {
	m, err := k.Lookup(ref)
	if err != nil {
		return nil, err
	}
	var counts []contracts.SyscallCount
	for route, count := range m.syscalls {
		if count != 0 {
			counts = append(counts, contracts.SyscallCount{Name: RouteID(route).String(), Count: count})
		}
	}
	return counts, nil
}
```

`Lookup` already rejects a foreign run (`ErrWrongRun`) and a destroyed or unknown machine (`ErrMachineNotFound`), so this inherits the same error behavior as `Label`.

### 5. Public API

Add a method to the root `gosim.Machine`, following the `fs.Stats` pattern (`fs/control_sim.go:27`). The counters belong to the kernel and need no optional feature, so they fit the root package better than `rt` or `fs`.

`machine_sim.go`:

```go
// SyscallStats reports the machine's admitted kernel requests by route name.
// Counts accumulate across restarts and are reproducible for an identical seed.
func (m Machine) SyscallStats() (map[string]uint64, error) {
	r, err := control.Call(kernel.SyscallStatsRequest{Machine: m.ref()}, kernel.DecodeSyscallStatsResult)
	if err != nil {
		return nil, err
	}
	stats := make(map[string]uint64, len(r.Counts))
	for _, c := range r.Counts {
		stats[c.Name] = c.Count
	}
	return stats, nil
}
```

`machine_native.go`:

```go
func (Machine) SyscallStats() (map[string]uint64, error) { return nil, ErrNotSim }
```

Usage in a simulation test, with the `fs` feature enabled:

```go
m, _ := gosim.NewMachine(gosim.MachineConfig{Main: func() { _, _ = os.ReadFile("/etc/hosts") }})
_ = m.Start()
_, _ = m.Wait()
stats, _ := m.SyscallStats()
// For example: stats["Openat"] == 1, stats["Fstat"] >= 1, stats["Read"] >= 1, stats["Close"] == 1.
```

A machine can read its own counters with `gosim.CurrentMachine().SyscallStats()`. The test's root machine can read any machine it created, including stopped or crashed ones.

### 6. Optional Linux-style names

If callers want `read`, `pread64` or `newfstatat` rather than `Read`, `Pread` or `Statat`, add a hand-written route-to-name table in the kernel and use it in `SyscallStats`. The table in this report is a starting point.

Keep the names approximate and documented, because one route can stand for several Linux calls (`Sync`, `Write`, `FcntlLock`). Netpoll routes would keep Gosim names. Adding a `linux` field to `routes.json` instead would change the manifest schema and its strict decoder (`DisallowUnknownFields`), which costs more for no gain.

### 7. Regenerate and accept the identity change

Changing `routes.json` changes `RouteSchemaDigest`, the generated transport files and the generation records. Regenerate with the existing target:

```sh
make go-generate
```

That target runs `profiles`, both family generations (`go125`, `go126`), `profiles` again, and the `-check` forms (`Makefile:49-57`). The route change produces a new build identity, so cached workers and artifacts are rebuilt automatically. Previous certification evidence no longer applies to the new revision. That is expected and does not block ordinary use.

## Lifetime semantics

| Choice | Behavior | Recommendation |
| --- | --- | --- |
| Per machine, cumulative | Counter lives on `Machine`. It survives `Stop`, `Crash` and `Restart`, and disappears with `Destroy`. | **Default.** Simple, and readable after `Wait`. |
| Per epoch | Counter lives on `Run` (`internal/kernel/machine.go:36`) and resets on each start. | Add later if needed, returning the epoch with the counts. |
| Explicit reset | A `ResetSyscallStats` route. | Not needed initially; callers can subtract two snapshots. |
| Run-wide total | Sum over all machines. | Easy to add as a second route if wanted. |

## Determinism, concurrency and cost

- **Deterministic.** Counters change only in the supervisor's dispatch order. The same seed and complete build identity give the same counts, so tests can assert exact values.
- **Observation is a scheduling point.** Calling `SyscallStats` is a kernel round trip like `Label` or `fs.Stats`. A program that reads the stats interleaves differently from one that does not, but both are deterministic.
- **No races.** Increments and reads both run inside `DispatchOne` on the supervisor. Application code receives a copy.
- **No self-counting.** The query is a `control` route, so it is excluded.
- **Negligible cost.** One bounds-checked array increment per admitted request.
- **Invariants preserved.**
  - No host I/O, fallback or success stub is added.
  - Native builds return `ErrNotSim`.
  - Kernel, contracts and public controls still import no concrete provider.

## Alternatives considered

1. **Aggregate `-simtrace syscall` output (no code change).** The trace already writes `call <Route>` and `ret <Route>` lines labeled with the machine (`dispatcher.go:160`, `dispatcher.go:327`). Counting them with `grep | sort | uniq -c` works for one-off analysis. The counts are not available inside test code and depend on log parsing. **Use it for ad-hoc investigation.**
2. **Count in every hook adapter.** This would give Go-API names (`syscall.Fstat` versus `unix.Fstat`) and include calls answered inside the hook (`Getrlimit`, early `EFAULT`). However, it means changing `internal/routegen/adapter_body.go` for every generated adapter on both architectures, plus the hand-written hooks (`writev.go`, `poll.go`, `unsupported.go`). It also needs counter storage reachable from application coroutines. Translated globals are per machine and are reset by `Restart`, so the counts would still need a route to read them from another machine. It changes compatibility source and its revision. **Not recommended** unless Go-API granularity is required.
3. **Count in the native engine (`rt/runtime`).** The engine does not know routes, and this would push kernel knowledge across the untranslated primitive boundary. **Rejected.**

## Tests

| Test | Location | Checks |
| --- | --- | --- |
| Kernel unit | `internal/kernel` (pattern of `dispatcher_test.go`) | Syscall and poll routes are counted per machine. Control routes, including `SyscallStats`, are not. Counts are cumulative across `Restart`. A destroyed machine returns `ErrMachineNotFound`; a foreign run returns `ErrWrongRun`. |
| Route classification | `internal/routegen` | `CountedSyscall` is true exactly for `syscall` and `poll` kinds. The existing `TestTargetServiceRouteCoverage` already requires a dispatcher case for the new route. |
| Simulation | `internal/tests/rt` (and an `fs` case under the matching feature suite) | A known workload produces expected counts, and two runs of the same seed produce identical maps. |
| Native API | `api_native_test.go:17` | Add a `"syscall-stats"` entry to `TestNativeControls`, expecting `ErrNotSim`. |

Validation follows `AGENTS.md`:

- `make go-generate`;
- focused `go test` runs of the touched packages;
- `make test`;
- `./.ci/gofmt check`.

Race and large suites are not needed and require explicit instruction.

## Change list and effort

| File | Change | Approx. lines |
| --- | --- | --- |
| `internal/routegen/routes.json` | One new route | 1 |
| `internal/routegen/manifest.go` | Allow `[]contracts.SyscallCount` | 1 |
| `internal/routegen/render.go` | Emit `routeLimit` and `CountedSyscall` | 15–25 |
| `internal/contracts/identity.go` | `SyscallCount` type | 4 |
| `internal/kernel/machine.go` | Counter array field | 1 |
| `internal/kernel/dispatcher.go` | `countSyscall` call and helper | 10 |
| `internal/kernel/control_runtime.go` | `Kernel.SyscallStats` | 15 |
| `machine_sim.go`, `machine_native.go` | Public method and native stub | 15 |
| `internal/kernel/routes_gen.go`, `dispatch_gen.go`, generation records | Regenerated | generated |
| Tests | Kernel, routegen, simulation and native | 80–150 |
| `README.md`, `docs/design.md` | Document the API and its semantics | 10–20 |

**Total:** about 100–200 hand-written production and test lines. Implementation risk is low because the counting point already exists and the query route uses the generator's standard control path.

## Decisions to make before implementing

1. **Names:** Gosim route names (`Read`, `Statat`) or approximate Linux names (`read`, `newfstatat`)?
2. **Netpoll:** count the `Poll*` routes, report them separately, or leave them out?
3. **Lifetime:** cumulative per machine (recommended), per epoch, or with an explicit reset?
4. **Errors:** add a per-route error count, or later an errno breakdown?
5. **Shape:** `map[string]uint64` (as requested) or an ordered `[]SyscallCount` in the public API as well?
6. **Scope:** also provide a run-wide total across all machines?
