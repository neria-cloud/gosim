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
