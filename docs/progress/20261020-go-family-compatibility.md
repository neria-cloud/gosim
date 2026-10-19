# Go family compatibility execution

Target: `docs/brainstorms/20261019-go-family-compatibility-target.md`.
Plan: `docs/plans/20261020-go-family-compatibility.md`.
Branch: `feat/refactorin-modularization`.

## Initialization — 2026-10-20

- User authorized full implementation and logical commits, local-platform tests only, no certification, long tests only at final verification.
- Read live worktree, target, repository instructions and implementation skills. Applying go-implement with focused incremental validation under the user's long-test constraint.
- Existing AMD64/race disablement is retained as a separate logical commit; its focused tests and matrix dry-run already passed in the preceding task. No certification was executed.
- Original separately staged README patch and unstaged disablement README patch saved in `.gosim/go-family-work/` to preserve user edits during later selective commits.
- Default local Go is go1.25.5 on darwin/arm64. This is an important real acceptance target: normal Go resolution must work without switching the application to a Gosim-maintained baseline.

## Task 1 — Go families and automatic compiler discovery — 2026-10-20

Implemented stable version/family policy (including unregistered future patches), schema-2 versionless configuration with explicit legacy migration, discovery through the caller's normal Go tooling, exact frozen application/output command contexts, and installed-toolchain content fingerprints. Removed the embedded distribution archive allowlist and network archive comparison. Adapted configuration consumers to keep native builds compiling during the staged migration.

Validation:
- Go 1.26.9 focused `internal/buildcfg` and `internal/buildspec` tests passed, including fake-tool discovery, context/override/cancellation and fingerprint mutation/location checks.
- The same focused packages passed using the machine's normal Go 1.25.5 (`GOTOOLCHAIN=local`).
- CLI, gosimtool and candidate packages compiled; focused CLI concurrency-budget check passed. Their expensive suites were not selected.
- gofmt/gofumpt for changed Go files and `git diff --check` passed.

Notes: ordinary command certification/tuple admission remains until Task 2; automatic worker bootstrap remains Task 4. `ConfigRequest.Legacy` is an additional process-local field allowing migration diagnostics at the CLI boundary without printing from library configuration code. No certification or cross-platform jobs ran.

## Task 2 — Evidence-independent identities and family admission — 2026-10-20

Implemented schema-2 executable/artifact identities, actual toolchain and contract digests, family support selection with no tuple/evidence lookup, and removal of embedded certification from generated runtime configuration. Kept the public Certified field deprecated/false. Exact manual reports now bind a framework digest and cannot substitute ordinary validation for large coverage. Removed active tuple validators and profile evidence embedding; historical files remain. Advanced metatest handshake to protocol 3 and replay revision to v3; old schemas and wrong-patch artifacts are rejected.

Verified focused native tests for canonical identities, report validation (in-memory only), family selection, adapter metadata, composition imports, artifact publication/corruption/locks, lookup isolation, translation-cache identities, and metatest protocol/reaping. Ordinary CLI resolution without current certification passed both Go 1.26.9 and default Go 1.25.5. Initial checks found stale fixture fields and the old negative schema value; fixed them and reran affected checks successfully. Regenerated family source revisions after metadata changes; formatting and diff checks passed.

Remaining planned work: full layout/constant contract inventories and compatibility-cache execution (Task 3); automatic worker/bootstrap and workspace integration (Tasks 4–5); ordinary test orchestration still calls the old entry until Task 6, so no full-suite command has been run. No certification or cross-platform execution occurred.


### Task 6 (orchestration portion brought forward)

The user's correction that certification is not a CI step was already recorded in the target, but the existing workflow still invoked the matrix and ordinary verification still called certify. Implemented this independent portion ahead of the remaining contract/bootstrap work: CI now runs ordinary `make test` plus reporting checks with family selectors and uploads test-reports. Local verification calls verify, defaults to one ordinary mode, and only selects race explicitly. No report certification or tuple resets occur. Shared snapshot/resource helpers are independent of matrix scheduling; snapshots preserve historical evidence bytes. Removed exact patch defaults from Makefile, test.sh and the reporter, and migrated the repository config to schema 2. Taskfile aliases now select ordinary/race deliberately. Updated AGENTS, command documentation and current design for the implemented policy.

Focused validation: all 18 Python orchestration checks and 15 reporting checks passed; focused native Go suite scheduling/concurrency checks passed on Darwin ARM64 with Go 1.26.9. Python checks use mocked matrix workers and do not execute certification or cross-platform processes. An initial mock captured a later-mutated environment dict; separated discovery and frozen child dictionaries and reran successfully. Full local/large verification remains deferred until the final stage. Script/boundary test migration and Task 3 contracts are still in progress, so Task 6 as a whole is not complete.
