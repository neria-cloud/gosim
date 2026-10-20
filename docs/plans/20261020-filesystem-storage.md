# Implementation Plan: Bounded and Controllable Simulated Filesystem

**Plan**: `docs/plans/20261020-filesystem-storage.md`  
**Date**: 2026-10-20  
**Project source**: `.`  
**Target architecture**: `docs/brainstorms/20261020-filesystem-storage-target.md`  
**Brainstorm log**: `docs/brainstorms/20261020-filesystem-storage.md`  
**Branch at planning**: `feat/filesystem-issues-ext`

## Goal and execution rules

Implement the target document from top to bottom. Keep the current inode tree, COW chunks, crash plan and recovery behavior. Capacity defaults to 128 MiB and is chosen before first start; resize is out of scope. `/` remains the volume root and `/app` remains the default working directory. All simulated syscalls use the Linux source ABI on either host OS. Preserve append-only route IDs, exact compiler selection, and deterministic replay for a complete build identity and seed.

Each numbered task is intended to remain below roughly 550 handwritten lines. Generated adapter/route output may exceed that count because the generator emits both supported Go families and architectures; review generated diffs separately. At the end of **every** implementation task, format touched Go files, build with `make build`, run `make test`, read `test-reports/unit/latest/report.md` and failing package logs, and fix failures before checking off the task. `make test` is the supported ordinary suite and includes unrelated packages. A focused test command may precede it. Never run race or large suites, aliases, direct equivalents, Docker/Colima, or manual certification without a later explicit user instruction. Preserve `-ldflags=-checklinkname=0 -tags=linkname` on any direct host build/test command. Do not use bare `go test ./...`.

The target's **Storage schema and invariants** is the only storage schema: no DB tables, ORM, migrations, external cache, cache keys or TTLs are introduced. The target's **Architectural view**, **Decisions**, **Data flows** and **Method decision logic** govern all tasks. If implementation evidence requires a deviation, record it under **NOT PLANNED CHANGES** and update the target document before considering the task complete.

Repository note: `.gitignore` excludes `docs/brainstorms/`, and this checkout has no separate nested Git repository there. The target and log exist in this worktree, but a future execution in another checkout needs those two local files copied or intentionally tracked. The plan file itself is not ignored.

## Resolved design review

| Point | Resolution used in this plan | Source |
| --- | --- | --- |
| Full-path KV wording versus inode tree | Keep inode tree; paths remain lookup inputs | Brainstorm log §User choices item 8; target §Decisions |
| Root versus process cwd | Volume root `/`, default cwd `/app` | Target final planning baseline; README §Filesystem recovery |
| Capacity resize request versus later answer | No resize in this change | Brainstorm log §User choices item 5; target §Input and scope |
| Delayed operations and borrowed `ByteView` | Execute once in the supervisor, clear borrowed arguments, then delay scalar completion with an epoch event | Target §Data flows item 7; `docs/design.md` §Dispatcher, descriptors and memory |
| Quota versus COW snapshots | Charge reachable/open live file blocks, not retained memory copies | Target §Storage schema and invariants |
| Crash result above target capacity | Keep the crash state, report zero free, reject further allocation until usage drops | Target §Storage schema and invariants and failure table |

## Target interfaces and data layout

These names and fields are plan checkpoints, not new decisions. Full signatures are in target §Public API and control wiring and §Model/provider methods.

```go
type Config struct {
    WorkingDir string
    CapacityBytes uint64 // 0 selects 128 MiB for old struct-literal callers
    UID, GID uint32
    Latency time.Duration
}
type StorageStats struct {
    TotalBytes, FreeBytes, UsedBytes, BlockSize uint64
    ReadOnly bool
}
type inodeMetadata struct {
    Mode, UID, GID uint32
}
type Identity struct { UID, GID uint32 }
```

The public `Config` converts explicitly to `contracts.FSConfig{WorkingDir,CapacityBytes,UID,GID,LatencyNS}`. The session machine adds `readOnly bool`; the model keeps `metadataOp` and `reserveOp` alongside existing `writeOp`, `resizeOp`, `allocOp` and `dirOp`. The route source is `internal/routegen/routes.json`; generated files are regenerated, never hand-edited. See target §Route and adapter table.

