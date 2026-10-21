# Gosim: simulation testing for Go

Gosim is a simulation testing framework that aims to make it easier to write reliable distributed systems code in go. Gosim takes standard go programs and tests and runs them with a simulated network, filesystem, multiple machines, and more.

In Gosim's simulated environment you can run an entire distributed system in a single go process, without having to set up any real servers. The simulation can also introduce all kinds of problems, such as flaky networks, restarting machines, or lost writes on a filesystem, to test that the program survive those problems before they happen in real life.

Features are fixed for the whole test binary before application initialization. Importing a control package does not enable its feature. Attempting a disabled operation aborts the simulation with a feature, operation, machine/epoch and caller diagnostic, even if application code ignores errors or recovers panics. There is no host filesystem or network fallback.

![Architecture](./docs/diagrams/architecture.svg)

![Test workflows](./docs/diagrams/test-workflows.svg)

**Status of the project:** stable core API and ready for regualar CI tasks. **gosim** is used in several private projects and in [gquorum](https://github.com/asalimonov/gquorum) library.

See [Features](docs/features.md) for supported, partially supported, and unsupported capabilities. PRs welcome.

## Technicla introduction

Gosim supports stable **Go 1.25.* and Go 1.26.*** releases on Linux AMD64,
Linux ARM64 and Darwin ARM64. The caller uses ordinary Go tooling: `go.mod`,
`go.work`, and `GOTOOLCHAIN` determine the compiler. Application code does not
select a Go version. Gosim checks the private runtime contracts it uses against the
selected compiler; an incompatible contract produces an actionable error.

Here is [design](./docs/design.md) and [translation](./docs/translation.md) document
which describe gosim's internals with more details.

Add Gosim to your module, then build the launcher in that module's context:

```sh
go get github.com/neria-cloud/gosim
go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool github.com/neria-cloud/gosim/cmd/gosim
```

In this repository, `make build` builds `.gosim/gosimtool`, `.gosim/gosim-worker`,
and `.gosim/gosim-gen`. It uses Go's normal compiler selection; an optional
`GOSIM_TOOLCHAIN` override selects an exact compiler for the build. The launcher
selects the caller's Gosim dependency, including workspace and local
replacements, and automatically builds/caches a worker with the compiler Go
selected. Switching supported Go patches does not require rebuilding the
launcher. Help and version commands work without worker bootstrap.

Use `-C directory` before the command to select another working directory,
including its module, Go toolchain, and configuration. Relative package and
configuration paths are resolved there; your shell stays in its current directory:

```sh
./.gosim/gosimtool -C examples/network/nats test -features rt,fs,net -v .
```

An optional `gosim.json` selects features, without a compiler pin:

```json
{"schema":2,"features":["rt"],"simulation_os":"linux"}
```

Without a config file, all three features are enabled and simulation uses the
Linux source ABI. `-config PATH` or `GOSIM_CONFIG` selects another file;
`-features` replaces the feature list. `-go` is an optional explicit compiler
override. Legacy schema-1 files remain explicit overrides with a migration
warning; remove their `toolchain` field and set `schema` to `2` for automatic
selection. Unknown fields, unsupported families/platforms and `fs`/`net`
without `rt` are errors. Prerelease and development compilers are unsupported.

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

Run it with:

```sh
./.gosim/gosimtool test -features rt -v -seeds 1-3 -run TestClock .
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

The in-memory filesystem is rooted at `/`; processes start in `/app`. Configure each machine before its first start with `fs.ConfigureMachine`. Capacity defaults to 128 MiB and must be a multiple of 1 KiB. `fs.Stats` reports total, used and free bytes in allocated 1 KiB file-data blocks, plus block size and read-only status. Sparse holes consume no blocks; `/etc/hosts` initially consumes one. A write or `fallocate` reservation that would exceed capacity fails atomically with `ENOSPC`. Capacity cannot be resized after configuration. A restored image above the target capacity remains readable, reports zero free space and rejects new allocation until usage falls.

`fs.Config` also sets the machine's effective UID/GID (root/root by default) and a fixed per-operation `Latency`. Files and directories store mode and ownership; creation applies a fixed `0022` umask. Owner/group/other checks cover file opens, directory traversal and mutations; chmod requires the owner or root, and chown requires root. `fs.SetReadOnly(machine, true)` immediately rejects data and metadata mutation with `EROFS`, including writes through existing descriptors. Set it to `false` to resume writes. The policy survives stop, crash, restart and snapshot restore. Configured latency delays completion of data I/O and mutating filesystem calls, including `ENOSPC` and `EROFS`; metadata reads such as stat and statfs remain immediate.

`fs.SetCrashMode` selects `DurableOnly` or `SamplePending`; `fs.SetSyncFault` configures bounded sync-failure probabilities.

After a crash, `fs.CrashStates(machine, fs.EnumerateOptions{MaxStates: n})` owns a synchronous iterator over immutable crash outcomes. A positive budget is required. `Next` returns a snapshot and availability flag; exceeding the budget while more outcomes exist returns `fs.ErrEnumerationLimit`. Close the iterator when finished, and close each returned snapshot separately. Closing the iterator does not invalidate snapshots already returned. `fs.Restore` accepts a retained snapshot from the same simulation for an unstarted or stopped/crashed target; configure its next main function and start/restart it explicitly. Destroying a machine does not invalidate separately owned snapshots.

Graceful stop flushes modeled writes and metadata. Crash behavior preserves uncertainty in non-fsynced operations, including block reservations and mode/ownership changes. Supplemental groups, ACLs, sticky bits, symlinks, special files and real POSIX locking remain outside the model.

## Network policy

Use `net.DefaultConfig` and `net.Configure` for global latency/loss defaults. `net.ConfigureMachine` assigns an IPv4 address before the first start; a zero address requests deterministic allocation. `net.Address` returns `ErrNotStarted` before allocation, and restart retains the assigned address.

`net.Config.ConnectionTimeout` sets the connection-opening timeout for new
attempts. It defaults to one second; zero also selects that default, and negative
values are invalid. Reconfiguration leaves pending attempts and established
connections unchanged. A shorter `net.DialTimeout` or dialer context deadline
still takes precedence. For high-latency links, configure a timeout longer than
the expected handshake round trip:

```go
config := simnet.DefaultConfig()
config.ConnectionTimeout = 10 * time.Second
config.TCPBufferSize = 256 * 1024
if err := simnet.Configure(config); err != nil {
    t.Fatal(err)
}
```

`net.Config.TCPBufferSize` sets each direction’s byte capacity for new TCP
connections. Zero uses the historical 1024-byte default; positive values up to
16 MiB are accepted. A larger window avoids an unintended throughput bottleneck
on high-latency links. Reconfiguration leaves pending and established connections
unchanged.

A `net.Link` selects `From`, `To`, and `Forward` or `Both` directions. `SetLinkPolicy` copies each non-nil latency/loss group; nil leaves that override unchanged. `ResetLinkPolicy` restores global defaults. For fixed latency, set `LatencyConfig{Min: delay, Max: delay, Lambda: 1}`. Values are validated atomically; lambda must be positive and probabilities must lie in `[0,1]`.

`Partition`/`Repair` control explicit connectivity separately from sampled loss state. Fault transitions are traffic-driven; packets retain deterministic delivery order, destination epochs and connection generations. Old packets cannot reach a restarted endpoint or a reused connection. Graceful close may deliver a final immutable packet to a still-live peer; crash does not send a close notification.

## CLI, metatesting and diagnostics

All build commands share configuration and identity validation:

```sh
./.gosim/gosimtool translate -features rt ./package
./.gosim/gosimtool build-tests -features rt ./package
./.gosim/gosimtool test -features rt -race -seeds 1-20 ./package
./.gosim/gosimtool debug -features rt -package ./package -test TestName -seed 1 -step 11
```

Debug requires Delve and supports `-headless`. Logging includes machine, goroutine and step information. `rt.SetTimeout` sets the simulation timeout; `rt.Info` reports the actual build identity. Its deprecated `Certified` field is false; manual reports remain separate. `-simtrace syscall` enables syscall tracing.

[Metatesting](metatesting/) runs prebuilt simulation workers from native Go tests. Prepare application workers with `build-tests` using the matching patch, features and race mode. `prepare-selftest` additionally prepares this repository’s own native test infrastructure and is run from the Gosim checkout. The runner checks the manifest, contents and versioned handshake before executing. A mismatch reports a rebuild command; it cannot reuse another feature set or stale source inputs. Native metatests automatically select a matching CLI worker from the Go test process’s compiler. An explicit `GOSIMTOOL` override requires that worker’s matching manifest; unset stale overrides to restore automatic selection.

Artifacts are stored per identity **and package content** at `.gosim/v2/<identity>/packages/<package-digest>/`. Manifests hash the binary and generated files. The separate CLI worker cache uses `workers/v1/<key>/` under the selected cache root, keyed by exact compiler, Gosim source and worker dependency content. The SQLite translation cache uses the v2 namespace and treats corrupt entries as observable misses; old cache formats are not decoded.

## Current scope

| Area | Current boundary |
| --- | --- |
| Files and persistence | Bounded in-memory file/directory volume, 1 KiB allocated-block accounting, sync/crash semantics; no live resize or special files |
| Locks and permissions | Owner/group/other mode and ownership checks; no supplemental groups, ACLs or sticky bits; `flock`/record-lock stubs do not establish ownership or contention |
| Networking | Simulated IPv4/TCP; UDP, IPv6 and hostname resolution are outside the supported model |
| HTTP/gRPC/TLS | Translated plaintext HTTP request/response, connection reuse, streaming, cancellation and graceful shutdown have focused coverage. The [three-node etcd example](examples/etcd/) exercises HTTP/gRPC over simulated TCP and storage, including partition recovery and snapshot/WAL round trips. The [standalone gRPC example](examples/network/grpc/) covers unary and bidirectional streams; the [NATS example](examples/network/nats/) covers plain TCP. TLS/certificate operations abort; untested HTTP surfaces are not claimed. |
| Linux process services | Deterministic getrandom, historical fixed PID and limited uname; full signals/procfs/process behavior is absent |
| Runtime modes | One active run per process; no native-scheduler virtual-I/O mode, mutable feature sets or runtime provider loading |
| Additional platforms | Go families other than 1.25/1.26, Darwin AMD64, Windows and non-Linux simulated ABIs are unsupported |

The reviewed [Go 1.25 baseline](internal/compat/profiles/go125.json) distinguishes implemented, limited, stub and rejected behavior. Effective Go 1.26 capabilities combine that baseline with the [Go 1.26 delta](internal/compat/profiles/go126.json). The historical [syscall audit](docs/reports/syscall-support-for-gquorum.md) records remaining application requirements; modularization does not claim to implement them.

## Development and test reports

```sh
make test
make test UNIT_PACKAGES=2 UNIT_PARALLEL=4
make test GOSIM_TOOLCHAIN=go1.26.9
```

Run `make test-race` and `make test-large` only when explicitly instructed to run
and check those suites. They are separate opt-in commands. `make test-large`
runs the candidate feature cases, selected-compiler translation checks and CLI
script fixtures; it does not repeat other toolchains or silently enable race
instrumentation. `make test-report-tools` checks report generation without
running the library suites.

| Command | Latest report directory |
| --- | --- |
| `make test` | `test-reports/unit/latest/` |
| `make test-race` | `test-reports/unit-race/latest/` |
| `make test-large` | `test-reports/large/latest/` |

Each run replaces the previous results in `test-reports/<suite>/latest/`. Read `report.md` for
pass/fail/error/skip totals, `timeline.md` or `timeline.json` for duration and
parallel execution, and per-package `.log` files for failure details. Reports
also retain raw events, JSON, JUnit XML and command logs. `TEST_REPORTS_DIR`
changes the report root. Skips are reported separately from passes.

Results describe the startup source snapshot. Working-checkout edits during a
reported run are recorded without invalidating that snapshot; mutations inside
the tested copy fail verification. Snapshot details live under
`.gosim/verification/<patch>/<mode>/verification.json`. The standalone
`./.ci/verify-step.sh --ordinary` also rejects checkout changes. `./test.sh`
defaults to ordinary verification; `./test.sh --race` explicitly selects race.
Neither path invokes certification or rewrites profile evidence.

CI runs ordinary native tests and reporting checks on ARM64 Linux with
Go family selectors, and uploads `test-reports`. It has no certification step.

Certification is an optional manual operation. It is neither a CI step nor a
requirement for normal commands or library use. Missing or stale certification
never blocks a compatible build. Exact compiler/source identities still isolate
artifacts and deterministic replay; a family policy does not promise replay
across different patches.

## Maintenance, generation and manual certification

See the [maintainer guide](docs/MAINTAINER_GUIDE.md) for adding Go families,
version-specific APIs, stdlib hooks and simulation features, and verifying
certification results.

Compatibility implementation lives once in `internal/compat/{hooks,shims,host}`.
`internal/compat/profiles/go125.json` is the baseline; `go126.json` contains only
its changes. A future 1.27 definition would inherit **1.25 + 1.26 + 1.27**.
An unchanged family needs an empty delta and a support declaration, with no copied
implementation files. Generated selectors and guards enforce supported families;
ordinary Go release constraints select source variants. Exact patches remain
separate compiler/source identities, not support declarations.

### Reviewing a new Go family

Run the metadata-only generator with each compiler in a separate process. These
commands inspect selected source; they do not execute race or cross-architecture
tests. For the existing 1.25→1.26 transition:

```sh
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen observe -profile=go125 -output=.gosim/compare/go125-observation.json
GOTOOLCHAIN=go1.26.9 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen compare -parent=go125 -parent-observation=.gosim/compare/go125-observation.json -output=.gosim/compare/go126
```

Use fresh destinations on every run. The tool creates missing parent directories
and rejects an existing destination before scanning; final publication is also
exclusive. The destination filesystem must support hard links. `observe` requires
the parent's recorded exact generation compiler. `compare` uses the running stable
compiler, including an unregistered family, and requires the immediate next minor.
Neither command downloads or runs a parent compiler. Parent snapshots bind the
host/context inventory, inspector implementation, parent definition/origin and
module inputs; use the same host and requested architectures for both runs.

With a snapshot, `api_diff=inspected` reports actual upstream API and method-set
changes. Omitting `-parent-observation` gives `skipped-no-parent`: declared contract
and source checks still run, but API comparison is skipped. Without a parent
snapshot, unreviewed packages are informational; the tool cannot determine which
packages are newly added. Failed inspections produce incomplete reports and exit 1;
complete differences, including incompatibilities, exit 0. Invalid arguments exit 2. Race contexts describe source
selection only. Source hash changes require behavioral review.

Outputs are `report.md`, `delta.draft.json`, then `report.json` last. The Markdown
groups repeated findings and places incompatible changes before review/information.
An absent `report.json` means interrupted publication; `complete=false` means an
incomplete scan. A draft cannot be loaded as a family definition and never enables
support. Review findings, implement any necessary source variants, and author the
new family's delta against its immediate parent. No automatic apply, baseline
rebase or semantic-compatibility proof is provided.

### Maintaining reviewed definitions and provenance

`profiles` is the sole support-table, guard and adapter-revision maintenance entry:

```sh
go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen profiles -root=.
go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen profiles -root=. -check
```

Write mode can bootstrap a reviewed definition before its origin exists. With the
matching compiler, run flag-only generation with `-profile`, `-arches`, `-output=.`
and explicit `-refresh-origin` to create/update independent provenance after
contract validation. Then regenerate `profiles` and run metadata and generation
checks under both recorded baselines. Existing checks:

```sh
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=. -check
GOTOOLCHAIN=go1.26.9 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=. -check
```

Generation baselines are reproducible provenance, not caller compiler pins.
`make go-generate` regenerates shared adapters, metadata and the filesystem-model
stringer output, then checks shared adapters independently under both compilers.
Origins remain separate files; historical evidence remains in `profiles/evidence.json`.
Neither is an ordinary support allowlist. See [design and storage accounting](docs/design.md#compatibility-storage-accounting).

Manual certification is independent of tests and CI. Its matrix defaults to
Go 1.25.14 and Go 1.26.9 on Linux/Darwin ARM64, with four feature sets and no race
instrumentation (16 tuples). `--toolchains` can select other stable supported
patches; these defaults are not an admission list. AMD64 and race certification
remain disabled at scheduler, worker and command boundaries.

An explicitly requested manual run uses `run-matrix.sh --native --platform
OS/arm64`, or `--context colima --platform linux/arm64` from Darwin ARM64. It
requires complete selected-tuple large coverage and records exact source,
compiler, observed architecture, image metadata and logs. Such a report never
changes library usability or promotes a pending tuple. No normal command
invokes this matrix.

Native matrix jobs reuse Go's content-addressed build cache across runs under
`.gosim/certification/go-build-cache/` (or the caller's `GOCACHE`). Test results
remain uncached. Each matrix replaces `.gosim/certification/matrix/`; each tuple
uses a private source snapshot that is deleted when it finishes. Logs and reports
remain until the next run. Output locks reject overlapping matrices using the same
output directory. Local verification also removes its snapshot after completion.
`gosim translate` replaces `.gosim/translated/`. Build staging directories are
temporary and cleaned up; compiler/source identities still separate cached artifacts.
The runner prints tuple and total durations and links failed suite logs.

See [the architecture](docs/design.md), [examples](examples/), and [nemesis](nemesis/) for implementation details and supported scenarios.

## License

[MIT](LICENSE). neria-cloud/gosim is a fork of [jellevandenhooff/gosim](https://github.com/jellevandenhooff/gosim).
