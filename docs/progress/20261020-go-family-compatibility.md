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


### Task 3 completed: contracts, provenance and generation

Added family-owned contracts for both source architectures: each contains the selected hooks/shims and native coroutine/race declarations, 20 mirrored syscall/timer layouts, and 72 syscall constants. Source inspection uses the selected Go executable's file selection and go/types layout/constant evaluation. Host and translated Linux contexts are explicit; race source selection does not compile or run an instrumented executable. Loader and checker share the owned translated-source tags. Profile descriptors own their nested metadata and validate hook/contract consistency.

Ordinary CLI and candidate resolution now perform/cache contract checks and pass a single compiler fingerprint inventory into identity creation. Successful records bind actual compiler/stdlib content, contract and checker revisions, platform/race/source tags, and ABI module content. Cache reuse checks complete records; corrupt records miss, failures are not cached, publication is atomic, and per-key OS locks wait cancellably and serialize concurrent checks. ABI module digests read contents even if sizes/mtimes were preserved.

Origin schema 2 separates family and generation-baseline compiler from caller eligibility. Generation records use schema 2, family and generated_with. The metadata catalog and family registry drive generation/revisions/bounds; inactive or retired family imports remain rejected. The generator no longer imports gosimtool/host ABI merely to find go.mod. Explicit -refresh-origin verifies the existing contracts before recording the current selected source files; it does not modify expectations. Initial Go 1.25.5 refresh exposed a moved baseline file (internal/syscall/unix/fchmodat_linux.go), so refresh now inventories selected current files instead of requiring baseline path survival. Baseline reproduction still verifies its exact provenance. Both baseline route records and family source revisions were regenerated.

Verified on native Darwin ARM64: full spec unit package on Go 1.26.9; focused contract/cache/source-context/identity checks; normal command resolution on Go 1.26.9 and Go 1.25.5; current-family provenance refresh on Go 1.25.5; scoped baseline generation and nonmutating check behavior on Go 1.26.9. Metadata generation read both Linux source architectures but executed only native generator processes. No certification, foreign executables, race-instrumented executables, or large suites ran. All final focused checks passed. Existing generation/translation/script test sweeps still need the Task 6 migration to current selected compilers before final ordinary verification; automatic worker startup and full workspace propagation remain Tasks 4–5.

Implementation details extending the draft: Hook.Context explicitly distinguishes host/host-race declarations; compatibility cache inputs also include checker revision and owned source tags; RefreshOrigin takes the resolved compiler plus reviewed manifests so it can validate and locate moved source declarations. These preserve the target's compatibility/provenance separation.


### Next stage inspection

While tracing worker preparation, found that prepareSelftest built the native race oracle even for ordinary identities. Added an explicit Identity.Race guard; the existing race harness already skips those oracles for ordinary builds. This closes an indirect race-build path before final ordinary verification. This small fix will accompany the worker/selftest refactor. No race oracle has been built during this goal.

### Task 4 completed: automatic worker bootstrap

Moved the command implementation and unit tests to internal/gosimcli. cmd/gosim is now a family-independent launcher; cmd/gosim-worker is the direct, nonrecursive worker. Launcher help/version needs no Go executable or module setup. Normal Go discovery chooses the caller's compiler; the effective module/workspace graph selects Gosim source, including local replacements. Missing Gosim is an actionable dependency error, with no install-time-source fallback. Selected worker dependencies are hashed by actual source content, independently of unrelated caller source edits.

Implemented workers/v1 content-keyed cache entries, binary/build-info validation, bounded protocol handshake, cancellable OS locks, concurrent recheck, private staging, input mutation checks and atomic publication. Workers validate private invocation files against their own compiler, binary and current source inputs. Stdin/stdout/stderr and exit codes are forwarded; cancellation signals process groups, escalates after a bounded grace period and reaps the process. Repeated fingerprints across the launcher/worker trust boundary are deliberate revalidation; the worker reuses its validated inventory for contract checks and identity creation. Cold publication also rechecks distribution content after compilation.

Focused local checks passed on default Go 1.25.5: internal/workerboot, internal/buildspec, internal/buildcfg and internal/gosimcli. Tests cover concurrent cache publication/reuse, corrupt binaries/manifests, fresh-inode repair, invocation-file cleanup, module replacements/workspace selection, preserved-mtime dependency edits, unrelated application edit reuse, absent Gosim, canceled lock waits/process groups, malformed/mismatched/oversized handshakes and stream/exit forwarding. Go's trimpath builds omit ldflags metadata; validation accepts that documented omission while binding the controlled flags in the cache key and exact binary bytes in the manifest. A real oversized-output test exposed bytes.Buffer.ReadFrom bypassing the embedded buffer's Write limit; switched to a named buffer and reran the targeted handshake tests successfully. The first workspace fixture needed an explicit versioned replacement for its unpublished synthetic dependency; corrected the fixture and reran the affected packages successfully.