## Task list

### Task 1: Expose configuration, read-only control and storage stats

**Files**: `fs/config.go`, `fs/config_sim.go`, `fs/config_native.go`, `fs/control.go`, `fs/control_sim.go`, `fs/control_native.go`, `internal/contracts/control_fs.go`, `internal/contracts/validate.go`, `internal/contracts/service_fakes_test.go`, `internal/kernel/control_fs.go`, `internal/fs/control.go`, `internal/fs/session.go`, `internal/fs/model/filesystem.go`, `internal/routegen/routes.json`, generated kernel routes/dispatch.  
**References**: target §Decisions (initial limit/read-only/identity), §Storage schema and invariants (`StorageStats`, zero-as-default), §Public API and control wiring (public/control signatures), §Data flows 1, 2 and 6; brainstorm log §User choices 3–5.  
**Goal**: Make public controls callable with correct basic behavior and keep every package compiling before quota enforcement lands.

- [x] Add the exact `Config`, `StorageStats`, `FSConfig` and `FSStorageStats` fields from the target. Make `DefaultConfig()` return `/app`, 128 MiB, root identity and zero latency; make zero `CapacityBytes` normalize to 128 MiB in the provider for legacy struct literals. Validate nonzero block alignment, nonnegative latency and cwd before changing machine policy.
- [x] Extend `FSControl` and its test fakes with `SetReadOnly(MachineRef,bool) error` and `Stats(MachineRef)(FSStorageStats,error)`. Add matching public simulation/native functions. Native calls return `gosim.ErrNotSim`.
- [x] Add append-only `FSSetReadOnly` and `FSStats` routes to `internal/routegen/routes.json`; run the documented `cmd/gosim-gen` generation for both pinned families and both architectures, then inspect typed kernel routes/dispatch and profile revisions. Forward through `filesystemControl` with the target state guards. Do not import the provider into kernel or public controls.
- [x] Add `machine.readOnly`, preserve it across settle/restart/restore, and implement `Session.SetReadOnly` and `Session.Stats`. `Stats` selects temporary initial image, active model or retained disk per target §Data flows 2, without publishing a disk before start.
- [x] Add deterministic `Filesystem.UsedBlocks` and `Stats` that count 1 KiB allocated data blocks only for linked/open live inodes. Count `/etc/hosts`; zero free if a recovered image is over quota. Add focused stats/control tests covering prestart, running, stopped/crashed and read-only toggling.
- [x] Keep direct writes unchanged for this task but ensure the new public API reports actual current usage; the next task enforces capacity. Add `// TODO(not planned): filesystem resize is outside the configured startup capacity contract` at the policy boundary, without adding a successful resize stub.

#### Verification

- [x] Run focused `go test -ldflags=-checklinkname=0 -tags=linkname ./internal/contracts ./internal/kernel ./internal/fs ./internal/fs/model` with the selected supported compiler.
- [x] Run both exact `gosim-gen -check` baselines after generation and fix any drift.
- [x] Run `./.ci/gofmt check`, `make build`, and `make test`; inspect report totals/failures. Fix all failures before Task 2.

---

### Task 2: Enforce block capacity on writes and truncation

**Files**: `internal/fs/model/chunkedfile.go`, `internal/fs/model/filesystem.go`, `internal/fs/file.go`, `internal/fs/ops.go`, focused model/provider tests.  
**References**: target §Storage schema and invariants (block accounting), §Model/provider methods (`AllocatedBlocks`, `NewBlocksForWrite`, `Write`, `Truncate`), §Data flows 3 and failure table, §Method decision logic (`UsedBlocks`, `Write`); brainstorm log §User choices 1 and 7.  
**Goal**: Make writes atomic at capacity, preserving sparse behavior and offsets.

