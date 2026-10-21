# Implementation plan: per-machine syscall monitor

**Plan:** `docs/plans/20261021-syscall-counters.md`

**Project source:** `.` (repository root; no `project.txt`)

**Target architecture:** [docs/brainstorms/20261021-syscall-counters-target.md](../brainstorms/20261021-syscall-counters-target.md), revision 2

**Full brainstorm:** [docs/brainstorms/20261021-syscall-counters.md](../brainstorms/20261021-syscall-counters.md)

**Date:** 2026-10-21

**Source baseline:** `20d517e`, branch `feat/syscall-counter`

**Status:** planned; no implementation, generation, or execution validation performed.

## Outcome and constraints

Expose `machine.SyscallsMonitor()` with Start, Stop, Reset, Counts, Stats, and
List. Counts returns syscall-name-to-count pairs. Stats adds a fixed syscall
group and descriptor-owner histogram. Only observed, nonzero names and buckets
appear in snapshots; List independently returns every available countable name.

The accepted design is revision 2, including the user's per-machine ownership,
fixed groups, routed-attempt boundary, restart-persistent lifecycle, and
owner-group-only descriptor information. Revision 1's standalone Machine methods
and package-level discovery function are superseded. The [peer review](../review/feat_syscall-counter.md)
applied to revision 1; its dependency-order and generator corrections are already
incorporated in revision 2. This plan does not claim a separate review of revision 2.

Architecture sources are [docs/design.md](../design.md), [README.md](../../README.md),
[AGENTS.md](../../AGENTS.md), and the current source. There is no `architecture/`
or `docs/architecture/` directory. The [gquorum report](../reports/syscall-support-for-gquorum.md)
is historical motivation, not a current route inventory or a request to implement
missing Linux semantics.

Documentation scope, updated 2026-10-22: create a brief `docs/syscalls.md` during
the final documentation/validation stage (Task 5), after the behavior is
implemented. Keep `README.md` and `docs/design.md` unchanged. Retain the small
feature-matrix correction in `docs/features.md`, linking to the new topic page.

Counting lives on kernel Machine, outside its replaceable Run/epoch. The capture
point is after admission and exact typed-payload validation, before execution
mutates descriptors. Native calls, hook-local early returns, polling, controls,
and provider-internal work are outside coverage. Failed modeled attempts and
retries count. Completion never counts again.

The existing target §6 pipeline is retained:

```text
admitted typed request -> validate -> canonical name + input FD owner
    -> machine total and owner bucket -> copied normalized nonzero rows
    -> Counts totals / Stats totals and histograms
single canonical catalogue -> available metadata -> sorted List
```

The normalized row preserves the owner dimension for future selection before
public aggregation. No CEL dependency, filter API, predicate interface, parser,
evaluator, callback, or expression cache is part of this implementation.

## Execution rules

- Complete tasks in order. Task 1 deliberately combines the mutually dependent
  contract, schema, generator, and real kernel implementation. A catalogue using
  RouteWritev/routeLimit cannot compile before those symbols are generated. Do
  not split that dependency across commits or add successful temporary stubs.
- Estimated authored changes per task remain below roughly 1,500 lines; generated
  output is additional. Prefer existing fixtures and table-driven behavioral
  checks over duplicating the implementation in tests.
- Use `cmd/gosim-gen` through `make go-generate`; never hand-edit generated files.
  The generator can run without loading `internal/kernel`, so it can resolve
  Task 1's temporary missing generated symbols before the first compilation.
- Keep kernel/contracts/public APIs independent of concrete providers, and keep
  contracts independent of kernel/public APIs. Preserve the translated/native
  boundary, supervisor ownership, and existing `//go:norace` conventions.
- Preserve all existing route IDs. At this baseline the maximum is 78; append
  79–83. If the source baseline changes before execution, reconcile the actual
  highest ID and catalogue deliberately and record the change here.
- Each task ends with a build or executable tests relevant to its changes.
  Fix every failure in the checks actually run, including unrelated failures,
  before marking the task complete. Log incidental fixes under NOT PLANNED
  CHANGES. Do not bypass repository restrictions to broaden test scope.
- Race and large suites, certification, Docker/VM jobs, and cross-architecture
  execution are excluded. Generating adapters for both source architectures is
  generation, not cross-architecture execution evidence.
- Record commands, exit status, real test names/counts, skips, source identity,
  and report paths in the progress log. A selector running no tests is not a pass.
  Check boxes only after implementation and its verification succeed.
