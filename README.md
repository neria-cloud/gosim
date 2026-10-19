# Gosim: simulation testing for Go

Gosim runs Go tests with deterministic scheduling, simulated time, isolated machine globals, and optional filesystem and TCP network providers. Tests use normal Go code and standard-library calls; `gosim test` translates and builds them for the simulator.

A build explicitly selects one of four feature sets:

| Features | Available services |
| --- | --- |
| `rt` | Scheduler, time, channels, maps, randomness, machine lifecycle and logging |
| `rt,fs` | Runtime plus simulated files, persistence and crash recovery |
| `rt,net` | Runtime plus simulated IPv4/TCP and network faults |
| `rt,fs,net` | All three providers and their integration scenarios |

Features are fixed for the whole test binary before application initialization. Importing a control package does not enable its feature. Attempting a disabled operation aborts the simulation with a feature, operation, machine/epoch and caller diagnostic, even if application code ignores errors or recovers panics. There is no host filesystem or network fallback.

## Toolchains and certification

The initial compatibility matrix contains **Go 1.25.14 and Go 1.26.9**, on **Linux AMD64, Linux ARM64 and Darwin ARM64**, with all four feature sets and race off/on: 48 tuples. The module's Go 1.25 language minimum is separate from this exact-patch policy.

Normal commands require reviewed certification for the selected tuple **and current executable source**. A version-family directory or successful build does not certify a tuple. Status and evidence live in the [go125 profile](internal/compat/go125/profile/tuples.json) and [go126 profile](internal/compat/go126/profile/tuples.json). Pending or stale evidence is rejected. Development uses the candidate workflow below; candidate output never renews or substitutes for certification.

Build the CLI with the same exact patch used for tests:

```sh
GOTOOLCHAIN=go1.25.14 go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim
```

For another module, add `github.com/neria-cloud/gosim` as a dependency and build its CLI with that module path instead of `./cmd/gosim`. A worker/toolchain mismatch requires rebuilding with the requested patch; the CLI does not bootstrap another worker automatically.

Create `gosim.json` in the module root:

```json
{"schema":1,"toolchain":"go1.25.14","features":["rt"],"simulation_os":"linux"}
```

`-config PATH` or `GOSIM_CONFIG` selects another file. `-go` and `-features` override the corresponding values; a feature override replaces the entire list. Unknown fields, missing selections, unsupported patches/platforms and `fs`/`net` without `rt` are errors. Application source uses Linux syscall definitions; the resulting executable runs on the selected native host platform.

## A deterministic test

```go
package demo_test

import (
    "math/rand"
    "testing"
    "time"

    "github.com/neria-cloud/gosim"
)

func TestClock(t *testing.T) {
    if !gosim.IsSim() {
        t.Skip("run with gosim test")
    }
    start := time.Now()
    time.Sleep(time.Second)
    t.Log(time.Since(start), rand.Int())
}
```

On a certified revision:

```sh
./.gosim/gosimtool test -go go1.25.14 -features rt -v -seeds 1-3 -run TestClock .
```

Repeating a seed with the same complete build identity and replay revision reproduces the execution. Toolchain, architecture, source, feature, race or approved build-policy changes define another identity. Run IDs protect handles from stale runs and are excluded from deterministic seed/trace derivation. Scheduler, application, filesystem and network randomness use separate owned streams.

## Machines and optional controls

`NewMachine` returns an **unstarted** machine. Configure its optional services, then call `Start`. Returning from its main function gracefully stops that epoch and its other goroutines.

```go
package demo_test

import (
    "fmt"
    "net/netip"
    "testing"

    "github.com/neria-cloud/gosim"
    simfs "github.com/neria-cloud/gosim/fs"
    simnet "github.com/neria-cloud/gosim/net"
)

func TestMachine(t *testing.T) {
    m, err := gosim.NewMachine(gosim.MachineConfig{
        Label: "node-a",
        Main: func() { fmt.Println("node-a started") },
    })
    if err != nil { t.Fatal(err) }
    if err := simfs.ConfigureMachine(m, simfs.DefaultConfig()); err != nil { t.Fatal(err) }
    if err := simnet.ConfigureMachine(m, simnet.MachineConfig{
        Addr: netip.MustParseAddr("10.0.0.1"),
    }); err != nil { t.Fatal(err) }
    if err := m.Start(); err != nil { t.Fatal(err) }
    exit, err := m.Wait()
    if err != nil { t.Fatal(err) }
    if exit.Kind != gosim.Graceful { t.Fatalf("unexpected exit: %+v", exit) }
    if err := m.Destroy(); err != nil { t.Fatal(err) }
}
```

