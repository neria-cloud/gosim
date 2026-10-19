# Implement Go family compatibility

Target: [architecture](../brainstorms/20261019-go-family-compatibility-target.md).
Progress: [execution record](../progress/20261020-go-family-compatibility.md).

User constraints: implement the full target; commit logical major changes; no certification execution; tests only on this native Darwin ARM64 host; long suites only at final verification. Preserve unrelated staged README changes. Exact versions in historical evidence do not become new support gates.

## Stages

### Task 1: Go family policy and automatic toolchain resolution

References: target §§1–4.1, 5.1, 8, 12.

- [x] Implement stable family registry, default/legacy configuration handling, ordinary Go discovery and frozen application/output command contexts.
- [x] Remove distribution allowlist/download admission; fingerprint installed toolchain content.
- [x] Focused native tests for discovery, version parsing, config and commands pass; commit.

### Task 2: Evidence-independent identities and support selection

References: target §§4.2, 7–9, 12.

- [x] Implement schema-2 identities/artifacts and remove certification/pending tuple admission.
- [x] Separate manual exact-build evidence from ordinary execution; retain disabled AMD64/race certification.
- [x] Update composition/metatest protocol/runtime info and focused checks; commit.

### Task 3: Family contracts, provenance and generation

References: target §§4.3, 6, 8, 12.

- [ ] Separate baseline origin from caller compatibility; implement actual hook/layout/constant contracts and successful-check cache.
- [ ] Add metadata catalog; replace scattered two-family and patch lists; update generation records/refresh behavior and revision metadata.
- [ ] Verify focused generation/contract tests on native host; commit.

### Task 4: Automatic matching worker and independent launcher

References: target §§3, 4.4, 5.1–5.3, 8, 12.

- [ ] Move command implementation into gosimcli; add direct worker entry and family-independent launcher.
- [ ] Implement application-selected Gosim source resolution, content-keyed cache, cancellable locking/publication, protocol verification and process forwarding.
- [ ] Test cache hit/miss, isolation, concurrency, interruption, replacements and error paths; commit.

### Task 5: Workspace graph and metatest integration

References: target §§5.4–5.5, 8–10.

- [ ] Carry module/workspace resolution through translation and generated output graph; preserve exact dependency selection.
- [ ] Integrate automatic workers into native metatesting with running-process compiler identity.
- [ ] Verify focused multi-module workspace, local replacements and artifact/handshake checks; commit.

### Task 6: Ordinary reporting and CI without certification

References: target §§7–8, 12.

- [x] Separate verify from certify; remove certification/reset metadata from ordinary test paths and CI.
- [x] Remove local exact-patch defaults/allowlists; retain report layout, parallel budgets and explicit long-suite gating.
- [ ] Adapt boundary/translation/script test assumptions without reintroducing duplicated sweeps; focused orchestration tests; commit.

### Task 7: Local verification, documentation and architecture audit

References: target §§1–12, especially validation §10.

- [ ] Reconcile AGENTS/README/current design with actual implementation while preserving unrelated user changes.
- [ ] Run and check ordinary local tests, early/later supported-patch coverage and final local large suites; no certification or cross-platform jobs. Race suite requires an explicit instruction naming it under AGENTS.
- [ ] Fix every observed failure; record reports and actual skipped coverage. Measure startup/cache timings.
- [ ] Audit every target component, entry-point wiring, schema, error case and test requirement. Resolve gaps before marking complete; commit final changes.

## Validation policy

Use focused native package tests with `-ldflags=-checklinkname=0 -tags=linkname` during implementation. Preserve compatible exact installed compiler selection where intermediate code still requires it. Full `make test` and expensive suites are final-stage validation; this supersedes the implementation skill's repeated whole-suite suggestion. No test command may invoke certification indirectly. Check running test jobs no more often than every five minutes unless consuming already-completed short-command output.

## Deviations and unresolved gaps

None recorded at initialization. Implementation findings and justified signature adjustments are recorded in the progress log; scope reductions require explicit user direction.
