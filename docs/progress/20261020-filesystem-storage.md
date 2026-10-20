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