- [x] Implement `chunkedFile.AllocatedBlocks() uint64` and `NewBlocksForWrite(pos,size int) uint64`. Count each nil chunk touched by a nonempty write once, including a partial first/last block. Existing non-nil chunks and COW copies do not increase virtual usage.
- [x] Change model `Write` to return `(int,error)` and reject invalid or overflowing offsets before converting to `int`, making chunks or adding pending ops. Compare `newBlocks` with the remaining block budget without unsigned underflow; an over-capacity recovered image has zero headroom.
- [x] Preserve existing sparse growth from `Truncate`: extending logical length does not allocate file-data blocks, shrinking releases truncated blocks, and the last partial block is zeroed as it already is. Update method signature and callers to return errors and reject impossible sizes before `Resize`.
- [x] Update `File.writeAt`, `File.Write`, `Pwrite`, `Instance.Truncate` and open-with-truncate path to pass configured capacity and preserve atomic `ENOSPC` plus file offset semantics. Use `EROFS` checks for mutation only after Task 5 supplies the shared check; until then, add a direct policy check in `file.go` so read-only is effective immediately.
- [x] Test exactly-full, one-byte-over, sparse high-offset write, overwrite of existing blocks, zero-byte write, COW snapshot, truncate shrink/expand and a write to an unlinked open file. Assert no pending-op/file/offset mutation on `ENOSPC` and that `Stats` matches allocation.

#### Verification

- [x] Run focused model and provider tests with linkname flags, then `./.ci/gofmt check`, `make build`, `make test`; inspect report totals/failures and fix all failures.

---

### Task 3: Make fallocate reserve capacity and survive recovery

**Files**: `internal/fs/model/chunkedfile.go`, `internal/fs/model/filesystem.go`, `internal/fs/model/pendingops.go`, `internal/fs/model/crash.go`, `internal/fs/ops.go`, model/provider recovery tests.  
**References**: target §Storage schema and invariants (`reserveOp`), §Model/provider methods (`Reserve`, `Fallocate` and dependency keys), §Data flows 4 and 8, §NOT CONSIDERED & TODO (no resize); `docs/design.md` §Filesystem provider.  
**Goal**: Replace the current logical-extend-only fallocate behavior with real virtual block reservation, without adding filesystem resize.

- [ ] Implement COW-safe `Reserve(offset,length)` for previously nil chunks and a `reserveOp` that replays the reservation during flush/crash selection. Keep existing data unchanged and extend logical size only when required by the supported fallocate mode.
- [ ] Validate mode 0, nonnegative range, overflow and writability before effects. Count newly reserved blocks, reject atomically with `ENOSPC` when unavailable, then apply size/reservation operations with the right allocation/resize/write dependencies.
- [ ] Extend `retainOp`/crash cloning and pending-op dependency handling so reservations survive graceful stop, appear in eligible crash states, and remain reference-safe. Verify abort preparation does not mutate retained disks or consume random draws.
- [ ] Update capability text for fallocate once behavior is real. Test reservation of sparse holes, overlapping reservations, quota exhaustion, shrink after reservation, fsync, crash enumeration, restore and replay stability.

#### Verification

- [ ] Run focused model/recovery/provider tests with linkname flags, then `./.ci/gofmt check`, `make build`, `make test`; inspect report totals/failures and fix all failures.

---

### Task 4: Persist mode, UID and GID in inode and crash state

**Files**: `internal/fs/model/filesystem.go`, `internal/fs/model/pendingops.go`, `internal/fs/model/crash.go`, `internal/fs/model/*_test.go`, `internal/fs/ops.go`.  
**References**: target §Storage schema and invariants (`inodeMetadata`, initial modes, `metadataOp`), §Model/provider methods (`Identity`, `StatResp`, `Chmod`, `Chown`), §Data flows 5 and 8, §Method decision logic (`Chmod`).  
**Goal**: Make metadata durable and reportable before enforcing access checks.

