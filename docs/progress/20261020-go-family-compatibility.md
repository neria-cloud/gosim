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
