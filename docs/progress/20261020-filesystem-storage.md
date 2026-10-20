# Filesystem storage implementation progress

Plan: `docs/plans/20261020-filesystem-storage.md`  
Target: `docs/brainstorms/20261020-filesystem-storage-target.md`  
Branch: `feat/filesystem-issues-ext`  
Started: 2026-10-20

## Task 1 — Expose configuration, read-only control and storage stats

Implemented
- Added public configuration, read-only control and stats, typed contracts/routes, kernel forwarding and provider session policy.
- Added live allocated-block accounting and focused model/session tests for prestart stats, lifecycle, config validation and unlinked open files.
- Regenerated both family route inventories and refreshed profile adapter fingerprints after the new routes changed the transport digest.

Verified
- Focused contracts/kernel/fs/model tests passed.
- Both exact `gosim-gen -check` baselines, `./.ci/gofmt check`, and `make build` passed.
- `make test` passed: 1,801 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T133054.592468Z-0vhlkghd/report.md`).

Notes
- The first ordinary run found stale adapter fingerprints in `internal/compat/spec`; `go run ./internal/compat/genprofile` refreshed them, and the rerun passed.
- Read-only enforcement and capacity admission are covered by later plan tasks; this task establishes policy/stats/control wiring.

## Task 2 — Enforce block capacity on writes and truncation

Implemented
- Added per-file allocated/new-block counting and atomic model write admission against machine capacity.
- Preserved sparse truncation, converted model write/truncate calls to explicit error returns, and passed capacity through provider I/O.
- Added tests for full-volume failure without changed offset/size/pending ops, sparse writes, block reuse, COW snapshots and read-only transitions through an open descriptor.

Verified
- Focused model/provider tests, `./.ci/gofmt check`, and `make build` passed.
- `make test` passed: 1,803 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T133908.692114Z-6xbyxeef/report.md`).

Notes
- Read-only admission for `Openat` and `Fallocate` was introduced early because these paths already mutate the model; Task 5 will refine path-aware error priority.

## Task 3 — Make fallocate reserve capacity and survive recovery

Implemented
- `Fallocate` now reserves zero-filled 1 KiB blocks over a nonnegative range after atomic capacity admission.
- Added `reserveOp` to pending/crash replay, including clone retention and dependency ordering after file resize.
- Added model/provider tests for overlapping and nonzero-offset reservations, `ENOSPC` atomicity, sparse reads, crash-state enumeration, sync durability and truncate freeing reserved blocks.

Verified
- Focused model/provider tests, `./.ci/gofmt check`, and `make build` passed.
- `make test` passed: 1,805 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T134338.094724Z-a32mh0gk/report.md`).

Notes
- Existing lifecycle rollback tests stayed green; reservation itself consumes no random draw or host I/O.

## Task 4 — Persist mode, UID and GID in inode and crash state

Implemented
- Stored mode and ownership on file and directory inodes, including the initial Linux tree and newly allocated inodes.
- Added metadata operations to pending dependency, flush, crash selection and restore paths.
- Reported mode, UID, GID and link count through model and provider stat; added chmod/chown authorization and no-op handling.
- Updated existing stat assertions for populated metadata and added model/provider durability tests.

Verified
- Focused filesystem, kernel, network and Go 1.25 hook tests passed; `./.ci/gofmt check` and `make build` passed.
- `make test` passed: 1,807 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T135214.086819Z-0n2qpo5l/report.md`).

## Task 5 — Enforce identity and read-only mode on mutations

Implemented
- Added owner/group/other access checks, directory execute traversal, and write access on directory mutations using per-machine UID/GID.
- Applied a fixed 0022 umask and ownership on file and directory creation; propagated identity through path and directory operations.
- Centralized read-only checks for mutation paths and preserved model error values in rename and directory listing.
- Added model/provider tests for access denial, creation metadata and read-only mutation transitions.

Verified
- Focused filesystem, kernel, network and Go 1.25 hook tests, `./.ci/gofmt check`, and `make build` passed.
- `make test` passed: 1,809 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T135743.593082Z-bsl3j_we/report.md`).
- After adding the restore-policy assertion, `make test` passed again: 1,809 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T140039.431773Z-4xwf8kc8/report.md`).

## Task 6 — Add Linux statfs and fstatfs routes

Implemented
- Added typed path and descriptor storage-stat routes, provider resolution and contract fakes.
- Added reviewed Linux `Statfs_t`/`Fsid` layouts and syscall/x/sys adapters for both Go families and both target architectures.
- Added provider and translated tests for capacity, allocated/free blocks, missing paths, descriptors and read-only reporting.
- Updated capability records and generated route/profile revisions.

Verified
- Both exact `gosim-gen -check` baselines, focused native/translated tests, `./.ci/gofmt check`, and `make build` passed.
- `make test` passed: 1,818 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T141002.245085Z-hkgd383b/report.md`).

## Task 7 — Add chmod and chown syscall routes

Implemented
- Added typed descriptor/path chmod and chown contracts, routes and provider methods with owner/root and read-only checks.
- Wired `syscall`, `x/sys/unix` and `internal/syscall/unix` entry points through reviewed Go 1.25/1.26 adapters.
- Added provider and translated syscall tests for mode, ownership, `-1` preservation, access denial and unsupported flags.
- Updated filesystem and profile capability records.

Verified
- Both exact `gosim-gen -check` baselines, focused native/translated tests, `./.ci/gofmt check`, and `make build` passed.
- `make test` passed: 1,823 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T141656.067148Z-riy4xdd4/report.md`).

## Task 8 — Delay filesystem completions safely

Implemented
- Added per-machine storage delay to the FS service and classified covered data I/O and mutation routes in the dispatcher.
- Executed provider effects once, cleared borrowed invocation arguments, then scheduled scalar completion on an epoch event.
- Tracked and canceled pending timers on completion and epoch cancellation; distinguished read-only and mutating opens.
- Added tests for deterministic time, immediate metadata/non-FS I/O, delayed `ENOSPC`/`EROFS`, cleared borrowed views and stop cancellation.

Verified
- Focused dispatcher/kernel tests, `./.ci/gofmt check`, and `make build` passed.
- `make test` passed: 1,825 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T142420.758365Z-2obk3e90/report.md`).

## Task 9 — Complete integration tests and architecture docs

Implemented
- Added public simulation tests for capacity and sparse allocation at two sizes, atomic `ENOSPC`, live read-only transitions, UID/GID access, fixed latency and immediate metadata, and an over-capacity restored image.
- Updated README, design, and filesystem example documentation to match the new storage API, crash behavior, and supported limits.
- Reviewed capability records and generated/profile declarations from the route tasks; retained explicit unsupported-operation descriptions.

Verified
- Focused translated tests passed with `rt,fs` and `rt,fs,net` for seeds 1–2; `./.ci/gofmt check` and `make build` passed.
- `make test` passed: 1,841 passed, 16 skipped, 0 failures/errors (`test-reports/unit/20261020T143134.733406Z-142kf_bw/report.md`).