- [ ] Add `inodeMetadata{Mode,UID,GID}` to file and directory nodes; clone it in `filesystemState.clone`. Set built-in root and `/etc` to `0755`, `/app` to `0777`, and `/etc/hosts` to `0644`, all UID/GID 0. Add initial metadata to `allocOp` for created files/directories.
- [ ] Add `metadataOp{inode,value}` and `depKeyMetadata`; order it after inode allocation and between successive metadata writes. Extend pending-op snapshots/retention/replay, `FreezeCrash`, `FlushEverything` and `Sync` coverage so metadata follows existing durability semantics.
- [ ] Expand `StatResp` and `statResult` with permission bits, UID, GID and link count while preserving file type bits. Keep unmodeled timestamps zero and capability descriptions honest.
- [ ] Implement model `Chmod` and `Chown` with owner/root and root-only authorization, `-1` preservation for chown, and no-op handling that does not create an unnecessary pending op. Add tests for root/non-root ownership, fstat/stat visibility, sync, crash subsets and restored metadata.

#### Verification

- [ ] Run focused model/recovery/provider tests with linkname flags, then `./.ci/gofmt check`, `make build`, `make test`; inspect report totals/failures and fix all failures.

---

### Task 5: Enforce identity and read-only mode on all mutations

**Files**: `internal/fs/model/filesystem.go`, `internal/fs/ops.go`, `internal/fs/file.go`, `internal/fs/control.go`, `internal/fs/session.go`, focused provider/model tests.  
**References**: target §Storage schema and invariants (DAC rules, fixed umask), §Model/provider methods (permission-aware path methods and `CheckAccess`), §Data flows 3, 5 and 6, failure table, §Method decision logic (`checkMutation`).  
**Goal**: Make mode/UID/GID meaningful and make read-only reversible for existing descriptors.

- [ ] Implement `Identity` and `Access` checks for owner/group/other using primary GID and root bypass, with directory execute traversal, read listing, and write+execute creation/removal/rename. Use configured effective UID/GID; defaults are root/root. Keep path resolution, `.` and `..` behavior within the existing inode tree.
- [ ] Apply mode `&^ 0022` at file/dir creation, owner UID/GID from machine config, and check existing target/open flags. Update `Getdirinode`, `Mkdir`, `OpenFile`, `Stat`, `Remove`, `Rename`, `ReadDir` and provider callers to the target signatures.
- [ ] Centralize read-only check for data and metadata mutation: `Openat` when creating or truncating, `Write`/`Pwrite`, `Truncate`, `Fallocate`, `Mkdirat`, `Renameat`, `Unlinkat`, and chmod/chown. Return `EROFS` without mutation even for a descriptor opened before the mode changed. `SetReadOnly(false)` re-enables normal operations on that descriptor.
- [ ] Preserve proper error priority for invalid descriptor/flags/range before mutation policy where documented; preserve `EACCES`, `EPERM`, `EROFS`, `ENOENT` and `ENOSPC` rather than collapsing errors to `EINVAL` in `Renameat` or other adapters.
- [ ] Test root and non-root traversal, mode changes, parent access, write via open descriptor across both read-only transitions, stop/crash/restart persistence, snapshot restore into a read-only target and disabled-feature abort behavior.
- [ ] Add `// TODO(not planned): ...` at existing limitations for supplemental groups/ACLs/sticky bits, symlinks and non-`AT_FDCWD` directory descriptors. Do not return fake success for an unsupported operation.

#### Verification

- [ ] Run focused provider and model tests with linkname flags, then `./.ci/gofmt check`, `make build`, `make test`; inspect report totals/failures and fix all failures.

---

### Task 6: Add Linux statfs and fstatfs routes and family adapters

**Files**: `internal/contracts/fileops.go`, `internal/routegen/routes.json`, generated kernel route/dispatch files, `internal/fs/ops.go`, `internal/compat/go125/profile/*`, `internal/compat/go126/profile/*`, generated family syscall adapters and family hook sources as needed.  
**References**: target §Public API and control wiring (`StatFS`, `FstatFS`), §Route and adapter table, §Data flows 2 and failure table; `docs/design.md` §Translation and composition and §Dispatcher.  
**Goal**: Make both path and fd filesystem statistics observable through selected Linux Go syscall surfaces.