- Docs are a normal directory in this worktree, not a symlink or separate Git
  repository. Planning changes belong to the current project branch. Preserve
  existing unrelated files and do not stage the existing review directory.

## Task list

### Task 1: Implement the typed routes and machine-owned counters together

**Depends on:** none. **Estimated authored change:** 1,000–1,450 lines including tests.

**Files:** new `internal/contracts/syscall_stats.go`,
`internal/kernel/syscall_stats.go`, `internal/kernel/syscall_stats_test.go`,
`internal/kernel/syscall_catalogue_test.go`; update `internal/kernel/machine.go`,
`internal/kernel/routes_generated_test.go`, `internal/kernel/dispatcher_test.go`,
`internal/routegen/routes.json`, `internal/routegen/manifest.go`,
`internal/routegen/render.go`, `internal/routegen/dispatch.go`,
`internal/routegen/generate_test.go`; generated `internal/kernel/routes_gen.go`,
`internal/kernel/dispatch_gen.go` and identity outputs produced by generation.
Reuse `lifecycle_test.go`, `lifecycle_fakes_test.go`, and `io_fakes_test.go` fixtures.

**Target references:** §3 lifecycle; §4 primary descriptor rule; §5 complete
catalogue; §6 Pipeline/Internal types/Future expression data contract; §7 Kernel
and transport data/Route manifest and generation; §8 Collection and failures;
§9 first buildable slice; §11 `syscallCounterSlot`, `syscallInfo`,
`descriptorGroupName`, `syscallRouteHasFD`, `syscallInputFD`,
`Kernel.observeSyscall`, `Kernel.countSyscall`, the four kernel control methods,
and `Dispatcher.decode`. Brainstorm: Revision 2 descriptor interpretation,
internal records, and review corrections.

Contract declarations must match:

```go
type DescriptorGroup uint8

const (
    DescriptorNone DescriptorGroup = iota
    DescriptorRT
    DescriptorFS
    DescriptorNet
    DescriptorUnknown
    DescriptorGroupLimit
)

type SyscallRecord struct {
    Name            string
    Group           string
    DescriptorBased bool
    DescriptorGroup string
    Count           uint64
}

type SyscallInfo struct {
    Name            string
    Group           string
    DescriptorBased bool
}
```

Kernel storage must match target §6:

```go
type syscallDescriptor struct {
    name  string
    group contracts.Feature
}

const syscallDataSyncSlot RouteID = routeLimit // not a dispatchable route

// Populate the complete target §5 catalogue, keyed by canonical route/extra slot.
var syscallCatalogue = [...]syscallDescriptor{
    // Full initializer required; FcntlLock shares RouteFcntlInt's populated slot.
}

type syscallObservation struct {
    slot            RouteID
    descriptorGroup contracts.DescriptorGroup
}
type syscallCounter struct {
    total        uint64
    byDescriptor [contracts.DescriptorGroupLimit]uint64
}
type syscallCounters struct {
    enabled bool
    values  [len(syscallCatalogue)]syscallCounter
}
```

The initializer above is a shape illustration; use every entry in target §5,
including RouteSync for fsync and syscallDataSyncSlot for fdatasync. Add
`syscalls syscallCounters` to kernel Machine only. The proposed IDs yield 85
slots and 4,080 bytes of counter values per machine, plus flag/alignment.

Implement these signatures without changing existing service/provider interfaces:

```go
func syscallCounterSlot(inv *Invocation) RouteID
func syscallInfo(slot RouteID) contracts.SyscallInfo
func descriptorGroupName(group contracts.DescriptorGroup) string
func (k *Kernel) observeSyscall(inv *Invocation) (syscallObservation, bool)
func (k *Kernel) countSyscall(inv *Invocation)
func (k *Kernel) SetSyscallCounting(ref contracts.MachineRef, enabled bool) error
func (k *Kernel) ResetSyscallStats(ref contracts.MachineRef) error
func (k *Kernel) SyscallStats(ref contracts.MachineRef) ([]contracts.SyscallRecord, error)
func (k *Kernel) CountableSyscalls(ref contracts.MachineRef) ([]contracts.SyscallInfo, error)

// Generated from the manifest:
func syscallRouteHasFD(route RouteID) bool
func syscallInputFD(inv *Invocation) (int, bool)
```

Append these manifest entries with empty legacy lists:

| ID | Route | Owner/kind | Request fields | Result fields |
| --- | --- | --- | --- | --- |
| 79 | Writev | kernel/syscall | FD int; From abi.ByteView | Count int |
| 80 | SetSyscallCounting | kernel/control | Machine contracts.MachineRef; Enabled bool | none |
| 81 | ResetSyscallStats | kernel/control | Machine contracts.MachineRef | none |
| 82 | SyscallStats | kernel/control | Machine contracts.MachineRef | Records []contracts.SyscallRecord |
| 83 | CountableSyscalls | kernel/control | Machine contracts.MachineRef | Syscalls []contracts.SyscallInfo |

For each new route X, generate XRequest/XResult, `func (p XRequest) Invocation()
Invocation`, and `func DecodeXResult(r Result) (XResult, error)`, along with
String, payload validation, and dispatch cases.

- [ ] Add the exact contract declarations and machine storage. Rely on zero
  initialization for disabled/empty state, existing Machine retention across
  restart, and existing Destroy invalidation. Do not move counters onto Run.
- [ ] Append the routes; extend `allowedType(typ string) bool` with only the two
  new slice types. Extend `Manifest.Validate() error` to reject a highest ID of
  MaxUint32 and non-int primary syscall input fields named FD. Keep schema and
  callback validation strict.
- [ ] Extend `RenderKernel(m Manifest) ([]byte, error)` to emit
  `routeLimit=maxRouteID+1` and both FD helpers. Use exact typed assertions for
  syscall input FD fields only; exclude Poll/control FD fields, directory-context
  fields, and result FDs. Do not add reflection or a handwritten extraction list.
- [ ] Fill the canonical catalogue with all 43 names (8 rt, 26 fs, 9 net).
  Merge FcntlLock into FcntlInt, split Sync using DataOnly, and derive
  DescriptorBased using the generated helper; use RouteSync for the private
  fdatasync slot. Preserve the target's stable fstatat spelling.
- [ ] Implement observation and increment in target §11 order: disabled guard
  before FD lookup; canonical slot and enabled-feature check; None for absent
  primary FD, Unknown for failed lookup, or Resource.Owner mapped to RT/FS/Net.
  Observe before close/mutation, call no provider operation, and retain no FD or
  resource pointer. Invalid owner follows the invariant failure path.
- [ ] Check total against MaxUint64 before mutation; increment total and exactly
  one bucket without allocation, locks, atomics, random draws, or scheduling.
  Maintain total equal to the bucket sum, making the same check sufficient for
  bucket overflow. Use the existing invariant failure behavior.
- [ ] Implement all four kernel methods with target Lookup validation. Start/Stop
  use the setter without clearing; Reset clears values without toggling. Snapshot
  emits copied, nonzero rows in slot/descriptor-enum order. Discovery copies all
  feature-available metadata, including uncalled names; it validates the handle.
- [ ] Change `renderDispatch(m Manifest) ([]byte, error)` to emit one
  `d.kernel.countSyscall(inv)` after successful `validateRoutePayload` and before
  the switch. Explicitly map Writev to Write beside Close to CloseFD, returning
  WritevResult without a Kernel.Writev method or nested Write request. Controls
  use existing control-result/error transport and never count.
- [ ] Add `// TODO(not planned): ...` comments at observation, normalized-row,
  and catalogue boundaries explaining where future collection/row/catalogue
  selection belongs. Document currently absent fields/stages without a no-op
  filter, successful stub, runtime warning, or new public method.
- [ ] Regenerate the complete dependent output before compiling. Inspect generated
  route/decoder/FD helpers and identity changes; preserve provider and engine APIs.

#### Verification

- [ ] Add generator tests for sparse route IDs, the upper-ID bound, non-int
  syscall FD rejection, exclusions for Poll/DirFD/output FD fields, and the
  explicit Writev-to-k.Write call. Assert the generated count placement follows
  payload validation and happens once; retain reproducibility tests.
- [ ] Extend payload/result tests for all five new requests/results, correct
  control errors, borrowed Writev input, wrong payload/result types, unknown
  routes, and the private extra slot's rejection as a route. Exercise FD
  extraction only after validation, including every syscall FD field.
- [ ] Add catalogue coverage tests against the manifest: every syscall route and
  canonical variant is represented, only syscall kinds count, aliases have one
  name, groups are fixed, and DescriptorBased agrees with the schema. Check
  feature-set sizes 8/34/17/43 and 27 descriptor-based names out of 43.
- [ ] Add kernel tests for disabled default; shared versus independent machines;
  root; idempotent Start/Stop/Reset; prestart controls; stopped/crashed/restarted
  retention; failed-start preservation; stale/wrong-run/destroyed handles for
  every control; copied rows/catalogue; and overflow without partial mutation.