Run this example with `-features rt,fs,net`. `Stop` is graceful; `Crash` retains the provider's crash outcome. `Restart` accepts a previously started, stopped/crashed machine and initializes fresh globals. `SetMain` changes its next boot program. `Wait` observes the epoch active when called, even if the machine restarts later. Destroy invalidates the handle. Machine termination cancels its goroutines without running their pending defers. The controller/test root cannot be explicitly stopped or destroyed.

Returning self-targeted `Stop`, `Crash` and `Wait` calls report `ErrSelfTransition`; `gosim.ExitSelf()` and `gosim.CrashSelf()` are the non-returning alternatives. Native error-returning controls return `gosim.ErrNotSim`; native `IsSim`, `CurrentMachine` and `rt.Info` return false/zero values. Pure defaults and handle ID accessors remain usable.

## Filesystem recovery

The filesystem starts at `/app`. Configure it before startup with `fs.ConfigureMachine`. `fs.SetCrashMode` selects `DurableOnly` or `SamplePending`; `fs.SetSyncFault` configures bounded sync-failure probabilities.

After a crash, `fs.CrashStates(machine, fs.EnumerateOptions{MaxStates: n})` owns a synchronous iterator over immutable crash outcomes. A positive budget is required. `Next` returns a snapshot and availability flag; exceeding the budget while more outcomes exist returns `fs.ErrEnumerationLimit`. Close the iterator when finished, and close each returned snapshot separately. Closing the iterator does not invalidate snapshots already returned. `fs.Restore` accepts a retained snapshot from the same simulation for an unstarted or stopped/crashed target; configure its next main function and start/restart it explicitly. Destroying a machine does not invalidate separately owned snapshots.

Graceful stop flushes modeled writes. Crash behavior preserves the existing persistence model, including uncertainty in non-fsynced operations; it does not add missing POSIX locking or permission semantics.

## Network policy

Use `net.DefaultConfig` and `net.Configure` for global latency/loss defaults. `net.ConfigureMachine` assigns an IPv4 address before the first start; a zero address requests deterministic allocation. `net.Address` returns `ErrNotStarted` before allocation, and restart retains the assigned address.

A `net.Link` selects `From`, `To`, and `Forward` or `Both` directions. `SetLinkPolicy` copies each non-nil latency/loss group; nil leaves that override unchanged. `ResetLinkPolicy` restores global defaults. For fixed latency, set `LatencyConfig{Min: delay, Max: delay, Lambda: 1}`. Values are validated atomically; lambda must be positive and probabilities must lie in `[0,1]`.

`Partition`/`Repair` control explicit connectivity separately from sampled loss state. Fault transitions are traffic-driven; packets retain deterministic delivery order, destination epochs and connection generations. Old packets cannot reach a restarted endpoint or a reused connection. Graceful close may deliver a final immutable packet to a still-live peer; crash does not send a close notification.

## CLI, metatesting and diagnostics

All build commands share configuration and identity validation:

```sh
./.gosim/gosimtool translate -go go1.25.14 -features rt ./package
./.gosim/gosimtool build-tests -go go1.25.14 -features rt ./package
./.gosim/gosimtool test -go go1.25.14 -features rt -race -seeds 1-20 ./package
./.gosim/gosimtool debug -go go1.25.14 -features rt -package ./package -test TestName -seed 1 -step 11
```

Debug requires Delve and supports `-headless`. Logging includes machine, goroutine and step information. `rt.SetTimeout` sets the simulation timeout; `rt.Info` reports the actual build identity and certification state. `-simtrace syscall` enables syscall tracing.

[Metatesting](metatesting/) runs prebuilt simulation workers from native Go tests. Prepare application workers with `build-tests` using the matching patch, features and race mode. `prepare-selftest` additionally prepares this repository’s own native test infrastructure and is run from the Gosim checkout. The runner checks the manifest, contents and versioned handshake before executing. A mismatch reports a rebuild command; it cannot reuse another feature set or a candidate artifact as certified output.

Artifacts are stored per identity **and package content** at `.gosim/v2/<identity>/packages/<package-digest>/`. Manifests hash the binary and generated files. The SQLite translation cache uses the v2 namespace and treats corrupt entries as observable misses; old cache formats are not decoded.

## Current scope

| Area | Current boundary |
| --- | --- |
| Files and persistence | Existing file/directory, sync and crash semantics; limits remain in mappings and syscall options |
| Locks and permissions | `flock`/record-lock stubs do not establish ownership or contention; chmod/permission completeness is not claimed |
| Networking | Simulated IPv4/TCP; UDP, IPv6 and hostname resolution are outside the supported model |
| HTTP/gRPC | Unmodified application stacks are explicitly rejected; the [etcd example](examples/etcd/) demonstrates this boundary |
| Linux process services | Deterministic getrandom, historical fixed PID and limited uname; full signals/procfs/process behavior is absent |
| Runtime modes | One active run per process; no native-scheduler virtual-I/O mode, mutable feature sets or runtime provider loading |
| Additional platforms | Other Go patches, Darwin AMD64, Windows and non-Linux simulated ABIs are not in the initial matrix |