- [ ] Add `StatFS(CallContext,string)(FSStorageStats,error)` and `FstatFS(CallContext,int)(FSStorageStats,error)` to `FileOps`, update fakes, and implement provider methods. Resolve path or fd before returning the one mounted volume's current stats; reject bad paths/descriptors with the existing errno contract.
- [ ] Append typed `StatFS` and `FstatFS` routes, regenerate kernel code and inspect route ID stability. Discover exact selected-family `syscall`, `os`, `internal/syscall/unix`, and `golang.org/x/sys/unix` symbols/signatures for Go 1.25 and 1.26 on Linux AMD64 and ARM64; update owned hook declarations and profile inventories accordingly.
- [ ] Have generated adapters fill architecture-correct Linux `Statfs_t` fields from the block-based stats. Use the selected family's source definitions, not host Darwin layouts; write output only on success. Do not route unknown/raw syscall entry points to the host.
- [ ] Update profile capability entries and `internal/fs/capabilities.go`; add route-generation and translated integration tests for total/free/available blocks, existing/missing paths, fd variant, read-only policy visibility and cross-family adapter compilation.

#### Verification

- [ ] Regenerate and run both documented `gosim-gen -check` baselines from `AGENTS.md`; inspect generated diffs and profile revisions.
- [ ] Run focused routegen/contracts/kernel/provider tests, then `./.ci/gofmt check`, `make build`, `make test`; inspect report totals/failures and fix all failures.

---

### Task 7: Add chmod and chown syscall routes and adapters

**Files**: `internal/contracts/fileops.go`, `internal/routegen/routes.json`, generated kernel route/dispatch files, `internal/fs/ops.go`, `internal/compat/go125/profile/*`, `internal/compat/go126/profile/*`, generated family syscall adapters and family hook sources as needed.  
**References**: target §Public API and control wiring (`Fchmod`, `Chmodat`, `Fchown`, `Chownat`), §Route and adapter table (`AT_FDCWD`, flags and `-1`), §Data flows 5, failure table; target §Storage schema and invariants (`metadataOp`, DAC).  
**Goal**: Expose already-implemented metadata operations to ordinary Go code through typed routes.

- [ ] Add exact `FileOps` signatures from target: `Fchmod`, `Chmodat`, `Fchown`, `Chownat`; update fakes and provider methods. Accept only `AT_FDCWD` and flags 0/`AT_SYMLINK_NOFOLLOW`; reject other options explicitly. Preserve UID/GID `-1` semantics and correct `EPERM`/`EROFS` errors.
- [ ] Append four typed routes without changing earlier IDs; regenerate kernel dispatch and route payload validation. Update family-owned hook declarations, generated adapters and profile records for Go 1.25/1.26 on Linux AMD64/ARM64. Confirm that `os.Chmod`, `os.Chown`, `syscall.Fchmod` and relevant `x/sys/unix` calls reach these routes.
- [ ] Update capability inventories and test fakes. Add translated tests for mode and UID/GID in `os.FileInfo.Sys()` and `syscall.Stat_t`, owner/root checks, `-1` chown fields, read-only toggling, crash/restore and both family adapter builds.

#### Verification

- [ ] Regenerate and run both documented `gosim-gen -check` baselines from `AGENTS.md`; inspect generated diffs and profile revisions.
- [ ] Run focused routegen/contracts/kernel/provider tests, then `./.ci/gofmt check`, `make build`, `make test`; inspect report totals/failures and fix all failures.

---

### Task 8: Delay covered filesystem completions without retaining borrowed memory

**Files**: `internal/contracts/fileops.go`, `internal/kernel/dispatcher.go`, relevant kernel descriptor/dispatch helper files, `internal/fs/lifecycle.go`, `internal/fs/ops.go`, dispatcher/provider tests.  
**References**: target §Architectural view (epoch event), §Public API and control wiring (`StorageDelayNS`, `scheduleFSCompletion`), §Data flows 7 and 8, §Method decision logic (`scheduleFSCompletion`), target failure table; `docs/design.md` §Dispatcher, descriptors and memory.  
**Goal**: Apply configured fixed latency without yielding during provider calls or retaining borrowed views after synchronous execution.

