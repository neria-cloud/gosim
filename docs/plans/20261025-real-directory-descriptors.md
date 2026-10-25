# Implementation plan: real directory descriptors and Go os.Root

**Plan:** `docs/plans/20261025-real-directory-descriptors.md`

**Project source:** `.` (repository root; no `project.txt`)

**Target architecture:** [20261025-real-directory-descriptors-target.md](../brainstorms/20261025-real-directory-descriptors-target.md)

**Full brainstorm:** [20261025-real-directory-descriptors.md](../brainstorms/20261025-real-directory-descriptors.md)

**Reviews:** [technical and gquorum peer discussions](../review/feat_real-directory-descriptors.md), Threads 1–3

**Date:** 2026-10-25

**Source baseline:** `d8f82ea`, branch `feat/real-directory-descriptors`, plus the preserved uncommitted prototype listed below

**Target SHA-256 at planning:** `7d5716c709dd3d93cca085a4ffaf6e0e0c55d12cd586d82512fe0759f91e31cc`

**Status:** completed on 2026-10-25 within the user-approved Darwin ARM64 execution scope, including the approved Task 6 translator extension. All nine tasks are committed after validation. Native Linux/AMD64 and the agreed GS-03/GS-02 qualification gates remain explicitly open; this is not full issue #12 acceptance.

## Active goal addendum — user steering, 2026-10-25

Execute this plan by aligning every change and the final architectural audit to the verified target architecture at `docs/brainstorms/20261025-real-directory-descriptors-target.md`. The target defines the desired end state; the incomplete prototype and passing narrow tests are not substitutes for it. If the schedule conflicts with the target, record the discrepancy and follow the target, except for explicit later user instructions.

The user specified: "arm64 Darwin for the plan. Amd64 we will test in future" and "Use the verified target architecture for aligning take to the target state. Add this info to the goal". Current execution is therefore Darwin ARM64, including native Darwin checks and translated Linux semantics under both Go families. AMD64 execution is deferred; native Linux execution is also outside this Darwin-only run. Keep Linux/macOS source compatibility and all corresponding test fixtures in scope. Report deferred native results explicitly, without claiming Linux execution, cross-platform qualification or full issue acceptance. No extra runner or infrastructure is required to finish the currently authorized execution scope.

This addendum is persisted here because the active goal references this plan and the goal API exposes status changes only; the API objective text itself was not edited. It supersedes the original execution requirements for native runners, without changing the verified implementation architecture or permitting restricted suites.

## Outcome and accepted scope

Implement inode-bound directory descriptors and the Linux syscall semantics required by Go's translated `os.Root` and gquorum's descriptor-based journal operations. A handle continues to refer to its original inode after rename or replacement; closing it invalidates relative use. Absolute operands ignore dirfd. Final symlink handling, open flags, removal and rename semantics must be intentional and independently tested.

The target is the single design source. Its accepted decisions are retained:

- Actual symlink inodes, traversal, readlink and Root confinement tests are in scope.
- Use the existing shared descriptor table, File inode references and epoch cleanup. Do not replace handles with stored paths or replace Go's Root algorithm.
- Add the narrowly defined directory-rename topology dependencies; do not make child sync persist an otherwise unsynced mkdir entry in its parent.
- Retain the documented early read-only mutation-denial gate. Do not claim Linux-exact read-only-mount error precedence.
- Retain integer fcntl ENOSYS, reject O_NONBLOCK, and accept/strip O_CLOEXEC. The prototype's extra status-flag implementation is outside the target.
- Directory listings remain bounded to one complete batch. Overflow aborts as UnsupportedOperation before partial output. Directory Seek(0, SeekStart) rewinds; other directory seeks remain explicitly limited.
- Linux and macOS compatibility is mandatory. Native Darwin evidence is distinct from executing the simulated Linux source ABI on Darwin.
- Gosim-owned validation covers Go 1.25 and 1.26. Pinned gquorum requires Go 1.26; do not lower its Go directive to manufacture a 1.25 consumer fixture.