The reviewed [go125](internal/compat/go125/profile/capabilities.json) and [go126](internal/compat/go126/profile/capabilities.json) capability records distinguish implemented, limited, stub and rejected behavior. The historical [syscall audit](docs/reports/syscall-support-for-gquorum.md) records remaining application requirements; modularization does not claim to implement them.

## Development and certification

Run ordinary native and simulation checks on the local host with `make test`.
It uses the pinned Go 1.26.9 toolchain by default and does not start Docker,
Colima, VMs, cross-architecture jobs, or the full race suite. Independent tests
run in parallel with two package processes and two parallel tests per package.
Use `make test UNIT_PACKAGES=2 UNIT_PARALLEL=4` to adjust these limits or
`make test GOSIM_TOOLCHAIN=go1.25.14` to select the other pinned patch.

Reports are written to `test-reports/unit/<run>/`, with `test-reports/unit/latest/`
pointing to the latest run. Each run includes `report.md`, a test timeline,
JSON events, full logs and per-package JUnit XML. `TEST_REPORTS_DIR` selects the
report directory. Use `make test-race` for the separate local race pass at final
verification; its reports are under `test-reports/unit-race/`.

`make test-large` opts into the expensive candidate feature sweep, selected
translation toolchain sweep, and CLI script fixtures. It runs only those suites,
with their workers launched by the parent tests. They remain skipped in ordinary
and race runs. Large runs use the same reporting format under
`test-reports/large/<run>/` and `test-reports/large/latest/`, with the same
`UNIT_PARALLEL`, `UNIT_PACKAGES`, and `TEST_REPORTS_DIR` settings. No VMs or
cross-architecture jobs are started.

Local reports describe the isolated source snapshot taken at startup. Edits to
the working checkout during a run are listed in the command log and verification
record; they do not invalidate results for that snapshot. Changes inside the
tested copy still fail verification. The standalone verification entry below
also rejects changes to the working checkout.

The underlying verification entry also accepts an exact patch:

```sh
GOSIM_TOOLCHAIN=go1.25.14 ./.ci/verify-step.sh --ordinary
GOSIM_TOOLCHAIN=go1.26.9 ./.ci/verify-step.sh --ordinary
```

`./test.sh` runs the complete ordinary/race gate (default patch `go1.26.9`). Reserve the full race pass for final verification during this implementation plan. Verification makes a private copy, checks identical executable inputs, and marks only that copy's existing tuples pending. The original checkout's status/evidence stays unchanged. This also supports testing source edits after a release without weakening the normal CLI or the pending-only candidate selector. Independent copies allow both patches to run concurrently.

On an already pending checkout, focused simulation checks can use the matching CLI's `internal-candidate test` command. Those builds are visibly candidate output. The internal `certify` command runs one tuple and records its suites; it never promotes support automatically.

The final matrix runner verifies observed architectures, pins Docker image digests, retains logs and artifacts, and shares the available CPU and memory budget between concurrent tuples and their test workers. `--jobs` caps the tuple count:

```sh
./.ci/crossarch-tests/run-matrix.sh --platform all --dry-run
./.ci/crossarch-tests/run-matrix.sh --native --platform darwin/arm64 --toolchains go1.25.14,go1.26.9 --jobs 2 --output .gosim/certification/darwin-arm64
./.ci/crossarch-tests/run-matrix.sh --context colima --platform linux/amd64 --toolchains go1.25.14,go1.26.9 --output .gosim/certification/linux-amd64
./.ci/crossarch-tests/run-matrix.sh --context colima --platform linux/arm64 --toolchains go1.25.14,go1.26.9 --output .gosim/certification/linux-arm64
```

Each platform expands to 16 tuples. `--smoke` executes one ordinary rt tuple and is not full certification. Colima routes require a Darwin ARM64 host; Linux CI uses `--native` on its actual architecture. On this workstation, the existing ARM64 VZ VM uses Rosetta for AMD64 execution, recorded as `container-emulated`. QEMU failed the pinned compiler smoke test. The runner does not recreate VMs or change their configuration.

Generated adapters use `cmd/gosim-gen` with explicit toolchain/profile/architecture flags; `task go-generate` runs both families and the filesystem model's stringer. Run `python3 -B -m unittest discover -s .ci/crossarch-tests -p '*_test.py'` for the matrix orchestration checks. Promotion requires reviewed successful reports for every required tuple against the final source; later source changes invalidate that evidence.

See [the architecture](docs/design.md), [examples](examples/), and [nemesis](nemesis/) for implementation details and supported scenarios.

## License

[MIT](LICENSE). neria-cloud/gosim is a fork of [jellevandenhooff/gosim](https://github.com/jellevandenhooff/gosim).