- [ ] Add `StorageDelayNS(CallContext) int64` to the FS service and return the machine's configured delay. Define a route classifier for FS descriptor `Read`/`Write`, `Pread`/`Pwrite`, `Fallocate`, `Truncate`, `Sync`, and mutating path/metadata operations. Delay `Openat` only when it creates or truncates. Do not delay socket/diagnostic descriptor I/O, control routes, `stat`/`statfs` or read-only open.
- [ ] Execute the provider call once under the existing supervisor protection, copy the scalar result, clear borrowed arguments before scheduling, and return `pending`. Schedule completion at checked `NowNanos()+LatencyNS` using the injected epoch event port. Do not defer the provider call itself, copy a `ByteView` into an event callback or draw randomness.
- [ ] Track timer ID by invocation ID, retire/cancel it on completion, `CancelEpoch` and dispatcher `Drain`; verify no duplicate completion, stale event wake or leaked lease. A sync fault/crash request aborts the delayed path and follows existing crash cleanup.
- [ ] Test simulated time advancement, zero delay, latency on success and `ENOSPC`/`EROFS`, exact-once side effects, open/close/restart cancellation, race between event and cancellation, read buffer cleanup, and unaffected network/diagnostic operations. Preserve replay trace determinism.

#### Verification

- [ ] Run focused dispatcher/kernel/provider tests with linkname flags, then `./.ci/gofmt check`, `make build`, `make test`; inspect report totals/failures and fix all failures.

---

### Task 9: Complete integration tests, capability records and architecture docs

**Files**: `internal/tests/fs`, `internal/tests/integration`, `internal/fs/capabilities.go`, both family profile capability records, `README.md`, `docs/design.md`, `examples/filesystem/README.md` where appropriate.  
**References**: target §Validation and documentation, §Data flows 1–8, failure table, §NOT CONSIDERED & TODO; brainstorm log §Repository observations and §User choices.  
**Goal**: Demonstrate the new behavior through public and Go syscall entry points, and make documentation match the implementation.

- [ ] Add or consolidate positive simulation tests for two machine capacities, stats before/after sparse and allocated writes, atomic full-volume failure, free space after truncate/unlink, read-only transition through an existing fd, restart/restore policy and metadata behavior, root/non-root access, and delayed I/O versus immediate metadata queries.
- [ ] Test ordinary `os` and selected syscall adapters on both supported families/architectures where the existing ordinary test harness covers them. Do not mark compile-only or skipped cases as passed. Verify disabled FS still aborts on attempted use.
- [ ] Update capability records for modeled operations and retain precise limitation descriptions for missing symlinks/links/special files, directory-relative descriptors, unsupported permission extras and no resize. Add `// TODO(not planned): ...` comments at relevant rejection branches; do not introduce logging-only or successful fallback stubs.
- [ ] Update README and `docs/design.md` for capacity units/default, `fs.Stats`, read-only toggle, identity/mode rules, latency timing, recovery over-capacity behavior and preserved `/app` cwd. Correct the former statement that chmod and permission handling are absent.
- [ ] Review generated/profile files and docs for accidental claims that ordinary `make test` certifies excluded tuples. Keep certification optional and manual.

#### Verification

- [ ] Run focused translated tests through `./.gosim/gosimtool` for `rt,fs` and relevant `rt,fs,net` cases, then `./.ci/gofmt check`, `make build`, `make test`; read `test-reports/unit/latest/report.md`, timeline and failing logs, and fix all failures.

---

### Task 10: Architectural delta verification

**Files**: Entire changed tree; this plan's **NOT IMPLEMENTED GAPS** and **NOT PLANNED CHANGES** sections.  
**References**: **all** target architecture sections, especially §Architectural view, §Storage schema and invariants, §Public API and control wiring, §Model/provider methods, §Route and adapter table, §Data flows, failure table, §Validation and documentation and §Method decision logic.  
**Goal**: Prove every designed component exists, behaves as specified and is reachable from real entry points.