- [ ] Exercise the real dispatcher for success and errno attempts, invalid FD,
  mixed owner buckets, pre-close owner capture and repeated close Unknown,
  fixed group differing from descriptor owner, and invalid-owner failure.
  Verify controls/Poll, malformed typed payloads, unknown routes, teardown, and
  callbacks do not add counts. Fold normalized rows to verify totals/histograms
  and deterministic row order, without introducing filter behavior.
- [ ] Run validation commands G and K below. Confirm package compilation and
  generated output checks pass before proceeding. Record results and fixes.

### Task 2: Route vectored writes distinctly while preserving completion behavior

**Depends on:** Task 1. **Estimated authored change:** 150–300 lines including tests.

**Files:** `internal/compat/hooks/kernel/writev.go`,
`internal/kernel/dispatcher.go`, `internal/kernel/dispatcher_test.go`,
`internal/kernel/dispatch_fs_test.go`, and generated compatibility identity
outputs. Extend counter test helpers from Task 1 when useful.

**Target references:** §3 attempts/retries; §4 descriptor ownership; §7 dispatcher,
Writev hook, and generated identity; §8 Collection and pending-completion failure
cases; §11 `InternalPoll_writev` and `Dispatcher.fsDelay`. Brainstorm: Review
corrections incorporated (distinct writev and unchanged modeled behavior).

Keep the existing signatures:

```go
func InternalPoll_writev(fd int, vectors []syscall.Iovec) (uintptr, error)
func (d *Dispatcher) fsDelay(inv *Invocation, openMutation bool) (int64, error)
```

- [ ] Change only the hook's submitted request/result decoder from Write to
  Writev. Preserve vector validation, flattening order, race memory annotations,
  invocation-scoped borrowed data, byte count, and partial-write/error behavior.
  Hook-local early validation failures remain outside observation.
- [ ] Extend the Read/Write fsDelay branch to Writev using its validated FD.
  The generated syscallInputFD helper may replace the existing two-type switch.
  Resolve the active epoch and descriptor owner as before; FS latency applies
  only to FS-owned descriptors. Leave completion/cancellation ownership intact.
- [ ] Extend existing filesystem dispatch fixtures to exercise Writev latency,
  immediate provider effects, and clearing borrowed input before delayed wakeup.
  Verify RT/Net-owned descriptors do not acquire FS delay or require FS service.
- [ ] Verify a dispatched Writev records writev once and no ordinary write entry,
  including partial/error attempts. Verify Stop while pending retains the entry,
  Reset while pending clears it, completion cannot restore it, and crash/cancel
  preserves the entry while discarding stale completion and borrowed views.
- [ ] Refresh generation/compatibility identity after the hook edit. The trace
  label changes from call Write to call Writev; retain ReplayRevision because
  source/route identity already separates these artifacts and no scheduler or
  random-stream semantics are being changed.

#### Verification

- [ ] Run G, K, B, and N. At this stage N checks the existing ordered payload and
  partial-write behavior; monitor assertions are added in Task 4. Confirm FS
  latency/cancellation tests execute and pass, and record actual results.

### Task 3: Expose the machine-bound public monitor in native and simulated builds

**Depends on:** Tasks 1–2. **Estimated authored change:** 350–600 lines including tests.

**Files:** new `syscall_monitor.go`, `syscall_monitor_sim.go`,
`syscall_monitor_native.go`, `internal/tests/rt/syscall_monitor_test.go`;
update `api_native_test.go`. Existing root Machine and lifecycle APIs otherwise
retain their fields and signatures.

**Target references:** §2 complete public contract and ownership; §3 lifecycle;
§6 normalized row folding; §7 Public API; §8 Queries/control/errors;
§11 `Machine.SyscallsMonitor` and all `syscallsMonitor` methods. Brainstorm:
Revision 2 Revised API and the user's accepted lifecycle.

Create the exact common declarations:

```go
type SyscallGroup string
const (
    SyscallGroupRT  SyscallGroup = "rt"
    SyscallGroupFS  SyscallGroup = "fs"
    SyscallGroupNet SyscallGroup = "net"
)
type DescriptorGroup string
const (
    DescriptorGroupNone    DescriptorGroup = "none"
    DescriptorGroupRT      DescriptorGroup = "rt"
    DescriptorGroupFS      DescriptorGroup = "fs"
    DescriptorGroupNet     DescriptorGroup = "net"
    DescriptorGroupUnknown DescriptorGroup = "unknown"
)
type SyscallStat struct {
    Count        uint64
    Group        SyscallGroup
    ByDescriptor map[DescriptorGroup]uint64
}
type SyscallInfo struct {
    Name            string
    Group           SyscallGroup
    DescriptorBased bool
}
type SyscallsMonitor interface {
    Start() error
    Stop() error
    Reset() error
    Counts() (map[string]uint64, error)
    Stats() (map[string]SyscallStat, error)
    List() ([]SyscallInfo, error)
}
type syscallsMonitor struct { machine Machine }
func (m Machine) SyscallsMonitor() SyscallsMonitor {
    return syscallsMonitor{machine: m}
}
var _ SyscallsMonitor = syscallsMonitor{}
```

The sim implementation adds these private helpers and implements the six
interface signatures on the value receiver `syscallsMonitor`:

```go
func (m syscallsMonitor) setCounting(enabled bool) error
func (m syscallsMonitor) snapshot() ([]contracts.SyscallRecord, error)
```

- [ ] Add the documented common API, constants, and compile-time interface
  assertion. The accessor is a pure non-nil handle constructor even on native or
  invalid Machines. It performs no validation, transport, scheduling, enabling,
  or separate monitor registration and owns no counter state.
- [ ] Add the `//go:build sim` implementation using existing `m.machine.ref()`,
  control.Call, and generated decoders. Start/Stop share the boolean setter;
  Reset uses its control. Each Counts/Stats query makes one snapshot call;
  List makes one machine-scoped catalogue call. Propagate validation errors.
- [ ] Fold rows by name for Counts and by name/descriptor group for Stats.
  Preserve fixed Group; allocate every returned map, including nested histograms,
  on the caller. List copies metadata and sorts by Name. Empty success returns
  non-nil empty containers; failed queries return nil and the existing error.
- [ ] Add the `//go:build !sim` methods: Start/Stop/Reset return ErrNotSim and all
  queries return nil, ErrNotSim. Add no native state, host tracing, private sim
  helpers, JSON tags, Close method, or superseded v1 compatibility wrappers.
- [ ] Extend TestNativeControls to verify the non-nil accessor and all six errors,
  including nil query results. Add sim tests named TestSyscallMonitor... for
  empty state, Start/Stop/resume, reset while enabled and disabled, two handles
  sharing state, read-only query behavior, static sorted List, nonzero-only
  results, frozen Counts/Stats equality, and detached maps/nested maps/slices.
  Exercise all six methods on an unstarted machine and a zero Machine handle;
  preserve existing Lookup error ordering and nil results on query errors.

#### Verification

- [ ] Run A, B, and R. Confirm the native root package compiles and the new rt
  tests execute through translated controls and generated dispatch. No source
  generator change is expected here; regenerate only if its inputs changed.

### Task 4: Cover descriptor groups and lifecycle through translated application calls

**Depends on:** Task 3. **Estimated authored change:** 500–900 lines of tests.

**Files:** extend `internal/tests/rt/syscall_monitor_test.go`; create
`internal/tests/fs/syscall_monitor_test.go`,
`internal/tests/integration/syscall_monitor_test.go`; update
`internal/tests/net/writev_test.go`. Reuse the suites' existing metatesting and
testsupport machinery without changing feature applicability.

**Target references:** §3 entire lifecycle table and scheduling semantics;
§4 all descriptor cases; §5 names and feature subsets; §6 normalized rows;
§8 complete failure table; §9 Required test changes. Brainstorm: descriptor
owner decision and future filters' retained owner dimension. No production
types, routes, DB/cache schema, or interfaces are added by this task.

- [ ] Use explicit syscall wrappers and channel barriers for exact counts.
  Name new behavioral tests TestSyscallMonitor... so focused commands select
  them. Keep logging/diagnostic writes outside measured windows. Do not assume
  os.ReadFile or net polling produces one routed attempt.
- [ ] In rt tests, cover Getpid/None, a standard descriptor/RT, invalid read/
  Unknown, root self-monitoring, isolation between child machines, prestart
  enablement, completed/crashed/restarted machine access, retained enabled state,
  and all six methods failing after Destroy. Cover wrong-run validation with
  existing kernel fixtures; do not expose handle-forging APIs for tests.
- [ ] Verify monitoring enabled before child Start includes its first routed
  workload and that restart retains statistics while using the new descriptor
  table. Retain Task 1's failed-prepare and stale-run evidence. Avoid new package
  init side effects solely for the test or any changed lifecycle contract.