Native smoke checks: default Go 1.25.5 launcher resolved identity successfully; that same launcher selected/built Go 1.26.9 through normal GOTOOLCHAIN configuration. Initial Go 1.26.9 startup was 14.748 seconds; cached startup 1.184 seconds, with identical identities. Logs and JSON are under .gosim/go-family-work (worker-focused.log, worker-protocol.log, bootstrap126-*.json/log and bootstrap-timings.json). The first worker-protocol.log records the detected oversized-output failure; the successful targeted rerun was printed in the terminal. Dependency inspection confirms the launcher imports no executable family adapters, host ABI, runtime engine, translator or gosimcli. No certification, race binaries, cross-platform jobs or large suites ran.

Next: carry the resolved workspace graph into application identities/loading/output modules and metatests (Task 5), then migrate legacy test assumptions (Task 6). Existing selftest/script entry handling and internal build context propagation will be completed with that integration before final suites. Final whole-library verification is still pending.

### Task 5 — workspace graph portion

Shared neutral module selection/flattening now lives in buildspec so bootstrap, contract checking and translation use the same Go-resolved graph without a spec/buildcfg import cycle. Application identities bind selected module versions/replacements and actual contents of all local/workspace main modules. BuildConfig carries the resolved caller context and cancellation context; loaders use caller cwd/workspace for application sources and GOWORK=off for private composition/output modules. Private module generation preserves selected requirements/replacements and rejects output graph upgrades, replacement changes, missing modules or extra modules before compilation. Commands also work at a workspace root without a nearest go.mod. Artifact lock waits and native compilation are cancellable; interrupted builders do not publish and clean their staging.

Contract inspection now uses the same flattened graph. Checker/profile revisions include neutral buildspec source as well as the adapter/checker source, so shared resolution changes invalidate compatibility-cache results. Regenerated family revisions.

Validation: focused workspace module construction/graph matching, contract-module preservation, translation configuration, artifact cancellation and existing artifact checks passed on native Go 1.25.5. A real application in a temporary multi-module workspace (Gosim plus a local dependency) passed TestWorkspace in a translated executable, invoked from the workspace root without go.mod. This took 11.518 seconds including cold worker startup and translation/build; output is .gosim/go-family-work/workspace-smoke.log and the fixture location is workspace-smoke-path.txt. Focused package logs are workspace-focused.log and workspace-contracts.log. An initial generator implementation promoted every transitive module to a direct graph root, which expanded Go's pruned graph and requested unrelated sums; fixed generation to retain the union of original roots plus required Gosim composition roots, and verified exact output selection. The temporary workspace's initial go directive also needed 1.25.0 to satisfy Gosim's existing language minimum.

The first Task 5 checkbox is complete. Automatic native metatest workers, selftest/script entry migration and final integration checks remain before Task 5 is complete. No certification, race binaries, foreign-platform jobs or large suites ran.

### Task 5 completed: native metatest integration

Native metatests now freeze the running Go test process's compiler and use workerboot source selection/cache/protocol execution. Removed the go-run/manual CLI fallback. Missing optional gosim.json works normally. GOSIMTOOL overrides must have the matching generated manifest alongside the worker binary: protocol/compiler alone cannot prove source provenance, so an unmanaged/stale override gets an actionable diagnostic to unset the override for automatic startup. Tests check both matching overrides and source-mismatched rejection.

Selftest preparation builds the independent launcher for script callers instead of copying its direct worker. Ordinary native verification and script environments no longer inject candidate-wrapper overrides; each caller selects its own source and compiler. The large script fixtures' remaining schema/pinning assumptions are Task 6. Suite command/converter execution now forwards cancellation through process groups; native metatest invocation handles interruption while its child group is active so it can stop that worker as well.

Focused worker-cache/override, artifact and CLI tests passed on Darwin ARM64/Go 1.25.5. An actual native TestNativeWorker in the external multi-module workspace, with no GOSIMTOOL or compiler in config, prepared its worker/artifact and successfully ran TestWorkspace through metatesting (10.579 seconds, including startup/build). Output: .gosim/go-family-work/metatest-workspace-smoke.log; focused package log: metatest-focused.log. Report-converter/concurrency tests and the added suite cancellation check passed. Profile revision check and diff checks passed. No certification, race, foreign-platform or large-suite execution occurred. Final whole-library verification remains pending until Task 6 migration is complete.

### Plain-module follow-up

The corresponding external application test with GOWORK=off found one additional graph-pruning case: rooting all Gosim-declared requirements expanded a caller's tidied graph with unrelated modules. Removed that expansion; private modules preserve only the caller's original main-module roots. Added a focused three-level pruned-graph regression. Root gosim and native gosimtool tool-import declarations now retain cmd/gosim-worker dependencies during normal go mod tidy, including callers that import only metatesting. These declarations use the repository's existing never-tag convention and do not introduce runtime imports into the launcher.

After ordinary Go module setup, the native external TestNativeWorker passed with GOWORK=off, automatically preparing and running its matching simulation worker. The caller's go.mod was byte-identical before/after execution. This took 10.543 seconds; evidence is .gosim/go-family-work/metatest-module-smoke.log and module-smoke-path.txt. Both focused private-module tests passed. This supersedes the earlier note about adding Gosim composition roots; only caller-selected roots are materialized now. No certification, race, cross-platform or large suite ran.