- [ ] Verify in-memory schema and quota: metadata fields, COW chunk accounting, linked/open inode charging, default/zero capacity, sparse write/reservation rules, atomic `ENOSPC`, over-capacity recovery, and no DB/cache schema delta all match target §Storage schema and invariants. Check code, not just tests.
- [ ] Verify model durability: `allocOp`, `writeOp`, `resizeOp`, `reserveOp`, `metadataOp`, pending dependencies, flush/crash clone/release, snapshot restore and stat metadata all match target §Model/provider methods and §Data flows 3–5 and 8.
- [ ] Verify public API/contracts/control state: exact `Config`, `StorageStats`, `FSConfig`, `FSStorageStats`, `SetReadOnly`, `Stats`, native `ErrNotSim` behavior, kernel state guards, provider ownership and retained policy match target §Public API and control wiring and §Data flows 1, 2 and 6.
- [ ] Verify syscall interfaces and generated routes: every target `FileOps` signature, append-only route, payload validation, Go 1.25/1.26 AMD64/ARM64 adapter, profile capability declaration, Linux stat/statfs layout and error mapping match target §Route and adapter table and failure table.
- [ ] Verify latency and lifecycle: covered route classification, descriptor ownership, borrowed-view clearing, epoch timer cancellation, exact-once completion, startup non-yielding, restart and trace/replay constraints match target §Architectural view, §Data flows 7–8 and pseudocode.
- [ ] Verify tests/docs/scope: all target §Validation and documentation cases are covered; README/design/capabilities reflect actual limits; every target §NOT CONSIDERED & TODO item has an explicit rejection, default behavior or `// TODO(not planned): ...` comment where applicable. Record all differences in **NOT IMPLEMENTED GAPS** and fix them. Mark an unfixable item `[_]` only if a later user decision explicitly defers it.
- [ ] **Last wiring check:** trace every method and component from public control or translated Go syscall entry point through generated route, kernel service, session/instance and model, including constructor/composition bindings and both selected Go families. No component counts as implemented merely because it exists or is tested in isolation. Record and fix every wiring gap in **NOT IMPLEMENTED GAPS**.

#### Verification

- [ ] Run `./.ci/gofmt check`, `make build`, both exact `gosim-gen -check` baselines when generated sources changed, and `make test`; inspect report totals/failures and fix all failures. Do not run race/large suites without an explicit later instruction.

## Validation Commands

Run these only at the tasks that need them; the final ordinary gate is `make test`.

```sh
./.ci/gofmt check
make build
make test
cat test-reports/unit/latest/report.md

# Focused native checks, with required linkname policy:
go test -ldflags=-checklinkname=0 -tags=linkname ./internal/contracts ./internal/kernel ./internal/fs ./internal/fs/model

# Generated adapter/profile checks after route or family changes:
GOTOOLCHAIN=go1.25.14 go run ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=. -check
GOTOOLCHAIN=go1.26.9 go run ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=. -check

# Focused simulation checks use the launcher and selected worker:
./.gosim/gosimtool test -features rt,fs -seeds 1-20 -run TestName ./internal/tests/fs
```

Use `make test` reports rather than treating terminal silence, skips or compile-only outcomes as success. The two exact Go versions above are generation baselines; ordinary callers may use any supported stable Go 1.25/1.26 patch. There is no certification step in this plan.

## NOT CONSIDERED & TODO

- [postponed] Filesystem resize. There is no resize API or successful stub. Configuration rejects changes after the first start; a comment documents that capacity is chosen before first start.
- [potential-improvement] Supplemental groups, ACLs, sticky/setgid/setuid rules and configurable umask. Use primary GID and fixed `0022`, with precise limitations and `// TODO(not planned): ...` where permissions reject unsupported behavior.
- [too-complex] Symlinks, hard links, device nodes, mount points, procfs and directory-relative descriptors beyond `AT_FDCWD`. Keep existing explicit errors and annotate relevant boundaries; never silently use host I/O.
- [potential-improvement] Variable/per-operation latency distributions. Use one fixed nonnegative delay and a comment at policy parsing/classification.
- [potential-improvement] Directory metadata quota. Count allocated file-data blocks only; document this in `fs.Stats` and architecture docs.

## NOT IMPLEMENTED GAPS

_Populate during Task 10; resolve every gap or mark `[_]` only after explicit user deferral._

## NOT PLANNED CHANGES

_Record implementation deviations here with reason, affected files, and target-document update. None at planning time._