- [ ] In fs tests, measure FS-owned descriptor operations and openat/None;
  distinguish fsync/fdatasync, merge both fcntl forms under fixed rt, and retain
  actual descriptor classification for failed calls. Check close before removal
  and invalid/reused descriptors using the current epoch's ownership.
- [ ] Extend TestNetBuffersWriteTo with a client-only window: start after dialing,
  stop immediately after WriteTo and before deferred close. Assert writev exists,
  Group is rt, all its observed buckets are net, ordinary write is absent in
  that window, and multiple routed attempts occur with the existing bounded TCP
  buffer. Preserve byte count, ordered payload, and consumed-buffer assertions.
  testsupport.StartConfigured starts immediately; do not enable after it and
  assume the client has not already run.
- [ ] In integration tests, issue controlled reads from an FS descriptor and a
  Net descriptor on one machine, producing one read entry with Group=rt and
  separate exact fs/net buckets. Attempt an FS-group Fstat on a socket and check
  Group=fs with descriptor net even when the provider returns an error. Assert
  Count equals bucket sum and frozen Counts matches Stats; mutate returned
  containers and confirm later snapshots remain intact.
- [ ] Retain tests for no self-counting by monitor queries/controls and absence
  of zero names/buckets. Use the existing deterministic metatests to repeat the
  same scenario/seed; keep fixtures deterministic and no new filter tests or
  native observation paths. Check all four discovery subsets via Task 1 fixtures.

#### Verification

- [ ] Run B if executable inputs changed, then R, F, N, and I. Confirm all
  selectors execute the intended tests under their feature sets. Record seed
  coverage; Task 5's ordinary metatests supply repeated same-seed validation.

### Task 5: Document the implemented contract and complete ordinary validation

**Depends on:** Tasks 1–4. **Estimated authored change:** 50–90 documentation lines,
plus any logged fixes and generated output.

**Files:** new `docs/syscalls.md`; update `docs/features.md`, generation-owned
outputs as required, and this plan's execution records. Keep `README.md` and
`docs/design.md` unchanged. Touch production code only to resolve demonstrated
verification failures and log those changes.

**Target references:** §1 scope; §2 API example; §3 lifecycle/routed-attempt limits;
§4 descriptor buckets; §6 Pipeline/Future expression data contract/Storage and
configuration boundaries; §7 Documentation and generated identity; §9 Commands;
§10 explicit exclusions. Existing architecture: docs/design.md package ownership,
dispatcher, replay, and build/cache identity sections.

- [ ] At this final documentation stage, create a brief `docs/syscalls.md` for
  the implemented monitor. Include one usage example with handled errors and a
  stopped/barriered measurement window; explain Counts/Stats/List, fixed groups,
  the five owner buckets, independent snapshots, native errors, and the accepted
  start/stop/reset/restart lifecycle. State routed-attempt limits, including
  translated vgetrandom and GODEBUG writes. Summarize Machine-owned counters,
  entry-time descriptor ownership, copied rows, and future filtering boundaries
  in a short implementation note; filtering is not implemented. Mention Writev's
  trace label and build-identity separation. Do not reproduce the full target
  architecture or catalogue, or extend README.md/docs/design.md.
- [ ] After behavior is validated, replace docs/features.md's unsupported
  "Syscalls couter or statistics" row with the correctly named implemented
  monitor capability, its routed/per-machine limits, and a link to syscalls.md.
- [ ] Verify there is no database/ORM/migration/index or cache-format change.
  The existing STRICT cache table remains `cache(key TEXT PRIMARY KEY NOT NULL,
  timestamp INT, contents BLOB)`, with existing keys, contents, seven-day
  retention, and hourly timestamps. Add no configuration fields, CLI flags,
  environment variables, provider ports, engine/ABI interfaces, or CEL dependency.
- [ ] Run final generation checks appropriate to changed inputs. G already
  regenerates and checks both recorded Go baselines, refreshes profile metadata,
  and runs existing filesystem stringers. Inspect only tooling-produced identity
  deltas; do not hand-write origin/revision/evidence records or certify anything.

#### Verification

- [ ] Run V once for the completed implementation. Read the ordinary report,
  package failures, and skips; verify existing deterministic metatests include
  the new simulation tests. Reuse focused results from earlier tasks only while
  their executable inputs remain valid; rerun affected checks after any fix.
- [ ] Run Q and `git diff --check`. Check docs examples/types against actual
  declarations. Record build/test exit statuses, source identity, report paths,
  and remaining skips. Fix every failure encountered in the authorized checks;
  do not expand to race/large/certification to obtain a completion claim.