There is no new database, cache, public configuration, public control API, mount model or native host-I/O fallback. The diagram and ownership rules in target §[Contract and ownership](../brainstorms/20261025-real-directory-descriptors-target.md#contract-and-ownership) apply unchanged:

```text
translated os.Root / syscall / x/sys
  -> generated feature-gated adapters
  -> kernel typed dispatch and shared FD table
  -> contracts.FileOps
  -> FS provider descriptor/argument checks
  -> FS model component lookup, namespace changes and pending operations
```

## Starting point and execution rules

1. Preserve the current worktree until execution is directly requested. Do not use git restore/reset or regenerate code during planning. The prototype backup is `.gosim/recovery/gs01-restored-work.tar.gz`, with a tracked patch and SHA-256 manifest alongside it; these are recovery material, not acceptance evidence.
2. Existing edits already span contracts, FS provider/model, adapters/schema, kernel dispatch/catalogue and three public-API tests. Existing new files are `internal/fs/model/path.go`, `internal/fs/symlink.go`, and `internal/tests/fs/directory_descriptors_test.go`. Reconcile them in place during execution: retain useful behavior, deliberately replace rejected parts, and do not duplicate routes or tests. Earlier prototype passes do not qualify this target.
3. The prototype still has a boolean resolver, broad fcntl flag support, silent directory truncation, no new topology dependencies, stale capability text and incomplete tests. All plan checkboxes start unchecked despite that prototype.
4. Complete tasks in dependency order. A task must compile and pass its stated local checks before it is marked done. Fix failures in checks actually run, including incidental failures, and record any unrelated fix under NOT PLANNED CHANGES. Do not hide failures by skipping cases, weakening assertions or resetting user work.
5. Keep each authored change around or below 1,500 lines including tests; generated files are additional. Task estimates below are planning bounds, not permission to omit cases. Prefer existing fixtures and semantic tables over duplicated harness infrastructure.
6. Keep kernel/contracts independent of concrete providers and contracts dependent only on the primitive ABI. Retain the supervisor-owned, non-yielding model operations, invocation-scoped buffers, independent random streams and epoch checks. No new randomness is needed.
7. Do not hand-edit generated adapters, dispatch, route types, revisions or stringer outputs. Use `cmd/gosim-gen` and the existing model stringer command. Reconcile IDs 84/85 with the current schema if the baseline changes; do not renumber existing routes.
8. Generation for AMD64 and ARM64 is source/ABI validation, not execution on both architectures. Use the recorded baselines for generation, while ordinary caller support remains stable 1.25.* and 1.26.*.
9. Each task runs relevant checks. Task 8 runs the broader ordinary suites for both families with independent report roots. Never run bare native `go test ./...` against missing or stale simulation artifacts. Use the launcher and ordinary prepared-worker verification path.
10. Race, large/candidate sweeps, manual certification, Docker/Colima/VM starts, new remote workers and CI triggers are not authorized by this plan. Do not use their aliases, environment variables or existing large test entry points to obtain restricted coverage. Focused new semantic fixtures are not a substitute name for a large sweep.
11. Record exact commands, compiler/source identities, OS/kernel, architecture, filesystem, results, skips, expected failures, missing runs and report locations. A selector running zero relevant tests is not a pass. Missing native evidence keeps its verification checkbox open.
12. Docs are a normal directory in this repository, not a symlink or nested Git repository. The current branch already matches the task. Planning changes must be limited to this plan; do not stage the prototype or review thread with it. The supplied go-plan skill's documentation commit applies only to this plan.

## Task list

### Task 1: Reconcile typed interfaces, adapters and descriptor flag scope

**Depends on:** none; starts from the preserved prototype. **Estimated authored change:** 350–900 lines.

**Target references:** [Contract and ownership](../brainstorms/20261025-real-directory-descriptors-target.md#contract-and-ownership), [Provider interfaces and semantics](../brainstorms/20261025-real-directory-descriptors-target.md#provider-interfaces-and-semantics), [Typed transport, hooks and metadata](../brainstorms/20261025-real-directory-descriptors-target.md#typed-transport-hooks-and-metadata), [Integer fcntl scope](../brainstorms/20261025-real-directory-descriptors-target.md#integer-fcntl-scope). Brainstorm: accepted scope and review closure, not superseded initial recommendations.

**Files:** `internal/contracts/fileops.go`, `service_fakes_test.go`; `internal/routegen/{routes.json,adapters.go,adapter_body.go}`, owning adapter/coverage tests; `internal/compat/profiles/go125.json`; `internal/fs/{file.go,ops.go,ops_test.go}`; `internal/tests/fs/syscall_monitor_test.go`; `internal/kernel/{dispatcher.go,syscall_stats.go,syscall_catalogue_test.go}`. Generated route/dispatch, architecture adapters and compatibility metadata are produced by the generator.

The added FileOps methods must be exactly:

```go
Symlinkat(contracts.CallContext, string, int, string) error
Readlinkat(contracts.CallContext, int, string, abi.ByteView) (int, error)
```

Within package contracts, use the local CallContext spelling. Existing FileOps signatures remain otherwise unchanged.

- [x] Inventory and preserve the useful prototype changes. Confirm the current route maximum and retain appended IDs 84 Symlinkat and 85 Readlinkat rather than adding duplicates. Keep their target argument/result order: Target/DirFD/Path, and DirFD/Path/Into → Count.
- [x] Retain real provider implementations already present in the draft; do not add successful temporary stubs to get a build. Finish their ordinary contract/fake wiring as a single buildable change.
- [x] Reconcile FcntlInt to validated-file ENOSYS; remove prototype-only statusFlags/descriptorFlags behavior and O_NONBLOCK acceptance. Preserve the existing File inode, read/write access, position and release ownership. Restore only the corresponding fcntl/NONBLOCK test expectations deliberately, retaining unrelated descriptor improvements.
- [x] Keep O_CLOEXEC accepted/stripped. Keep DIRECTORY/NOFOLLOW values in the simulated-architecture files: ARM64 0x4000/0x8000, AMD64 0x10000/0x20000. Do not import Darwin constants into provider decisions.
- [x] Verify the added private hooks Fstatat, Mkdirat, Renameat, Readlinkat and Symlinkat plus syscall readlinkat/symlinkat and x/sys Readlinkat/Symlinkat against both compiler families. Preserve existing Openat/Unlinkat hooks and raw-call rejection.
- [x] Implement/test adapterType's selector handling for private `*syscall.Stat_t` using the existing reviewed kernel Stat_t layout. Preserve NUL validation, Require(FS), borrowed-view and race annotations. No new native-I/O bypass or Statx layout is introduced.
- [x] Author shared declarations/capabilities in the go125 baseline and inherit them in go126. Do not copy family implementations or introduce runtime version switches. Refresh origins only when required, with the matching recorded compiler and contract validation; leave historical evidence untouched.
- [x] Keep catalogue names/counts consistent: rt 8, rt/fs 36, rt/net 17, full 45. Directory-context FDs remain outside primary descriptor attribution. Add Symlinkat mutation latency; Readlinkat remains immediate metadata.
- [x] Regenerate adapters and profiles in the documented order and inspect the diff. Format the touched draft Go files, including the unformatted prototype test, as part of future execution only.

#### Verification

- [x] Run focused contracts, kernel, provider and routegen checks under Go 1.25.14 and 1.26.9 (V1); test hook selector conversion, route ownership/coverage and catalogue counts.
- [x] Run generation checks for both recorded families (V2); no generated-output mismatch or stale revision remains.
- [x] Through the launcher, run the existing descriptor/Root prototypes and filesystem monitor under both compilers (V3). Confirm NewFile and the directory chain work through ENOSYS fallback; no extra fcntl success expectation is retained.

### Task 2: Complete component resolution, symlink storage and inode-bound operations

**Depends on:** Task 1. **Estimated authored change:** 800–1,450 lines including focused model/provider tests.

**Target references:** [Data structures](../brainstorms/20261025-real-directory-descriptors-target.md#data-structures), [Provider interfaces and semantics](../brainstorms/20261025-real-directory-descriptors-target.md#provider-interfaces-and-semantics), [Model method inventory](../brainstorms/20261025-real-directory-descriptors-target.md#model-method-inventory), [Data flows and failure modes](../brainstorms/20261025-real-directory-descriptors-target.md#data-flows-and-failure-modes), and algorithms `dirInode`, `resolve`, `OpenFileWithOptions`, `Symlink and Readlink`.

**Files:** `internal/fs/{ops.go,symlink.go,file.go}`, provider tests; `internal/fs/model/{filesystem.go,path.go,symlink.go,crash.go}`, new `path_test.go`/`symlink_test.go` and existing model storage/open-directory tests. Moving Symlink/Readlink from prototype path.go to the target's model/symlink.go is a focused organization change, not another resolver.

Target structures (use the target's field and method inventory throughout):

```go
type finalPolicy uint8
const (
    resolveFollowing finalPolicy = iota
    resolveNoFollow
    resolveCreateEntry
    resolveMutationEntry
)
type resolveOptions struct {
    Final finalPolicy
    AllowMissingFinal bool
}
type resolvedPath struct {
    Parent int
    Name string // empty only for slash-only root; dot/dotdot retained
    Inode int
    TrailingSlash bool
}
type OpenOptions struct { Directory, NoFollow bool }
type backingSymlink struct {
    inode int
    target string
    linkCount int
    meta inodeMetadata
}
```

- [x] Replace the prototype's followFinal/mutation booleans with the target final policies. Use one bounded component walker; migrate remaining walkpath callers and remove the obsolete walker. Keep OpenFile as the convenience wrapper over OpenFileWithOptions.
- [x] Preserve empty-input versus slash-only-root and literal final dot/dotdot. Resolve relative links from their containing directory, absolute links from RootInode, and link/.. through actual target traversal. Enforce the 40-link limit and Linux pathname/component limits without filepath.Clean normalization.
- [x] Preserve prefix/search failure order and syscall-specific terminal policies. Mkdir/Symlink inspect an existing final entry before their slash rule; ordinary O_CREAT final names with slash return EISDIR before final lookup; exclusive creates do not follow existing links. Treat root/dot/dotdot separately. Use the referenced native-oracle table as an expectation to qualify, not as executed evidence.
- [x] Enforce DIRECTORY before any truncation/creation effect and reject CREATE|DIRECTORY with EINVAL. Enforce final NOFOLLOW, including DIRECTORY|NOFOLLOW versus NOFOLLOW alone, and distinguish intermediate traversal. Ignore EXCL without CREATE on regular files. Require write permission for TRUNC even with O_RDONLY. Failed opens leave data, entries and reference counts unchanged.
- [x] Implement dirInode validation: invocation first, ordinary empty name before FD lookup, absolute names ignoring dirfd, AT_FDCWD using cwd, owned/live directory File using its inode, EBADF for invalid/closed/stale resources and ENOTDIR for live non-directories. Validate old and new rename bases independently; do not conflate path lookup with current cwd.
- [x] Complete symlink allocation, stat type/size, directory entry type, metadata ownership, unlink/rename link counts, garbage collection and by-value crash-state cloning. Targets are immutable metadata, not allocated file-data blocks. Do not introduce hard links or O_PATH symlink handles.
- [x] Implement Symlinkat target validation before dirfd lookup after feature/invocation checks: ENOENT for empty target, EINVAL for NUL, ENAMETOOLONG for oversized target. Implement Readlinkat with no NUL terminator, bounded truncating copy and zero-buffer EINVAL; retain only invocation-scoped output memory.
- [x] Make Statat, Chmodat and Chownat explicitly follow/no-follow aware. No-follow chmod of a link returns EOPNOTSUPP; no-follow ownership changes the link metadata, not the target. Preserve the approved early read-only gate and document its deliberate error-precedence difference rather than adding hidden policy state.
- [x] Retain exactly-once reference release on close, failed descriptor allocation and epoch teardown. Opening after directory rename addresses the old inode, not a replacement at its old path. Unlinked open files remain usable. Unlinked directories reject new entries and Getcwd fails when there is no linked path.
- [x] Add model/provider regressions for cwd independence, renamed/replaced handles, absolute operands, closed/non-directory/foreign resources, permissions, read-only denial/re-enable, link loops/dangling targets, no-follow metadata and failed mutation atomicity. Record removed-directory dotdot as a native-qualification case before claiming its precise semantics.

#### Verification

- [x] Run the affected model/provider suites under both compilers (V1), including reference rollback and stale-epoch cases.
- [x] Run portable descriptor/Root smoke checks natively on the available supported host and through simulation (V3/V4). Linux-native qualification not available locally remains recorded for Task 8, not passed by a Darwin result.
- [x] Verify no new public control/configuration, concrete-provider import across boundaries, host fallback or random-stream draw was introduced.

### Task 3: Finish namespace mutation rules and minimal crash-topology dependencies

**Depends on:** Task 2. **Estimated authored change:** 500–1,250 lines including enumeration regressions.

**Target references:** [Directory topology and crash ordering](../brainstorms/20261025-real-directory-descriptors-target.md#directory-topology-and-crash-ordering), [Rename](../brainstorms/20261025-real-directory-descriptors-target.md#rename), [Namespace dependency publication](../brainstorms/20261025-real-directory-descriptors-target.md#namespace-dependency-publication), [Startup, shutdown and recovery](../brainstorms/20261025-real-directory-descriptors-target.md#startup-shutdown-and-recovery).

**Files:** `internal/fs/model/{filesystem.go,path.go,pendingops.go,crash.go}`, new `namespace_test.go`/`topology_test.go`, relevant pending/crash tests, generated `depkind_string.go`; provider rename/removal tests.

```go
type dirOp struct {
    parts []dirOpPart
    movedDirectory int
    destinationChain []int // includes destination Q, excludes root and moved D
    emptyDirs []int
}
func (fs *Filesystem) directoryAncestorsInclusive(inode int) ([]int, error)
func (fs *Filesystem) prepareDirOp(parts []dirOpPart, moved int, chain, empty []int) *dirOp
func (o *dirOp) addDeps(p *pendingOps, op *pendingOp)
```

- [x] Complete rename/removal validation before a single atomic dirOp: existing source, special terminal names, trailing slashes, same-inode no-op, permissions, replacement types, empty-directory replacement and cycle rejection. Do not strengthen ordinary rename into no-replace.
- [x] Represent linked/unlinked directory state independently of retained parent identity. Preserve already-open overwritten directory handles. Prevent traversal from looping on impossible parent state; do not invent complete directory GC or POSIX directory link-count support.
- [x] Append depKeyDirTopology without renumbering keys. Only a directory rename publishes Topo(D), using depStoreAs. Its read dependencies are Topo(Q) and each other non-root destination ancestor visited by the cycle check, including Q itself.
- [x] Removing/replacing empty X depends on pending WriteDir(X). Retain existing allocation and per-entry dependencies. Do not publish topology on mkdir or add ordinary child-parent, source-chain or file/symlink-mutation topology edges.
- [x] Do not add topology keys to flushInode and do not expand touched-inode reference bookkeeping for dependency-only directory IDs. Existing entry-key chains already order subsequent moves of D.
- [x] prepareDirOp owns copies of input slices; maintain their immutability after addOp. Crash-plan snapshotting already records dependency keys; keep dirOp.parts copying and symlink value cloning without unnecessary deep-copy work for inert dependency inputs.
- [x] Generate the model stringer output. Keep scheduler/provider streams and replay revision unchanged; source identity captures changed algorithms.
- [x] Enumerate a small crash graph for B moved out of A, then A moved into B. Check reachable-entry and parent-pointer consistency, detached objects, at-most-one location and absence of cycles.
- [x] Add the negative durability cases: mkdir P/create P/f/fsync(P) without parent sync permits P to be missing; moving D then syncing D without a flushed Topo(D) dependent does not force its own link; unrelated file renames, sibling moves and unsynced data remain uncertain.
- [x] Add the positive exception: move D, then move E into D, then fsync(D) persists D's move through E's destination dependency. Never drop Q to make the negative case pass. Empty-directory removal/replacement must not recover deleted children.

#### Verification

- [x] Run model pending-operation, topology and crash tests under both compilers (V1). Keep enumeration fixtures small and positively bounded.
- [x] Assert both allowed uncertainty and forbidden states; a test that only validates the all-durable outcome is insufficient.
- [x] Confirm filesystem teardown and snapshot/iterator ownership still balance resources and no topology dependency accidentally becomes a direct fsync root.

### Task 4: Make directory listings bounded and fatal on overflow; support rewind

**Depends on:** Tasks 1–3. **Estimated authored change:** 300–850 lines; generated transport output additional.

**Target references:** [Directory listing must not return false success](../brainstorms/20261025-real-directory-descriptors-target.md#directory-listing-must-not-return-false-success), [Getdents64 and directory rewind](../brainstorms/20261025-real-directory-descriptors-target.md#getdents64-and-directory-rewind), failure-mode table and typed transport.

**Files:** `internal/fs/ops.go`, `ops_test.go`, owning kernel integration tests; `internal/routegen/dispatch.go`, generator/dispatch tests; generated `internal/kernel/dispatch_gen.go` and revisions.

```go
func (i *Instance) Getdents64(ctx contracts.CallContext, fd int, into abi.ByteView) (int, error)
func (i *Instance) Seek(ctx contracts.CallContext, fd int, offset int64, whence int) (int64, error)
// Generated helper; authored behavior belongs in routegen/dispatch.go:
func syscallRouteResult(value any, err error) (Result, bool, error)
```

- [x] Validate the owned readable directory, then preflight every sorted entry using exactly the length rule the encoder uses. Do not write any output or advance didReaddir until the entire batch is known to fit.
- [x] Return a typed contracts.Failure/UnsupportedOperation on capacity overflow with Feature="fs", Operation="getdents64", Epoch=ctx.Epoch and PC=ctx.PC. Report the simulator limit, not a fabricated Linux errno or a successful truncated listing.
- [x] Preserve typed Failure in generated syscallRouteResult before errno conversion, using the existing control-result pattern. Keep other unmodeled errors on their existing invariant-failure path. Do not add global diagnostic defaults or weaken disabled/raw syscall guards.
- [x] Complete-batch success sets didReaddir; only completed enumeration may subsequently report EOF. Empty-directory EOF remains valid. Include symlink dirent types and no partial-output invariant tests.
- [x] On directory Seek(0, SeekStart), reset didReaddir and return zero. Other directory seeks, including zero SEEK_CUR, return EINVAL as an explicit model limitation. Regular-file position handling remains unchanged.
- [x] Test byte-for-byte unchanged output/cursor on overflow, exact fit, genuinely empty directory, read/rewind/read, closed/unreadable/non-directory resources and independent handles. Use a modest over-buffer fixture, not the restricted large suites.
- [x] In a kernel/engine integration test, verify fatal dispatch classification, calling run/machine/epoch/PC attribution, cancellation/cleared borrowed references and inability to continue or recover. Direct provider return-value tests alone do not prove fatal behavior.
- [x] Regenerate dispatch/adapters/profiles as required and check both families; no hand-edited generated helper.

#### Verification

- [x] Run provider/kernel/routegen checks under both compilers (V1/V2), retaining ordinary errno and unmodeled-error regression coverage.
- [x] Confirm the fatal path reaches Dispatcher.AbortRun with the supplied caller diagnostic and does not produce an ignorable result.
- [x] Verify the correct limited seek contract without weakening native Linux's legal SEEK_CUR reference behavior.

### Task 5: Add public-API, portability, fatal-guard and translated crash regressions

**Depends on:** Tasks 1–4. **Estimated authored change:** 850–1,450 lines including narrowly scoped guard fixtures/harness.

**Target references:** [Verification specification](../brainstorms/20261025-real-directory-descriptors-target.md#verification-specification), [Exact test-file applicability](../brainstorms/20261025-real-directory-descriptors-target.md#exact-test-file-applicability), [Complete Root surface](../brainstorms/20261025-real-directory-descriptors-target.md#complete-root-surface-under-go-125-and-126), topology regressions, mandatory portability.

**Files:** reconcile `internal/tests/fs/directory_descriptors_test.go`; add `directory_reference_linux_test.go`, `directory_linux_test.go`, `directory_darwin_test.go`, `directory_sim_test.go`, `directory_crash_test.go` as justified by applicability. Reuse existing `meta_test.go`, `disk_test.go`, crash helpers and `metatesting`. Add narrowly scoped guard templates and a host checker under `.ci/gs01-guards/`; owning kernel/hook unit checks may reuse existing fixtures. Do not run or repurpose the existing internal/candidate large test entry points. The new directory_reference_linux_test.go uses linux && !sim; directory_sim_test.go and translated directory_crash_test.go use sim; common directory_descriptors_test.go remains untagged; directory_linux_test.go is shared Linux/sim only where contracts match; directory_darwin_test.go carries native Darwin expectations.

| Planned category | Selection contract |
| --- | --- |
| Common descriptor/Root cases | Untagged; native Linux, native Darwin and simulation agree |
| Native Linux reference for model divergences | linux && !sim, even if the filename ends in _linux_test.go |
| Linux semantics also required of simulation | Untagged _linux_test.go, intentionally selected in both |
| Simulator-only limits/rejections | sim |
| Native Darwin expectations | _darwin_test.go |

- [x] Split the prototype's assertions into the appropriate categories; keep common files natively runnable on macOS. setupRealDisk provides fixture isolation, not build exclusion. Add a selected-file inventory check for the contexts rather than relying on filenames alone.
- [x] Cover all supported/limited Root rows: nested roots, Open/OpenFile/Create/ReadFile/WriteFile, Stat/Lstat, Mkdir/MkdirAll, Remove/RemoveAll, Rename, Symlink/Readlink, metadata follow policy, FS wrapper behavior, Close/Name and closed-root errors. Use modest fixtures within the declared listing bound for positive tests.
- [x] Exercise raw Openat → NewFile blocking I/O and Root.Open(".") → Fd → directory Openat → NewFile → Sync → Close on both Go families. Test cwd independence and renamed/replaced/unlinked handles through the actual standard-library/x/sys paths.
- [x] Test valid in-root relative links, dangling/cyclic links and attempted absolute/.. escapes. Separately prove raw openat can follow an intermediate link outside its supplied directory when the syscall contract permits it; Root confinement must still reject the corresponding escape.
- [x] Add native reference tables for terminal slash/dot/dotdot/root, empty name/bad FD, target validation ordering, CREATE|DIRECTORY, exclusive/dangling creation, TRUNC|RDONLY write permission, EXCL without CREATE, replacement permissions/types, removed-directory dotdot and directory seek. Record observed platform differences; preserve only the target's declared simulator divergences.
- [x] Add translated fs.CrashStates regressions matching Task 3's positive/negative topology and symlink-persistence cases. Own/close iterators and snapshots; use a small MaxStates and fail on enumeration-limit errors rather than treating truncation as complete evidence.
- [x] Keep expected-fatal cases outside the positive package registry. The focused guard checker materializes only its small templates in an isolated source directory, invokes the normal launcher for rt or rt/fs, and verifies nonzero failure, the required diagnostic and absence of returned/recovered sentinels. Do not invoke internal-candidate, matrix runners or restricted suites.
- [x] Guard cases: rt-only os.Symlink/os.Readlink/os.OpenRoot in init/test and ignored-error/recover contexts; fs-enabled Root.Link and Root.Chtimes with valid arguments; overflow after a modest directory fixture. Verify feature/operation attribution, nonzero caller identity and no host-file effects. The host checker succeeds only for the intended expected failure, never compilation failure or zero test selection.
- [x] Reuse the existing fs TestMetaDeterministic/CheckSeeds; do not add a redundant repeat harness. Run them through matching prepared workers during ordinary verification, not bare native go test ./... against stale artifacts.

#### Verification

- [x] Run relevant native/common and translated tests under both compilers (V3/V4); check real selected test names and failure modes.
- [x] Run only the focused guard checker (V5), checking its case report and expected diagnostic for every case. Its generated fixtures are not included as expected passes in ordinary positive suites.
- [x] Record missing native OS/architecture runs for Task 8. Do not mark a native reference case passed because its simulated analogue passed.

### Task 6: Prepare the pinned gquorum harness and require journal gate A

**Depends on:** Tasks 1–5. **Estimated authored change:** 400–1,100 lines, principally isolated preparation and manifest tests.

**Target references:** [Consumer acceptance gates](../brainstorms/20261025-real-directory-descriptors-target.md#consumer-acceptance-gates--simulation-and-native-evidence), [Disposable consumer harness](../brainstorms/20261025-real-directory-descriptors-target.md#disposable-consumer-harness), agreed evidence matrix and execution boundary. Peer Thread 2 supplies pinned consumer paths and compiler applicability.

**Scope extension approved by the user, 2026-10-25:** “Include the translator fix in Task 6.” Repair indirect test-dependency classification, graph handling and typed reload; add focused regressions and rerun unchanged consumer gate A. Preserve consumer source/assertions, replay identity, package boundaries and native Darwin/Linux compatibility.

- [x] Fix and test indirect test-dependency translation, including full production declarations/globals, owning-test-context typed reload and cache/dependency identity.

**Files:** new host-only `.ci/gs01-consumer/` preparation helper and helper tests; generated copies/manifests/bootstrap under ignored `.gosim/gs01-consumer/`. No changes to the real consumer repository, production consumer bodies or existing test assertions. The helper is ordinary preparation tooling, not certification or a new library API. Use package main with its own focused Go tests under .ci/gs01-consumer; no nested module or new production dependency is needed. The normal root ./... traversal does not discover dot-prefixed tooling directories, so V6 explicitly runs its helper tests.

- [x] Prepare from local Git objects at full revision `10c78f90aab45267d4dc29265c59f410c1d87c40`; verify that object and repository identity. Do not use the current consumer worktree HEAD or copy its untracked edits. Require a fresh destination and refuse overwrite.
- [x] Materialize two independent copies: original native functional source and simulation-only harness. Use safe archive extraction, keeping output inside its destination. Do not mutate either original checkout.
- [x] In the simulation copy only, parse build constraints and replace the functional atom with sim while preserving negation/boolean structure. Rewrite consistent legacy +build constraints if present. Keep test bodies and all non-test production source byte-identical. Verify the assumption that no compiled non-test source uses functional before proceeding; surface a changed baseline instead of silently transforming production code.
- [x] Add only the target-approved sim temporary-directory bootstrap, schema-2 feature configuration and Gosim module requirement/replacement selecting the exact implementation checkout. Keep gquorum's Go 1.26 directive and dependency graph requirements; do not force Go 1.25, substitute Darwin source in simulation, or downgrade x/sys to conceal a compatibility mismatch.
- [x] Record the manifest fields below, including the precise added files and module/config differences. Verify production/body hashes before and after simulation execution. Compare original Linux functional selection with transformed Linux sim selection, including when the host is Darwin; separately retain native Darwin selection. Do not require identical selected source sets between different native OSes or substitute Linux files into the native Darwin copy.
- [x] Unit-test atom substitution for positive, negative and combined expressions; matching legacy constraints; unchanged bodies; selection equivalence; fresh-output refusal; and manifest integrity. These tests use synthetic local fixtures, not consumer test execution or network access.
- [x] Run gate A with Go 1.26 and exact names/seeds, retaining t.Parallel and actual descriptor operations. Inspect execution to confirm the static call-graph assumption; first-use third-party failures must be diagnosed by their actual route/declaration, not called a statx blocker in advance.
- [x] Require all four A tests to pass. If a new required capability is outside the accepted target, record the concrete blocker and return it for scope resolution; do not replace production operations, disable checks, or count compilation/skip/abort as acceptance.
- [x] Keep B/C tests in the copied sources and manifest but report their simulation dependencies separately. Select any supported staging shapes by exact subtest name; never label the whole staging test passed with hardlink omitted. If a required unchanged fixture exceeds the listing limit, report the pagination dependency instead of shrinking it.

The harness manifest is evidence, not a new runtime schema. It records: schema version; consumer repository/revision; exact Go version; Gosim source identity/path; native/sim copy paths; original and final file hashes; rewritten constraint expressions; test-body/production hashes; added bootstrap/config/module edits; selected files/tests/subtests; features/seeds; command log paths; exit status and passed/failed/skipped/blocked outcomes. Add no secrets or host credentials.

Gate A exact names:

- `TestApprovalPublicationFaultsPreserveSourceAndGuard`
- `TestPhasePublicationReopenAndConcurrentReaders`
- `TestCancellationHistoryAndMatchingPointerClearing`
- `TestResetJournalHasNoReplaySeedsAndRecordsExactJoin`

#### Verification

- [x] Run preparation-helper tests with host linkname flags and inspect the generated manifest (V6).
- [x] Run simulated A on seeds 1–3 with Go 1.26.5 where available, or a recorded supported 1.26 patch with the baseline difference explicit (V6). Require real test execution and unchanged assertions/production code.
- [x] Verify B/C and lock-exclusion limitations are reported separately. The journal boundary-error tests do not replace translated crash-state enumeration.

### Task 7: Complete capability metadata, architectural documentation and generated checks

**Depends on:** Tasks 1–6. **Estimated authored change:** 250–650 lines, with generated outputs additional.

**Target references:** [Typed transport, hooks and metadata](../brainstorms/20261025-real-directory-descriptors-target.md#typed-transport-hooks-and-metadata), [Complete Root surface](../brainstorms/20261025-real-directory-descriptors-target.md#complete-root-surface-under-go-125-and-126), accepted limitations, consumer gates and NOT CONSIDERED & TODO.

**Files:** `internal/fs/capabilities.go`, `internal/compat/profiles/go125.json` and actual required descendant deltas only; generated revisions/guards/adapters; `README.md`, `docs/design.md`, `docs/features.md`, `docs/syscalls.md`; owning capability/metadata tests. Do not rewrite the historical audit as if it described today's execution evidence.

- [x] Remove stale AT_FDCWD-only/no-symlink statements for implemented paths. Describe actual real-FD, symlink/final-policy, metadata and Root coverage without claiming full POSIX compatibility.
- [x] Explicitly retain integer-fcntl/NONBLOCK, early read-only precedence, limited stat metadata, absent hard links/mounts/real locks, bounded directory listing with fatal overflow, limited directory seek including SEEK_CUR, and lack of procfs/special-file fallbacks.
- [x] Map every Root row to its underlying implemented or rejected capability. Link and Chtimes remain fatal when reached with valid inputs. Do not introduce redundant Root hooks only to fill a table.
- [x] Update syscall catalogue documentation/counts (36 rt/fs, 45 full) and preserve monitoring's directory-context ownership rule and real failure accounting.
- [x] Document the single inode-bound resolver, narrow topology graph and its precise fsync effects, including the positive move-E-into-D exception. Preserve the distinction between model ordering and hardware power-loss guarantees.
- [x] Document mandatory native Linux/macOS evidence, gquorum's Go 1.26 applicability and simulation-only B/C blockers. Do not present existing CI configuration or historical prototype passes as fresh qualification.
- [x] Add appropriate `TODO(not planned)` comments for exclusions where the implementation would otherwise imply support. Prefer explicit existing rejection paths; do not add warning noise or success stubs for unsupported features.
- [x] Regenerate all changed adapters/revisions/profiles/stringers as appropriate, inspect origin changes, and run independent generation checks for both recorded baselines. Historical evidence remains unchanged.

#### Verification

- [x] Run capability/spec/profile/routegen tests and both generator checks (V1/V2); verify declarations, metadata and source are mutually consistent.
- [x] Run the formatting check (V7). Lint only changed cmd/examples files if any are actually introduced; do not expand lint scope into runtime/providers/generated code.
- [x] Review documentation against implemented behavior and explicit incomplete evidence, including native/simulation file selection.

### Task 8: Execute Darwin ARM64 validation and record deferred native qualification

**Depends on:** Tasks 1–7; current execution is Darwin ARM64 under the user steering recorded above. **Authored changes:** normally none except fixes justified by failures and plan evidence entries.

**Target references:** [Mandatory portability requirement](../brainstorms/20261025-real-directory-descriptors-target.md#mandatory-portability-requirement), [Verification specification](../brainstorms/20261025-real-directory-descriptors-target.md#verification-specification), [Agreed evidence matrix](../brainstorms/20261025-real-directory-descriptors-target.md#agreed-nativetranslated-evidence-matrix), consumer gates and design completion boundary.

- [x] Run ordinary make test under Go 1.25.14 and 1.26.9 with separate report roots (V7). Read report.md and failing package logs; distinguish skipped large suites from passed tests. Record tested snapshot identity and any concurrent-checkout-edit warning.
- [x] On native Darwin ARM64, execute the shared/public and Darwin-specific cases under both Gosim families, plus translated Linux regressions separately. Require actual unskipped native tests for handle rename/replacement/unlink/close, Root links/confinement, NewFile/Sync and complete listings.
- [x] Record native Linux ARM64 checks as deferred for this Darwin-only run. Preserve the native-only reference/shared fixtures for later CI; do not claim simulated Linux execution on Darwin is a native Linux result.
- [x] Record Linux AMD64 execution as explicitly deferred by the user. Retain AMD64 source/constants/generation checks, without presenting them as executed AMD64 semantics or certification.
- [x] Record source-derived Linux errno and held-removed-directory dotdot expectations as awaiting native Linux qualification. Implement the verified target and retain its reference fixtures; do not weaken native expectations to match the prototype or promote cross-compilation/Darwin results to Linux execution.
- [x] Run native pinned gquorum A plus supported B/C archive/alias and relevant descriptor cases on Darwin ARM64 with Go 1.26 and original functional tags (V6); record native Linux consumer runs as deferred. Preserve native Fstatfs/F_FULLFSYNC paths; do not install simulator fcntl behavior or batch limits on native execution.
- [x] Confirm the final simulated A result belongs to the final source identity; rerun it if relevant source/dependency changes since Task 6 invalidate that evidence. Do not rerun unchanged successful checks without a reason.
- [x] Collect exact compiler/OS/kernel/architecture/filesystem, command/test selections, source identities and report paths. Separate model crash outcomes, native reopen/error tests and actual hardware durability claims. Native results do not satisfy simulated B/C when its capability is still missing.
- [x] Fix failures within the accepted target and revalidate the affected checks. Log incidental changes, external blockers and unimplemented gaps. Do not claim the full issue closed while its archive gate or mandatory native evidence is outstanding.

#### Verification

- [x] Both ordinary suite reports have no unaccounted failures/errors and all relevant skipped/missing cases are explicitly listed.
- [x] Every required Darwin ARM64 row has an actual result. Deferred native Linux/AMD64 rows remain visibly deferred by user scope, never counted as passes or removed from future qualification.
- [x] No race/large/certification or extra infrastructure was invoked through an alias, environment variable or helper.

### Task 9: Architectural delta verification and final wiring audit

**Depends on:** Tasks 1–8. **Estimated authored change:** corrective changes only; any material scope change requires user resolution.

**Target references:** ALL sections of the finalized target, its full method/data inventory, algorithms, failure/data flows, capability surface, exclusions and acceptance/evidence matrix. No DB/cache/schema work is applicable because the target explicitly introduces none.

- [x] Verify descriptor/provider delta against target [Provider interfaces and semantics](../brainstorms/20261025-real-directory-descriptors-target.md#provider-interfaces-and-semantics): existing shared FD ownership, correct absolute/relative base selection, renamed/replaced/unlinked handles, closed/stale errors, no hidden path handles, ENOSYS/NONBLOCK/CLOEXEC scope and read-only limitation.
- [x] Verify model delta against target [Data structures](../brainstorms/20261025-real-directory-descriptors-target.md#data-structures) and [Model method inventory](../brainstorms/20261025-real-directory-descriptors-target.md#model-method-inventory): every target inode/stat/dirent/open-option/resolver structure and method exists with the intended signatures and policies; one walker; correct lexical root/dot/dotdot; symlink metadata/lifetime/cloning; validation before mutation; no unrelated database/public API/config additions.
- [x] Verify namespace/crash delta against target [Directory topology and crash ordering](../brainstorms/20261025-real-directory-descriptors-target.md#directory-topology-and-crash-ordering): atomic rename/remove rules, inclusive Q topology reads, depStoreAs publication only for directory moves, empty-directory dependencies, no forbidden broad edges/fsync roots, immutable operation inputs and all positive/negative durability scenarios.
- [x] Verify listing/transport delta against target [Directory listing must not return false success](../brainstorms/20261025-real-directory-descriptors-target.md#directory-listing-must-not-return-false-success): complete preflight, untouched output/cursor on overflow, typed Failure preservation, correct caller attribution, non-recoverable abort, rewind and explicit other-seek limitation, with no change to ordinary errno or raw/feature rejection.
- [x] Verify compatibility/generation delta against target [Typed transport, hooks and metadata](../brainstorms/20261025-real-directory-descriptors-target.md#typed-transport-hooks-and-metadata): both Go families and both source architectures, baseline inheritance, private Stat_t selector conversion, stable appended route IDs, correct feature gates/borrowed views/race annotations, generated-current checks and no manufactured evidence.
- [x] Verify monitoring/capability/docs delta against target [Typed transport, hooks and metadata](../brainstorms/20261025-real-directory-descriptors-target.md#typed-transport-hooks-and-metadata) and [Complete Root surface](../brainstorms/20261025-real-directory-descriptors-target.md#complete-root-surface-under-go-125-and-126): new names/counts and unchanged attribution rules; Root method table matches actual underlying support/rejection; all exclusions and deliberate divergences remain explicit.
- [x] Verify test/evidence delta against target [Verification specification](../brainstorms/20261025-real-directory-descriptors-target.md#verification-specification): exact native/simulated build constraints, real public APIs, deterministic repeat and epoch cleanup, bounded CrashStates, fatal guard fixtures, native Linux/macOS evidence and correct compiler applicability. Confirm reference-only Linux cases cannot run in simulation accidentally.
- [x] Verify consumer delta against target [Consumer acceptance gates](../brainstorms/20261025-real-directory-descriptors-target.md#consumer-acceptance-gates--simulation-and-native-evidence) and [Disposable consumer harness](../brainstorms/20261025-real-directory-descriptors-target.md#disposable-consumer-harness): pinned original/body hashes, both tag polarities, bootstrap/module/config manifest, gate A execution, native B/C, simulation B/C and lock limits, no production path substitution or weakened assertions.
- [x] Review every entry in NOT IMPLEMENTED GAPS. Fix target mismatches within scope and rerun affected checks, or record an explicit user-approved deferral. Mark still-unresolved items with `[_]`; do not check this task complete while a required local implementation gap remains. Keep external evidence blockers and agreed GS-03 exclusions distinct from missing code.
- [x] Verify **all methods/components are wired for actual use**: composition binds the real FS provider, actual os/syscall/x/sys calls reach the intended selected hooks and typed requests, dispatcher calls the new FileOps, model changes occur through live operations, overflow reaches AbortRun, controls/lifecycle release resources, and native consumer paths remain native. Require end-to-end evidence, not merely declarations or unit-test-only reachability.

#### Verification

Review the final diff and all referenced reports against the target. Confirm the last checklist item above is the real-entrypoint wiring audit. Report source status, completed plan scope, remaining external gates and test evidence without equating this increment with complete issue acceptance. Do not create a release, merge or publish as a side effect.

## Validation Commands

These commands are for a future directly requested execution task. **None was run during planning.** Run from the repository root unless stated otherwise, one compiler process at a time for source mutation/generation. Preserve host linkname flags. Record command exit status and reject zero-test selections.

### V1 — focused native units

Run the owning subset after each task; this full list is appropriate once all touched packages are integrated. Repeat with both supported baseline compilers. The helper packages are added only in their owning tasks.

```sh
GOTOOLCHAIN=go1.25.14 go test -ldflags=-checklinkname=0 -tags=linkname ./internal/contracts ./internal/kernel ./internal/fs ./internal/fs/model ./internal/routegen ./internal/compat/spec ./internal/compat/profiles
GOTOOLCHAIN=go1.26.9 go test -ldflags=-checklinkname=0 -tags=linkname ./internal/contracts ./internal/kernel ./internal/fs ./internal/fs/model ./internal/routegen ./internal/compat/spec ./internal/compat/profiles
```

These are package units, not the prepared simulation metatest path. Additional hook tests run only where their native build constraints apply.

### V2 — generation and exact-family checks

Use profiles generation before/after changed compatibility sources and adapters. For declaration changes requiring an origin update, add `-refresh-origin` only to the matching family's write invocation after source-contract validation. It is not combined with `-check`. Do not change the recorded baseline or evidence to bypass an error.

```sh
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen profiles -root=.
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=.
GOTOOLCHAIN=go1.26.9 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=.
GOTOOLCHAIN=go1.25.14 go generate ./internal/fs/model
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen profiles -root=.
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=. -check
GOTOOLCHAIN=go1.26.9 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=. -check
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen profiles -root=. -check
```

### V3 — focused translated positive regressions

```sh
GOTOOLCHAIN=go1.25.14 go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim
GOTOOLCHAIN=go1.25.14 ./.gosim/gosimtool test -features rt,fs -seeds 1-3 -run '^(TestDirectory|TestRoot|TestGS01|TestSyscallMonitorFilesystem)' ./internal/tests/fs
GOTOOLCHAIN=go1.26.9 ./.gosim/gosimtool test -features rt,fs -seeds 1-3 -run '^(TestDirectory|TestRoot|TestGS01|TestSyscallMonitorFilesystem)' ./internal/tests/fs
```

Keep the selector aligned with final test names, including translated crash regressions. Expected-fatal fixtures must not be put in this positive package run. Source selection is Linux on both native hosts.

### V4 — focused native behavior and selection evidence

Run on each actual available native Linux/Darwin worker; command execution on Darwin is not Linux reference evidence.

```sh
GOTOOLCHAIN=go1.25.14 go test -ldflags=-checklinkname=0 -tags=linkname -count=1 -v -run '^(TestDirectory|TestRoot|TestGS01)' ./internal/tests/fs
GOTOOLCHAIN=go1.26.9 go test -ldflags=-checklinkname=0 -tags=linkname -count=1 -v -run '^(TestDirectory|TestRoot|TestGS01)' ./internal/tests/fs
```

Do not select the native metatests here; V7 supplies their matching artifacts. Record selected native test files and names. `go list` with GOOS/tags is permitted to inspect selection, but is compile/source evidence only. No native Darwin test may be silently replaced by a Linux-source simulation pass.

### V5 — focused expected-failure checker

Task 5 creates `.ci/gs01-guards/check.py` with explicit `--gosim`, `--toolchain` and `--reports` arguments. It invokes only its named fixtures through the normal launcher, validates each expected failure, and emits case results plus logs. Its own nonzero status means a missing/wrong failure, harness/build error, host effect or zero-case execution.

```sh
python3 -B .ci/gs01-guards/check.py --gosim .gosim/gosimtool --toolchain go1.25.14 --reports test-reports/gs01/guards/go125
python3 -B .ci/gs01-guards/check.py --gosim .gosim/gosimtool --toolchain go1.26.9 --reports test-reports/gs01/guards/go126
```

This helper must not call internal-candidate, make test-large, race or certification; no environment setting may enable them. It is not implemented or executed during planning.

### V6 — pinned consumer preparation and exact acceptance selection

Task 6 creates a host-only helper at `.ci/gs01-consumer` with `-source`, `-revision`, `-output`, and `-gosim-source` arguments. Output contains `native/`, `sim/` and its preparation manifest. Supply a real local consumer repository containing the pinned object; the paths below are shell variables, not assumed infrastructure.

```sh
GOTOOLCHAIN=go1.26.9 go test -ldflags=-checklinkname=0 -tags=linkname ./.ci/gs01-consumer
GOTOOLCHAIN=go1.26.5 go run -ldflags=-checklinkname=0 -tags=linkname ./.ci/gs01-consumer -source "$GS01_CONSUMER_REPO" -revision 10c78f90aab45267d4dc29265c59f410c1d87c40 -output "$GS01_CONSUMER_OUTPUT" -gosim-source "$PWD"
GOTOOLCHAIN=go1.26.5 ./.gosim/gosimtool -C "$GS01_CONSUMER_OUTPUT/sim" test -features rt,fs -seeds 1-3 -run '^(TestApprovalPublicationFaultsPreserveSourceAndGuard|TestPhasePublicationReopenAndConcurrentReaders|TestCancellationHistoryAndMatchingPointerClearing|TestResetJournalHasNoReplaySeedsAndRecordsExactJoin)$' ./internal/recovery/disk
```

For native consumer qualification, run in the untouched native copy on each actual Linux/Darwin worker:

```sh
GOTOOLCHAIN=go1.26.5 go -C "$GS01_CONSUMER_OUTPUT/native" test -ldflags=-checklinkname=0 -tags=functional,linkname -count=1 -v -run '^(TestApprovalPublicationFaultsPreserveSourceAndGuard|TestPhasePublicationReopenAndConcurrentReaders|TestCancellationHistoryAndMatchingPointerClearing|TestResetJournalHasNoReplaySeedsAndRecordsExactJoin|TestArchiveMoveFaultsPreserveEvidenceAndRequireJoinedSync|TestArchiveMoveNeverReplacesExistingDestination|TestApprovalStagingDoesNotHideForeignContents)$' ./internal/recovery/disk
```

The consumer's functional tag is its native test-selection contract; it is not the Gosim large suite. Inspect that consumer's own repository instructions before future execution. Add the relevant native descriptor/mount cases from the pinned inventory; a real mount-creation probe needs its own suitable environment and must not be started implicitly. An alternative supported Go 1.26 patch must be recorded instead of silently relabeling 1.26.5 results.

### V7 — ordinary broad validation and reporting

On each approved actual native worker, run separate roots for each compiler. The following local commands neither create a Linux runner nor certify any tuple:

```sh
./.ci/gofmt check
make test GOSIM_TOOLCHAIN=go1.25.14 TEST_REPORTS_DIR="$PWD/test-reports/gs01/go125"
make test GOSIM_TOOLCHAIN=go1.26.9 TEST_REPORTS_DIR="$PWD/test-reports/gs01/go126"
```

Read `unit/latest/report.md`, `timeline.md`/JSON and failing per-package logs beneath each chosen root. Verify ordinary metatests and deterministic-repeat cases actually run with matching snapshots. Restricted suites remain skipped and are reported as such. External native evidence additionally records source hash, exact command, exit code, compiler, OS/kernel, architecture and filesystem.

## Acceptance and evidence ledger

Updated after Task 9. Final executable inputs are production source f4d3204 plus the audited portable test additions, bound by snapshot digest `75c95a5c22c17a07412e63f93549005f1e87180f50b00fc934a27ddba1561d8b`. Earlier prototype/Task 8 runs remain tied to their own snapshots.

| Gate | Required result | Current state |
| --- | --- | --- |
| Model/provider/kernel/routegen checks | Target behavior and errors, both families | Passed focused checks and both final ordinary suites |
| Translated public APIs and crash/determinism tests | Both families, complete identity and repeat evidence | 17 focused top-level tests × 3 seeds per family passed; ordinary FS determinism and five-seed metatests passed |
| Fatal feature/raw/Root/overflow boundaries | Correct failure, attribution, no recovery/host effects | All 9 focused failure fixtures passed per family on final source; provider/kernel assertions passed |
| Native Darwin ARM64 | Both Gosim families, native common/Darwin behavior | 9 focused tests passed per family; ordinary suites passed on macOS 27.0 / Darwin 27.0.0 / APFS |
| Native Linux ARM64 | Both families, reference and shared behavior | Deferred by current Darwin-only execution scope; no native result |
| Native Linux AMD64 | Focused native/translated evidence, both families | Explicitly user-deferred to future testing |
| Simulated consumer A | Four exact journal tests on pinned Go 1.26 consumer | Passed four unchanged tests × seeds 1–3 on Go 1.26.5 after the approved translator repair; 12 top-level and 36 total pass lines, zero skips. Task 9 repeated the result on final source; manifest `.gosim/gs01-consumer/go1265-v6/manifest.json`. |
| Simulated consumer B | Archive/artifact mount identity | Depends on GS-03 increment 2 |
| Simulated consumer C | Staging hardlink safety | Depends on GS-03 increment 1 |
| Native consumer A/B/C | Original source/tags, Go 1.26 | 13 selected tests / 33 cases including subtests passed, zero skips, original source/functional tags, Go 1.26.5 on Darwin ARM64; Linux deferred |
| Real locking/exclusion | Ownership/contention/lifetime | GS-02; journal test passes do not qualify it |

Plan implementation completion, native qualification and full issue #12 acceptance are different claims. Keep all three explicit. Required missing evidence leaves the relevant task/gate open. Approved GS-03 exclusions remain dependencies even if every GS-01 local test passes.

## NOT CONSIDERED & TODO

- [postponed; GS-03/#14] Hard links, complete alias/link-count safety, Statx and multiple mounts/bind mounts/cross-device/mount replacement behavior. Keep real unsupported paths; do not add constant-ID or successful link stubs.
- [postponed; GS-02/#13] Real flock/OFD/process-lock contention and shared-volume/process semantics. Existing inherited lock stubs remain explicitly classified; no ownership claim is made here.
- [postponed; GS-05/#16] Resumable directory pagination and richer dirent inode/cookie fields. GS-01 must implement fatal overflow and rewind correctness. A required fixture exceeding the bound creates a dependency, not permission to truncate or reduce it.
- [postponed] Dup/shared open descriptions, special files, O_PATH/O_TMPFILE/openat2, supplemental groups/ACLs/sticky-bit completeness, native process execution and procfs emulation. Retain explicit rejection and accurate TODO(not planned) comments where relevant.
- [postponed] General directory garbage collection and complete POSIX directory link counts. Retained detached directory objects must remain safe and be released at filesystem teardown.
- [postponed] Linux-exact read-only-mount error precedence and integer status-flag support. Preserve the accepted limited gate/ENOSYS behavior; native references remain separate.
- [postponed] Exhaustive multiply-invalid syscall precedence outside the target's named cases. Named flags/terminal/descriptor/no-mutation cases and removed-directory dotdot still require native qualification.
- [user-deferred] Native Linux ARM64/AMD64 execution: current execution is Darwin ARM64 only. Keep future qualification gaps visible; no cross-architecture/infrastructure workaround is authorized.

## NOT PLANNED CHANGES

None during planning. The user explicitly extended Task 6 to repair indirect test-dependency translation after the unchanged consumer exposed it. `internal/translate/packages.go` now distinguishes indirect production variants from direct test additions; `internal/translate/main.go` reloads indirect misses with their owning test context. `internal/translate/indirect_test.go` covers classification, graph/cache identity and native/translated batch/cache behavior under both families. The Task 6 commit contains the focused diff. No consumer bodies or production operations changed.

Task 9 audit correction: added the exact portable Root.Open(".") → Fd → directory Openat → NewFile → Sync → Close chain, including renamed inode identity, independent child lifetime, repeated Root.Close and closed-handle errors. The consumer had covered it under Go 1.26, but the original Gosim fixtures did not explicitly cover that complete chain under both families. Production behavior is unchanged. Refreshed ordinary, native, translated, fatal-boundary and consumer checks passed after both portable test corrections; evidence is in the final audit.

## NOT IMPLEMENTED GAPS

This is an execution ledger, not a list of waived requirements. Use `[_]` for a still-unresolved item; replace it with a recorded fix/evidence reference only when resolved.

- [x] Tasks 1–9 are implemented, validated and audited within the user-approved Darwin ARM64 scope. The final audit closed the explicit both-family Root directory/NewFile-chain and absolute link/metadata-call coverage gaps. No local implementation or wiring gap remains; see [the audit](../reports/20261025-gs01-architecture-audit.md).
- [_] [user-deferred] Native Linux ARM64/AMD64 qualification awaits future testing; the user limited this execution to Darwin ARM64. This is a declared qualification gap, not a passing result or a current-run infrastructure blocker.
- [x] Darwin qualification and final-source consumer A execution passed in Task 8. Native Linux errno/removed-directory-dotdot qualification remains user-deferred; source review is not native execution evidence.
- [_] Simulated consumer B/C requires GS-03 capabilities by the accepted scope. Preserve this dependency in final reporting and do not claim full issue #12 acceptance prematurely.

- [x] Task 6 scope resolution and repair: `internal/translate/packages.go` previously classified an indirect recompiled dependency (`gquorum [disk.test]`) as its own test variant, causing the graph to reject its absent base. The original minimal parent/leaf/external-test fixture passed natively and reproduced the same simulation error before the repair. The user approved the repair on 2026-10-25. Production classification and owning-context typed reload now pass focused regressions under both families and unchanged gquorum A under Go 1.26.5. No consumer assertions/imports were changed. See `.ci/gs01-consumer/README.md` and `.gosim/validation/gs01/task6/`.

## Progress log

- 2026-10-25: Created the nine-task plan after the user selected “Use nine focused tasks (Recommended).” Read the finalized target, repository guidance, package/test/generation paths and prior peer findings. Verified documentation is in the project repository. No implementation changes, generation or test execution performed.

Future entries must name the completed task, exact checks/results, source identity and remaining evidence/gaps. Checkbox completion without supporting evidence is invalid.

- 2026-10-25 Task 1: Reconciled typed routes/hooks and approved descriptor-flag scope. Seven native packages passed under go1.25.14 and go1.26.9; both-family amd64/arm64 generation checks and profiles check passed; four translated tests × three seeds passed for each compiler. Logs: `.gosim/validation/gs01/task1/`. Includes the preserved model/provider draft required by the live interfaces; detailed path/topology correction remains Tasks 2–3.

- 2026-10-25 Task 2: Completed final-component policies, one shared prefix/final symlink budget, target metadata/lifetime and inode-base validation. Added nine focused model/provider regressions. fs/model and fs native suites passed under both compilers; three native Darwin public tests and four translated tests × three seeds passed per compiler. Formatting/diff checks clean. Logs: `.gosim/validation/gs01/task2/`; Linux-native errno/dotdot qualification remains user-deferred.

- 2026-10-25 Task 3: Added minimal directory topology and empty-directory dependencies, final mutation validation, owned pending-operation inputs and invariant cycle detection. Both compiler-family model/provider suites passed, including bounded crash enumeration and positive/negative fsync cases; formatting/diff checks clean. Logs: `.gosim/validation/gs01/task3/`.

- 2026-10-25 Task 4: Whole-directory preflight now fails fatally without partial output; directory rewind and typed-failure transport/attribution are verified. fs, kernel and routegen passed under both compilers, along with both-family generation/profile checks and formatting. Logs: `.gosim/validation/gs01/task4/`.

- 2026-10-25 Task 5: Added portable/native-only/sim-only API and CrashStates regressions, exact file-selection inventory, and a dedicated nine-case fatal guard checker. Both compiler families passed native Darwin tests, 16 translated top-level tests × three seeds (plus subtests), existing determinism/seed metatests, and all guard cases. Linux-native references remain user-deferred. Logs: `.gosim/validation/gs01/task5/`, `test-reports/gs01/guards/`.

- 2026-10-25 Task 6 (incomplete): Added pinned native/sim preparation, hash/selection verification and strict gate-A execution checking. Four host helper tests passed on go1.26.9; pinned preparation and post-failure integrity verification passed on go1.26.5. Gate A failed before test execution at the translator missing-base check; independently reproduced with a minimal rt-only fixture whose two tests pass natively. No A/B/C consumer passes are claimed. Scope resolution is required by Task 6 before a translator repair; Tasks 7–9 were not started.

- 2026-10-25 Task 6 completed: The user-approved translator repair passed three focused regression functions and related graph/cache checks under go1.25.14 and go1.26.9. Four unchanged gquorum A tests passed seeds 1–3 under go1.26.5 (12 top-level / 36 total pass lines, no skips), with source/body integrity verified afterward. Manifest: `.gosim/gs01-consumer/go1265-v4/manifest.json`; logs: `.gosim/validation/gs01/task6/`. Simulation B/C and lock-exclusion limits remain separate.

- 2026-10-25 Task 7: Updated provider/Root semantic capabilities, shared-family syscall reasons, current architecture/features/README and catalogue counts. Regenerated profile revisions; both-family AMD64/ARM64 adapter checks and profiles check passed without origin/adapter/evidence changes. FS, spec, profiles and routegen packages passed under both compilers; the new external-package metadata test needed a spec.Capability qualification, then passed. Formatting/diff checks clean. Logs: `.gosim/validation/gs01/task7/`.

- 2026-10-25 Task 8: Both final ordinary suites passed (go1.25.14: 2,235 pass / 18 skip; go1.26.9: 2,251 pass / 16 skip; zero fail/error). Both tested input digest `1460d097c791c93aded6b95b06ebff16607c0296cb5b12452b4f4b2c3052adcc`, with no checkout changes. Final native FS (8 per family), translated FS (16 × 3 seeds per family), nine failure fixtures per family, simulated consumer A (4 × 3 seeds) and native consumer A/B/C plus descriptor/artifact cases (13 top-level / 33 total) passed. Host Darwin ARM64/APFS; native Linux/AMD64 deferred, simulation B/C and locks remain external dependencies. Evidence `.gosim/validation/gs01/task8/summary.json`, `test-reports/gs01/{go125,go126}/unit/latest/`, and final consumer v5 manifest.

- 2026-10-25 Task 9: Completed the target-by-target architecture and real-entrypoint wiring audit. Added missing portable directory NewFile-chain and absolute link/metadata regressions, then refreshed source-bound validation. Final ordinary results: go1.25.14 2,240 pass / 18 skip; go1.26.9 2,256 pass / 16 skip; zero fail/error. Both snapshots bind `75c95a5c22c17a07412e63f93549005f1e87180f50b00fc934a27ddba1561d8b` with no checkout changes. Nine native tests, 17 translated tests × three seeds and nine fatal fixtures passed per family; unchanged consumer A passed under Go 1.26.5. See `docs/reports/20261025-gs01-architecture-audit.md` and `.gosim/validation/gs01/task9/summary.json`. All local tasks are complete; user-deferred native Linux/AMD64 and agreed GS-03/GS-02 qualification gates remain open.