### Task 6: Architectural delta verification

**Depends on:** Tasks 1–5. **Estimated change:** review/logging; targeted fixes only.

**Files:** every changed production/test/documentation/generated file, compared
with the target; update this plan's NOT IMPLEMENTED GAPS and progress log.

**Target references:** all revision 2 sections, especially §2 exact public
signatures, §5 entire catalogue, §6 all structures/schema boundaries/diagram,
§7 complete file specification, §8 all paths/errors, §9 coverage, and §11 each
method algorithm. Compare with the brainstorm's accepted decisions, not v1's API.

#### Verification

Use the source and successful execution evidence from Tasks 1–5. For every gap,
write the affected target section, actual behavior, fix, and verification into
NOT IMPLEMENTED GAPS. Fix it within the agreed architecture; record any needed
departure and obtain user agreement before treating it as deferred. Mark an
unresolved too-complex item `[_]` and leave affected completion boxes unchecked.
If fixes change executable inputs, regenerate where needed and rerun affected
checks; refresh V when needed so final ordinary evidence matches the final code.
Do not rerun already valid tests solely to repeat their evidence.

- [ ] Verify the public component: exact constants/types/interface, pure
  machine-bound accessor, sim/native method signatures, one-call snapshots,
  errors for all six methods, sorting, detached ownership, non-nil empty successes,
  and absence of v1 wrappers or extra public filtering/session APIs (§2/§7/§11).
- [ ] Verify contracts and machine storage: enum values/limit, normalized record
  fields, static metadata, canonical array and private slot, total/histogram
  invariant, overflow, bounded allocation-free updates, supervisor ownership,
  and counters surviving Run replacement without extra Destroy cleanup (§3/§6).
- [ ] Verify catalogue and generator: all names/variants/groups and feature
  subsets, stable existing route IDs, exact five new schemas, max-based routeLimit,
  generated FD capability/extraction, validations, and regeneration consistency.
  The fdatasync slot must remain uncallable and Writev must call k.Write (§4/§5/§7).
- [ ] Verify capture/dispatch and memory behavior: exactly one admitted and
  validated entry observation, owner before mutation, correct None/Unknown,
  modeled errno/retry counts, preserved feature aborts, no Poll/control/cleanup
  counts, no new happens-before relationships, and no retained memory views or
  descriptor references. Check Writev FS-only latency and pending reset/stop/
  crash/completion behavior against actual fixtures (§3/§4/§8/§11).
- [ ] Verify the future-filter data boundary without implementing filtering:
  observation resolves static metadata; copied rows retain owner bucket counts;
  public folding preserves exact totals; catalogue supplies only static fields.
  Confirm the agreed TODO comments and the absence of CEL/predicates/caches,
  per-event/FD histories, and speculative expression APIs (§6/§10).
- [ ] Verify package/dependency/storage/configuration delta: no concrete provider
  imports in controls/kernel/contracts, no cross-provider coupling, no new
  engine/ABI/port/config/HTTP/DI or DB/cache schema change. Check generated
  identity and trace-label documentation, unchanged replay revision for this
  scoped change, and the brief docs/syscalls.md plus feature-matrix row accurately
  describing evidence. Confirm README.md/docs/design.md were not extended (§6/§7).
- [ ] Verify test evidence covers all target §9 files/cases, ordinary reports
  match final executable inputs, skipped suites are distinguished from passes,
  and every logged gap has been fixed or explicitly deferred with its unresolved
  status preserved. Record any incidental fixes under NOT PLANNED CHANGES.
- [ ] Verify actual production wiring end to end: real translated wrappers and
  the Writev hook submit the generated requests; existing bound control transport
  carries all monitor methods and target MachineRef to live dispatch; generated
  decode reaches the kernel counter/control methods; caller Machine/current FD
  table supplies owner/state; normalized rows return to the public folds and
  catalogue sorting. Confirm reachability from real application and public API
  calls, not only direct test calls. No extra composition registration is needed.

## Validation Commands

Run from repository root during implementation, not as part of drafting this
plan. Normal builds/tests use Go tooling's selected supported compiler. Exact
versions inside `make go-generate` are recorded generation baselines. Preserve
linkname flags. These labels are referenced by task verification checklists.

**G — regenerate and check all dependent outputs:**

```sh
make go-generate
```

**K — focused native contracts/kernel/generator checks:**

```sh
go test -ldflags=-checklinkname=0 -tags=linkname ./internal/kernel ./internal/contracts ./internal/routegen
```

**A — native public control behavior:**

```sh
go test -ldflags=-checklinkname=0 -tags=linkname -run '^TestNativeControls$' .
```

**B — build source-matched launcher and workers:**

```sh
make build
```

**R/F/N/I — focused translated behavior with matching feature selections:**

```sh
# R
./.gosim/gosimtool test -features rt -seeds 1-3 -run '^TestSyscallMonitor' ./internal/tests/rt
# F
./.gosim/gosimtool test -features rt,fs -seeds 1-3 -run '^TestSyscallMonitor' ./internal/tests/fs
# N
./.gosim/gosimtool test -features rt,net -seeds 1-3 -run '^TestNetBuffersWriteTo$' ./internal/tests/net
# I
./.gosim/gosimtool test -features rt,fs,net -seeds 1-3 -run '^TestSyscallMonitor' ./internal/tests/integration
```

**V — final ordinary local checks in an isolated source snapshot:**

```sh
make test
```

Read `test-reports/unit/latest/report.md`, `timeline.md` or `timeline.json`, and
the reported failing package logs. Confirm zero failures/errors, and record
skips separately. Record snapshot/source identity from the command logs and
`.gosim/verification/<patch>/<mode>/verification.json`; concurrent checkout
changes do not validate those later changes. Existing ordinary metatests provide
repeated same-seed checks. Do not substitute bare native `go test ./...` with
missing/stale simulation workers.

**Q — formatting:**

```sh
./.ci/gofmt check
```

No race/large suite, matrix, certification, container, or VM command is included.
No new benchmark suite is needed; inspect the counter path for bounded storage,
absence of allocation/synchronization/random draws, and disabled early return.
Full-project lint is outside repository policy and outside this plan's scope.

## NOT CONSIDERED & TODO

These are the accepted target §10 boundaries, not implementation gaps. Use
focused `// TODO(not planned): ...` comments at the relevant extension points;
do not add generic success/NotImplemented methods or warning logs.

- [postponed] CEL parsing/checking/evaluation, filter APIs, compilation placement,
  evaluation budget/errors, cache, and translated-runtime compatibility. Preserve
  observation, normalized-row, and static-catalogue boundaries only.
- [postponed] Individual FD numbers/generations, paths/inodes/addresses, time,
  errno, latency/bytes, per-event history, multiple primary FDs, output-FD
  attribution, and mapping-address attribution. Owner-only aggregates cannot
  answer these later queries without another data model.
- [postponed] Native tracing, rejected raw calls, hook-local early failures,
  scheduler/Poll metrics, simulation-wide aggregation, CLI export, and persistence.
  Existing supported operations/errors are unchanged; no unobservable names are
  inserted merely because Linux can issue them.
- [potential-improvement] Review future handwritten hooks that bypass typed
  routes; manifest checks cannot prove those hooks are observed.
- [potential-improvement] Add an explicit primary-descriptor schema annotation
  if future multi-FD routes need it. Today only the agreed syscall input FD
  convention is supported; do not silently choose among multiple primary FDs.

## NOT PLANNED CHANGES

None during planning. During execution, record incidental fixes with file,
reason, scope, and relevant diff/commit, particularly fixes to unrelated failures
found by authorized verification. This section does not authorize scope expansion.

## NOT IMPLEMENTED GAPS

No design-to-plan gap found during planning. Implementation has not started;
this is not a claim that any feature exists. During execution, record each gap
with target section, component, observed difference, resolution, and validation.
Use `[_]` for unresolved too-complex gaps; never mark those as completed work.

## Progress log

| Date | Stage/task | Change and evidence | Status |
| --- | --- | --- | --- |
| 2026-10-21 | Planning | Compared revision 2 with public machine/control patterns, route manifest/generators, dispatcher/FD ownership, existing latency/writev/metatest fixtures, and repository validation rules. Confirmed 78 routes, 42 syscall routes, IDs 79–83 available, and no separate docs repository. Checked links, fence balance, task sequence, per-task verification, and the final production-wiring checkbox. | Plan only; no implementation/tests run |
| 2026-10-22 | Planning clarification | User accepted the six-task breakdown and directed the final documentation stage to create a brief docs/syscalls.md instead of extending README.md or docs/design.md. Updated Task 5, final verification, and the target documentation scope. | Plan updated; implementation remains unstarted |

For each completed implementation task, append changed files/commit, commands,
exit statuses, executed test names/seed coverage, reports/source identity, and
any fixes or gaps. Leave all task checkboxes unchecked until their work passes.
