# Implementation Plan: Compact Go version support

**Plan:** `docs/plans/20261021-compact-go-versions.md`

**Date:** 2026-10-21

**Project source:** `.` (relative to repository root; `../..` from this plan)

**Branch / starting revision:** `feat/compact-go-versions` / `550d5e3`

**Target Architecture:** [revision 3.1](../brainstorms/20261021-compact-go-versions-target.md)

**Brainstorm session:** [complete log](../brainstorms/20261021-compact-go-versions.md)

**Design review:** [thread 1](../review/feat_compact-go-versions.md)

**Plan review:** thread 2 closed after final re-review; all verified findings incorporated. Final small corrections were not sent for another peer round.

**Status:** implementation in progress; see task checkboxes and the execution log.

## Scope and execution rules

Implement the reviewed shared-package design, Go 1.25 baseline plus cumulative metadata/source changes, stable `1.X.*` admission, and comparison/report/draft tooling. Go 1.27 is a hypothetical fixture, not a newly supported release in this plan. Keep the four existing immutable feature sets and all runtime/determinism/native-boundary invariants.

The source of design decisions is target revision 3.1. Existing README, AGENTS.md and docs/design.md describe today's family-owned trees and will be updated after migration; their current build/test restrictions continue to apply throughout. There is no `project.txt`, `architecture/` or `docs/architecture/` in this checkout. `docs/brainstorms` is an ignored ordinary directory, not a symlink or nested repository. The plan is tracked in the existing branch; do not create another repository or force-add ignored brainstorm files.

- Execute tasks in order, batching two consecutive tasks per commit (user direction, 2026-10-21). Validate both tasks before committing; retain separate completion records. Each task ends with build and focused verification; fix any failures encountered, including unrelated failures in commands actually run, before reporting that gate green. Record unexpected changes and evidence in the sections below.
- Each task budgets **at most 550 lines of authored Go implementation and test edits**. Pure file renames, verified deletion of an identical second copy, generated JSON/Go and fixture relocation are separately reported mechanical changes, not hidden inside the authored-code estimate. Preserve moved bodies; do not rewrite large files. If the authored portion exceeds its bound, split it into consecutively numbered, buildable subtasks before execution and update references.
- Migration tasks may temporarily retain existing old metadata/admission until their consumers move. This is an ordered checkout transition, not a new compatibility feature. Add no forwarding packages, runtime family switches, manual release tags, legacy cache decoder or successful runtime stub. Final retirement removes every transition rule.
- Update both families' root `shims/guard_test.go` and `guards_test.go` imports in the same task as every moved dependency (starting with runtimebridge in Task 9); do not defer these importers until guard consolidation in Task 14.
- Task 1 may expose explicit command-not-yet-implemented errors while later tasks supply handlers. They must never claim success, affect ordinary Gosim commands or remain after Task 27. Deferred features use a concise `// TODO(not planned): ...` at the relevant maintenance boundary where useful; do not add warnings or runtime stubs for hypothetical features.
- Regenerate affected revision outputs at the end of every source-changing task. Through Task 17 use the existing genprofile command for active old descriptors; Task 18 switches to `gosim-gen profiles`. Both hashing paths must include shared inputs during the transition. Do not leave normal test fixtures reading stale revision constants.
- No new third-party dependency or database migration is needed. SQLite DDL, CacheEntryV2 and contract-cache schemas/retention remain exactly as target §5; fingerprint changes invalidate artifacts without new cache formats.
- Race, large, cross-architecture execution, Docker/Colima and certification are **not** part of this plan's validation. Do not enable their environment gates, call their fixtures directly, or substitute tags-only race compilation. Source selection/contract inspection for another architecture or race context is reported as inspection only. Lint remains scoped to cmd/examples.
- Execution authorized 2026-10-21: commit each pair of completed tasks, request Claude review every eight implementation commits, and pass ordinary tests before final architectural verification. Certification is authorized only after final review and only on native Darwin ARM64; race and large suites remain excluded.

## Architecture and task ordering

The high-level CLI/metadata contracts come first. Then prepare boundary/source-selection/generator code for safe moves; consolidate native host, hooks and shims while existing profiles continue serving callers; convert metadata and switch selection; remove transition code; finish comparison and end-to-end validation.

```mermaid
flowchart TD
  CLI[1 CLI dispatch] --> Schema[2–4 definitions and resolved views]
  Schema --> Prep[5–7 boundaries, source selection, rendering]
  Prep --> Move[8–14 shared host, hooks, shims]
  Move --> Metadata[15–19 profile data, maintenance and cutover]
  Metadata --> Boundary[20–21 retirement and compiled graph]
  Boundary --> Inspect[22–25 stable compiler inspection and observation]
  Inspect --> Compare[26–28 comparison, publication and onboarding checks]
  Compare --> Finish[29–31 integration, docs and ordinary validation]
  Finish --> Delta[32 architectural delta and actual wiring]
```

Intermediate generated/shared code must work under both currently supported families. A path move updates its imports, selected profile target strings, policy permissions, generation destinations and fixture expectations together. Do not move only one side of a type-universe boundary and leave an unbuildable task.

### Contract index reused by tasks

The following are the target structures/signatures, not alternate schemas. Tasks must implement exact JSON tags/validation in target §§3, 4, 6 and 7; do not duplicate expanded derived metadata in committed files.

```go
// target §3: all RecordDelta.Remove keys are tuples, even scalar keys.
type FamilyDefinition struct {
    Schema int `json:"schema"`
    Family string `json:"family"`
    Parent string `json:"parent,omitempty"`
    Supported bool `json:"supported"`
    Delta ProfileDelta `json:"delta"`
}
type RecordDelta[T any] struct {
    Add []T `json:"add,omitempty"`
    Replace []T `json:"replace,omitempty"`
    Remove [][]string `json:"remove,omitempty"`
}
type Declaration struct {
    Hook Hook `json:"hook"`
    Arches []string `json:"arches,omitempty"`
}
// ProfileDelta contains declarations, packages, bridges, native_imports,
// shared_globals, probes, modules, capabilities, runtime_packages and
// architecture-keyed layout/constant deltas, exactly as target §3.
// ResolvedFamily has Family, ProfileID, Supported, Chain, DefinitionDigest
// and materialized record slices/maps; no origin or delta operations.
// Profile gains Chain and DefinitionDigest and always holds BOTH
// architecture contract manifests; callable hooks select one architecture.
```

Origin remains schema 2 `{schema,family,baseline_toolchain,files:[{path,digest,hooks?}]}` in independently generated on-disk files. No runtime origin embed. AdapterRevision excludes origins; complete-source TranslatorDigest includes them. InspectorDigest excludes data/generated support artifacts and binds implementation independently of parent DefinitionDigest/OriginDigest/module inputs.

New CLI routes are `profiles`, `observe`, `compare` plus existing flag-only generation. Compare output uses target §6 ComparisonReport, DraftDelta, Finding, DeltaProposal, ContextResult and observation schemas exactly. `api_diff` is `inspected`, `skipped-no-parent` or `incomplete`. Exit 0 is completed comparison, never compatibility approval; exit 1 is incomplete/operational error; exit 2 is bad CLI input.

## Validation Commands

Commands below are execution gates, not commands run while writing this plan. Use the repository-selected compiler for ordinary focused tests; repeat family-sensitive checks under each recorded baseline where specified. Always retain the linkname flags.

### V-BUILD: intermediate native build

```sh
go build -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim ./cmd/gosim-worker ./cmd/gosim-gen
```

For a family-sensitive task run the same command separately with `GOTOOLCHAIN=go1.25.14` and `GOTOOLCHAIN=go1.26.9`. Do not build inactive version packages with a mismatched compiler. This builds host commands, not simulation validation. During Tasks 10–14 also build `./internal/compat/... ./internal/hostabi/...` with the same flags under each baseline: command dependencies alone do not compile every moved adapter. This is a native non-race build, not cross-architecture execution.

### V-FOCUS: relevant ordinary package tests

```sh
go test -ldflags=-checklinkname=0 -tags=linkname ./internal/compat/spec
```

Replace the package argument with the explicit task package list below, after checking those tests do not require prepared simulation workers. New focused fixture tests should select their own package/test names; do not trigger the large suite or all metatesting from a direct wildcard. For source/runtime changes that need translated execution, use V-ORDINARY with source-matched preparation, not bare `go test ./...`. Existing large-gated tests remain skipped; record that rather than weakening their gates.

### V-REVISIONS: active maintenance entry

Before Task 18, refresh active legacy profile revisions with:

```sh
go run -ldflags=-checklinkname=0 -tags=linkname ./internal/compat/genprofile
```

Task 5 broadens its hash coverage to shared inputs. From Task 18 onward use only:

```sh
go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen profiles -root=.
go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen profiles -root=. -check
```

Write-mode metadata generation may precede first origin creation; check mode requires complete origin metadata. Re-run write mode after executable generated bytes change, not merely for provenance refresh.

### V-GENERATION: separate exact-compiler checks

```sh
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=. -check
GOTOOLCHAIN=go1.26.9 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=. -check
```

Each process checks its own compiler/source only. No in-process cross-family parsing. Both compare full shared bytes with version-neutral headers; ignored records are not prerequisites. If intentional regeneration is needed, use the same command without `-check`, then re-run both checks. `-refresh-origin` is only for an intentional reviewed origin update/bootstrap. These commands inspect architecture/race-selected source contracts; they do not execute crossarch/race suites.

### V-ORDINARY: source-matched native and simulation validation

```sh
GOSIM_TOOLCHAIN=go1.25.14 TEST_REPORTS_DIR="$PWD/.gosim/compact-go-validation/go125" make test
GOSIM_TOOLCHAIN=go1.26.9 TEST_REPORTS_DIR="$PWD/.gosim/compact-go-validation/go126" make test
```

Use separate report roots so the second run cannot overwrite the first. Read each `unit/latest/report.md`, timeline and failing logs; record exit status, pass/fail/error/skip totals, exact compiler and tested source. `make test` creates/prepares the isolated snapshot. Do not claim execution coverage for skipped platforms, race or large tests. Fix observed failures before acceptance; broadening into restricted suites still needs an explicit user instruction.

### V-AUDIT: real known-family comparison

After Tasks 22–27, use a new output directory for each invocation:

```sh
GOTOOLCHAIN=go1.25.14 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen observe -profile=go125 -output=.gosim/compact-go-audit/go125-observation.json
GOTOOLCHAIN=go1.26.9 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen compare -parent=go125 -parent-observation=.gosim/compact-go-audit/go125-observation.json -output=.gosim/compact-go-audit/go126
GOTOOLCHAIN=go1.26.9 go run -ldflags=-checklinkname=0 -tags=linkname ./cmd/gosim-gen compare -parent=go125 -output=.gosim/compact-go-audit/go126-declarations
```

Create the parent output directory first. Never remove a previous report merely to reuse these example names; choose a fresh suffix. Verify known iterator/ArtifactDir/API and StoreReluintptr/clearenv/source findings as applicable. Declaration-only mode must say `skipped-no-parent`. Use fixtures for a hypothetical unregistered future family rather than claiming its compiler is available or enabling support.

### V-DOCS and final hygiene

Check Markdown links/fences and referenced command names. Format only changed Go files with gofmt; use repository formatting checks where applicable. Run `make lint` only for changed cmd/examples scope; no internal/all-project linter. Record actual production/test/doc/generated file counts and byte totals with paths, including new audit/tooling files. Review git diff and remove transition code/stubs, not historical evidence.

## Task List

### Task 1: Establish the generator command dispatch contract

**Files:** cmd/gosim-gen/main.go, commands.go, focused command tests.

**References:** target §6 command table/exits; §7 cmd interfaces; §8 command flows.

**Budget:** ≤300 authored Go lines. **Dependencies:** none.

- [x] Refactor process-global flag handling into `runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int`, preserving existing flag-only generation and `-check`/`-refresh-origin` exclusion. Dispatch subcommands before legacy `run()` or buildcfg.Resolve; profiles maintenance must not require supported-compiler discovery. It still requires a compiler capable of building the tool itself.
- [x] Add `runProfiles`, `runCompare`, `runObserve` with the same handler signature. Initially return an explicit nonzero unavailable error for new subcommands; route existing generation unchanged. Do not expose a successful placeholder.
- [x] Define help/argument contracts from target §6, including required destinations and stable-family terminology. Keep normal gosim application CLI unchanged.
- [x] Test dispatch, invalid arguments, errors reaching stderr and legacy generation argument preservation with injected command-level fakes; no generator execution is needed for argument tests.

#### Verification

- [x] V-BUILD, V-FOCUS for cmd/gosim-gen and changed-file `make lint` pass; existing behavior remains reachable and placeholders fail explicitly. Test that profiles dispatch never invokes normal compiler discovery.

### Task 2: Add strict family-definition types and decoding

**Files:** internal/compat/spec/definition.go and focused tests.

**References:** target §3 full schema, keys/normalization; §7 DecodeDefinition/LoadDefinitions; §11 DecodeDefinition.

**Budget:** ≤500 authored Go lines. **Dependencies:** 1.

- [x] Implement FamilyDefinition, RecordDelta, Declaration, PackageRef, ArchitectureDelta and ProfileDelta exactly as target §3 and the contract index, using existing spec records.
- [x] Implement `DecodeDefinition([]byte) (FamilyDefinition,error)` with recursive duplicate-key detection, unknown-field rejection, one-value EOF check, explicit required-field presence and canonical family/path validation.
- [x] Implement `LoadDefinitions(fs.FS) (map[string]FamilyDefinition,error)` with strict goNNN.json filename matching; ignore origin/evidence files intentionally and reject definition filename/family mismatch.
- [x] Normalize omitted/both architecture sets to common scope, reject duplicates/unsupported architectures, preserve layout/ordered payload order and canonicalize only sets.
- [x] Cover malformed nested JSON, absent false-valued supported, draft rejection, trailing bytes, invalid remove tuples and unsafe paths with focused table tests.

#### Verification

- [x] V-BUILD and V-FOCUS for internal/compat/spec pass; no current profile consumer changes yet.

### Task 3: Resolve cumulative deltas and immutable ancestry

**Files:** internal/compat/spec/resolve.go and focused tests.

**References:** target §3 operations/ancestry; §7 ResolvedFamily; §11 ResolveDefinitions/applyRecords.

**Budget:** ≤500 authored Go lines. **Dependencies:** 2.

- [x] Implement `ResolveDefinitions(map[string]FamilyDefinition,string) (ResolvedFamily,error)` and generic `applyRecords[T](base []T, delta RecordDelta[T], key func(T) []string) ([]T,error)`.
- [x] Require 1.25 root, immediate predecessor links, no cycles/missing parents, no duplicate family keys, and retained unsupported ancestors. Pure resolution must not require generated support admission.
- [x] Enforce absent Add, present Replace/Remove, no cross-operation duplicate key, whole-record replacement, redundant replacement error, and persistent removal until explicit re-add.
- [x] Add Clone and Canonical methods; deep-copy nested maps/slices, including layout fields and probe arguments. Compute chain DefinitionDigest deterministically.
- [x] Test three-layer inheritance, empty intermediate delta, retirement, split common declaration into disjoint arch records, collision after architecture expansion, and mutation isolation.

#### Verification

- [x] V-BUILD and spec tests pass; resolved results are independent of map iteration and current compiler family.

### Task 4: Materialize profile, policy and contract views

**Files:** spec/resolve.go, profile.go, contracts.go and focused tests.

**References:** target §3 derived views/nine-contract correction; §5 identity; §7 resolved methods; §11 Descriptor.

**Budget:** ≤500 authored Go lines. **Dependencies:** 3.

- [x] Add Profile.Chain and DefinitionDigest; implement `ResolvedFamily.Descriptor(arch,revision string) (Profile,error)`, PolicyManifest(), ShimManifest(arch), using existing record semantics.
- [x] Derive declaration category from Hook.Context and target category. Build BOTH amd64/arm64 ContractsByArch maps every time; only callable hooks select the requested architecture.
- [x] Keep metadata structural validation separable from supported-family admission so retained ancestors and unregistered pure fixtures resolve, while production Profile.Validate/selection remains strict.
- [x] Test derived policies/hooks/shims/contracts using fixtures, including all nine previously omitted shim contracts and unknown target/context rejection. Record their intentional coverage expansion, not false legacy equivalence.
- [x] Preserve ContractDigest and identity schemas; new fields must be deterministically populated wherever selection comparisons depend on them.

#### Verification

- [x] V-BUILD and spec tests pass for both baseline compilers. Existing runtime descriptors remain active.

### Task 5: Prepare path admission and fingerprints for incremental moves

**Files:** spec/paths.go, profile.go, policy.go, revision.go; translate/profile.go; compose validation; related tests.

**References:** target §§2, 5 package boundaries/fingerprints; §7 classifier/consumers; §8 ordinary flow.

**Budget:** ≤500 authored Go lines. **Dependencies:** 4.

- [x] Add `CompatPackageKind` and `ClassifyCompatPackage(path string)` for original/shared and relocated spellings; separate category recognition from exact import authorization.
- [x] Allow shared targets through the same reviewed metadata permissions while still recognizing the existing selected family's paths during migration. Do not admit another family's executable target or unknown compat metadata into translated code.
- [x] Adapt composition/translation path validation to support incremental package moves without adding forwarding packages or runtime family selection. Retain existing native islands, feature and provider rules.
- [x] Broaden the active SourceRevision input inventory to include shared compatibility paths as they appear, while old profiles remain active. Exclude all generated revision constants to avoid self-reference. Final origin-independent chain hash replaces this transitional inventory in Task 17.
- [x] Record base revision `550d5e3`, original counts and inventory acquisition command for the one-time migration comparison in Task 15. Optional ignored captures are maintainer evidence only; no committed test may read them or require Git history. No committed expanded-profile copy.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused spec/translate/compose boundary tests pass. Test inactive-family rejection and shared paths requiring exact permissions.

### Task 6: Select only active shim source bindings

**Files:** spec/shims.go, source-selection helpers, declaration_ledger_test.go, runtime_families_test.go and focused tests.

**References:** target §3 binding defect; §7 VerifyShimBindings; §11 binding/destination selection.

**Budget:** ≤450 authored Go lines. **Dependencies:** 5.

- [x] Implement a private explicit go/build.Context selector with family release tags, OS/arch/filename tags and approved source predicates. Do not inherit the maintenance process's release tags.
- [x] Use selected files in existing shim checks; update callers atomically. Final public signature is `VerifyShimBindings(root string, r ResolvedFamily, source SourceContext) error`; while old descriptors remain active, share the private selection implementation rather than inventing a public compatibility facade.
- [x] Preserve duplicate rejection in readHookDeclarations; inactive files must neither satisfy required bindings nor trigger false duplicates.
- [x] Replace TestExecutableAdaptersHaveBoundedFamilySelection and ownedDeclarations traversal in declaration_ledger_test.go with family/context-aware selected-declaration checks. Inspect shared roots plus still-live old roots during migration; require nonempty expected inventories, complete bindings and no duplicate selected declaration. Common files may serve both families and additions/replacements must select only their intended contexts; do not require every variant to select in every family or prohibit cumulative 1.27 source selection. Test unsupported-family guard reachability separately. Preserve the immutable declaration_ledger.json payload/digests.
- [x] Prepare runtime_families_test.go to validate selected signatures/permissions rather than physical `/<profile>/` ownership; Task 9 updates its exact target expectations when paths move. Test selected/inactive variants, methods, filename predicates and hypothetical next-minor inheritance. Translated checks are cgo-off; preserve native race source context separately.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused spec binding/source tests pass under both families. No race build or execution.

### Task 7: Separate generation rendering from publication and checking

**Files:** routegen/generate.go, check.go, adapters.go, layouts.go, current_test.go and focused tests.

**References:** target §4 neutral provenance; §7 routegen; §8 generation; §11 GenerateProfile/CheckProfile.

**Budget:** ≤500 authored Go lines. **Dependencies:** 6.

- [x] Factor rendering into a pure output map stage that receives current profile inputs; CheckProfile compares only committed selected outputs without requiring ignored records or writing final outputs.
- [x] Enforce running compiler equals inspected compiler before parsing. Keep each test/process on its selected compiler; update tests that previously implied cross-family parsing.
- [x] Replace exact-version generated source comments with version-neutral text. Keep exact version/fingerprints in origin/generation records. Remove test normalization that would hide source-header discrepancies. Keep legacy committed profile/routes_*.json frozen only until Task 18 removes the old embeds/trees; stop treating them as current outputs. Update routegen/family_test.go, current_test.go, go126_test.go and generate_test.go to verify in-memory rendered/ignored records and independent other-origin/architecture isolation instead of old routes-file ownership.
- [x] Add `selectedGeneratedPath(root,packagePath,stem string,source SourceContext,family string) (string,error)` using Task 6's selection semantics and one selected destination requirement. Rendering preserves authored constraints instead of recomputing a family ceiling.
- [x] Until moves complete, support selected existing destinations through explicit current profile targets; no guessed shared path or automatic copied tree. Resolve authored destination selection from the input source root independently of the publication output root, so an empty temporary output tree still works. Update generator expectations and regenerate affected comments for both families.

#### Verification

- [x] V-REVISIONS, V-BUILD, routegen focused tests and V-GENERATION pass. Check mode leaves mtimes/bytes unchanged and works with ignored generation records absent.

### Task 8: Consolidate native host ABI and its facades

**Files:** compat/{go125,go126}/host → compat/host; hostabi facades/guards/tests; current profile target records.

**References:** target §2 source inheritance/guards; §4 provenance; §7 hostabi; final design-review Q1.

**Budget:** ≤350 authored Go lines plus verified moves/deletions. **Dependencies:** 7.

- [x] Move the identical host implementations once, delete the verified duplicate, preserve unsafe/linkname/race semantics and update hostabi imports and profile native target paths together. Rewrite only Gosim-side Hook.Target fields inside each origin file to the shared host path; preserve upstream paths/signatures/file digests/exact baseline. These target annotations are not upstream content hashes. Test the unchanged provenance fields explicitly.
- [x] Add direct compat/host unsupported guard using the current supported intervals; keep hostabi/selected rejection. Task 18 takes over guard generation.
- [x] Remove union support ceilings from the six shared hostabi facade/test files named in target §2; preserve fixed floor and race predicates. Collapse identical family facade files into shared implementations without forwarding packages.
- [x] Update linkname/closure tests and confirm maps/synctest leaf packages do not need native metadata imports to reject unsupported simulations.

#### Verification

- [x] V-REVISIONS, V-BUILD under both families, focused hostabi/spec checks and V-GENERATION pass. Record source-only guard checks separately from execution.

### Task 9: Consolidate runtime hooks and their runtimebridge dependency

**Files:** both hooks/rt and shims/runtimebridge copies → shared packages; profile hooks/bridges/permissions; runtime_families_test.go, hooks_test.go and root shim guard-test imports.

**References:** target §2 variants; §3 declarations; §5 permissions; §9 behavior preservation.

**Budget:** ≤400 authored Go lines plus verified moves/deletions. **Dependencies:** 8.

- [x] Move normalized-identical shims/runtimebridge together with hooks/rt: runtime.go/race.go import that package, so a shared hook cannot retain either bounded family import. Update both families' runtime/internal-race bridge rewrites and root shim guard-test imports atomically. Preserve the full test union and non-release predicates.
- [x] Move common runtime hook bodies once and normalize imports; replace hooks_test.go/runtime_families_test.go physical-family target assertions with shared target plus selected signature/permission assertions.
- [x] Extract StoreReluintptr into the two small signature variants: bounded baseline and cumulative go1.26 replacement. Do not unify incompatible return signatures with runtime branching.
- [x] Update both active family metadata sets, runtime bridge references and exact sync/atomic/native permissions in the same task; preserve simulator scheduling/atomic semantics.
- [x] Test family-specific signature/source selection and inherited 1.26 selection for a hypothetical 1.27 context, without declaring support for it.

#### Verification

- [x] V-REVISIONS, both-family V-BUILD and relevant native hook/spec checks pass. Use V-ORDINARY if changed translated behavior requires execution; do not run sim-only tests as native coverage.

### Task 10: Consolidate kernel, filesystem and network hooks

**Files:** both families' hooks/{kernel,fs,net} → shared hooks; compose imports; routegen destinations; profile records/tests.

**References:** target §2 clearenv/shared layouts; §§5, 7 composition/generation; §9 retained tests.

**Budget:** ≤500 authored Go lines plus verified generated moves/deletions. **Dependencies:** 9.

- [x] Move the coupled hook packages together to preserve their kernel layout imports. Keep generated architecture-specific syscall/layout files shared and selected by existing architecture predicates.
- [x] Extract clearenv as a go1.26 addition; keep common process/transport code and the union of statfs/GID assertions.
- [x] Update composition, selected target metadata, generator expected paths and canonical imports atomically. Preserve exact route IDs, provider isolation and disabled-feature fatal behavior.
- [x] Use Task 7 destination selection for shared generated files; neutral headers and authored constraints must byte-match under independent family checks.

#### Verification

- [x] V-REVISIONS, both-family V-BUILD, routegen/spec focused checks and V-GENERATION pass; run V-ORDINARY for the hook/composition transition and record report totals.

### Task 11: Consolidate reflect while preserving each method surface

**Files:** both families' shims/reflect → shared shims/reflect; active profile shim/bridge records; boundary tests.

**References:** target §2 reflect variant gate; §3 contracts; §9 embedded-interface validation.

**Budget:** ≤450 authored Go lines plus verified moves/deletions. **Dependencies:** 10.

- [x] Preserve shared reflect bodies/type identity and isolate 1.26 Type/Value iterator additions under cumulative constraints.
- [x] Try the target's small extension-interface composition only with a meaningful translated-interface fixture. If unsupported, use the allowed small whole-interface variants and record that outcome; do not alter translator semantics solely to avoid a short duplicate declaration.
- [x] Update active bridge replacements, shim targets and native reflect/sync/iter permissions together; reject native object/type-universe escapes.
- [x] Preserve shared implementation assertions and add focused tests for exact selected interfaces and iterator behavior where ordinary simulation supports them.

#### Verification

- [x] V-REVISIONS, both-family V-BUILD, spec/boundary checks and ordinary prepared translated fixture validation pass. Record whether embedding or short interface variants were selected.

### Task 12: Consolidate testing with small API and temporary-root variants

**Files:** both families' shims/testing → shared shims/testing; compose references; active metadata/tests.

**References:** target §2 testing state decision; §§3, 7 methods; §9 behavioral validation.

**Budget:** ≤450 authored Go lines plus verified moves/deletions. **Dependencies:** 11.

- [x] Keep the common struct and both Gosim-owned artifact fields shared. Do not add a familyTestState embedding or duplicate the entire testing implementation.
- [x] Select the TB method surface, ArtifactDir method and GOTMPDIR behavior with small authored constraints; retain 1.25 semantics and inherited 1.26 behavior.
- [x] Update composition and both family metadata/binding expectations together. Apply the same verified interface strategy as Task 11.
- [x] Preserve tests from both copies and add focused artifact/temp-root regressions through ordinary prepared simulation.

#### Verification

- [x] V-REVISIONS, both-family V-BUILD and relevant ordinary prepared tests pass; disabled feature behavior remains unchanged.

### Task 13: Consolidate crypto/rand's version-sensitive reader choice

**Files:** both shims/cryptorand packages → shared package; active policies/contracts/tests.

**References:** target §1 measured difference; §2 crypto variant; §6 behavioral source review.

**Budget:** ≤350 authored Go lines plus verified moves/deletions. **Dependencies:** 12.

- [x] Share the existing prime algorithm and common rand/text code; isolate only the version-sensitive reader choice as specified by existing behavior.
- [x] Preserve 1.25 reader semantics and 1.26 custom-reader/GODEBUG behavior exactly; do not introduce host randomness fallbacks.
- [x] Update replacement/contract paths and preserve deterministic-reader tests and known feature behavior.

#### Verification

- [x] V-REVISIONS, both-family V-BUILD and relevant native/ordinary simulation tests pass. Do not use lint-driven crypto algorithm changes.

### Task 14: Consolidate remaining unchanged shims and test guards

**Files:** remaining shims including debug, exec, httppprof, maps, metrics, ocsp, pprof, signal, syncmap, synctest, tls, user, x509 (runtimebridge moved in Task 9); shared guard tests.

**References:** target §2 layout; §3 derived categories; §5 boundaries; §9 union-of-tests requirement.

**Budget:** ≤450 authored Go lines; process mechanical moves in package-sized batches. **Dependencies:** 13.

- [x] Move each normalized-identical package once, verify and delete its second implementation, and update imports/profile targets as a coherent package batch. Keep genuine differing declarations bounded rather than erasing them.
- [x] Preserve complete package replacements, guarded aborts and exact native permissions. Sparse contract ledgers must not change bridge classification.
- [x] Consolidate root guard and synctest tests without dropping assertions or compiling sim-only tests as native proofs. Their imports have already followed moved dependencies in each earlier task; runtimebridge itself was moved with runtime hooks in Task 9.
- [x] Confirm there are no executable hooks/shims/host left in old family directories; old metadata remains temporarily until cutover.

#### Verification

- [x] Each mechanical batch compiles with relevant focused checks; task completion requires V-REVISIONS, both-family V-BUILD, V-GENERATION and ordinary validation of the fully shared executable tree.

### Task 15: Convert existing metadata to baseline and cumulative delta

**Files:** new profiles/go125.json, go126.json; durable spec resolution/contract tests; one-time base-revision comparison artifact.

**References:** target §3 complete schema/key rules; §4 origin separation; §9 migration/accounting.

**Budget:** ≤450 authored Go/test lines; JSON is generated/reviewed data. **Dependencies:** 14.

- [x] Convert current reviewed metadata into one full baseline and only actual 1.26 replacements/additions/removals. Normalize already-shared paths; derive architecture applicability and remove redundant contracts/hooks/shims duplication.
- [x] Use a bounded one-time conversion/comparison command under ignored migration output, not a committed test or permanent second generator. Read original family metadata directly from base commit `550d5e3` with `git show`/a temporary archive, materialize converted records with the resolver, and record exact command, revision, normalization rules, differences and exit status in this plan. Review every delta.
- [x] Run the one-time canonical old-to-new semantic comparison after path normalization; explicitly account for nine added shim contracts, Gosim-side origin target annotations and any genuinely changed metadata. Re-run it before old-tree deletion in Task 18 if intervening edits change metadata. Do not wire this base-history/ignored-artifact dependency into tests, CI or ordinary snapshots.
- [x] Assert empty future delta inherits all intermediate changes in fixtures. Keep future-family fixture support isolated from the actual registry.

#### Verification

- [x] V-REVISIONS, V-BUILD and durable spec resolution/contract tests pass, plus the one-time comparison succeeds with recorded explained differences. Committed tests remain self-contained after old trees and ignored captures disappear. Existing callers still use old metadata until the facade is ready.

#### Conversion evidence (2026-10-21)

Original source: commit `550d5e3`, read using `git show` by the bounded ignored helper. Commands, each exit 0:

```sh
python3 -B .gosim/compact-go-migration/convert.py
go run -ldflags=-checklinkname=0 -tags=linkname ./.gosim/compact-go-migration/verify.go
```

Normalization replaces family-owned hooks/shims/host target paths with shared paths, changes the two selected-version shim reason labels to “selected Go shim”, and reconciles equivalent maps-policy reason prose. Record sets, native symbols, rewrites and set-valued probe arguments are sorted; ordered layout fields and signatures are preserved. Original/current origins, tuples and evidence were compared independently; only previously documented Gosim target annotations change in origins, with upstream identity untouched.

Both resolved families match original hooks, shims, policies, runtime packages and both architecture contracts after those normalizations and the nine explicitly added contracts from target §3: tls CipherSuites/InsecureCipherSuites; metrics All/Read/Value.Kind/Uint64/Float64/Float64Histogram; debug ReadGCStats. Actual compiler contract inspection passes for both baselines. This is semantic comparison with declared differences, not byte equality.

Reviewed the complete go126 delta: eight declaration additions (six reflect iterators, clearenv, ArtifactDir); three replacements (StoreReluintptr and the two ABI function-PC signature spellings); eight package additions/three removals; four probe additions; three capability additions/two replacements. Other sections inherit unchanged. Baseline: 367,659 bytes; delta: 8,894 bytes. The two ABI signatures retain the original interface{}/any textual difference; the capability replacements preserve original crypto/rand behavior/reason differences.

Durable tests use only committed definitions and frozen semantic digests, not git history or ignored captures. They also verify empty 1.27 inheritance without registering it. Both baseline spec suites and command/compatibility/hostabi builds pass; revisions refreshed and formatting passes. Old metadata remains the live authority. Repeat the one-time comparison unconditionally immediately before Task 18 deletion (strengthened after checkpoint 2).

### Task 16: Add the profiles facade and independent on-disk provenance

**Files:** profiles/profiles.go, initial revisions_gen.go; spec origin loader; old profile embed/VerificationOrigin and catalog callback; routegen origin reads/writes/tests; profiles/*.origin.json, evidence.json; Go/Python exclusions/tests.

**References:** target §4 origin/evidence; §5 hashes/cache; §7 profiles/LoadOrigin; §8 metadata flows.

**Budget:** ≤500 authored Go/test lines plus lossless data moves. **Dependencies:** 15.

- [x] Implement Definitions, Resolve, Descriptor, Policy, Shims and Revision signatures from target §7; embed only `go*[0-9].json`, use lazy immutable metadata and return independent values.
- [x] Implement `LoadOrigin(fs.FS,profileID string) (Origin,error)` for checkout readers. COPY complete origin payloads to the new independent paths first; do not remove an embedded file while its old profile still compiles. Preserve Task 8's normalized Gosim targets and all upstream provenance.
- [x] In the same task remove origin.json from both old embed lists, change their temporary VerificationOrigin callback to take an explicit repository root and return errors, and update catalog.Origin plus every caller/test accordingly. Read with LoadOrigin at the source root; do not infer it from arbitrary cwd. Routegen must resolve source root before loading origin and publish RefreshOrigin only at `internal/compat/profiles/goNNN.origin.json` relative to output root. Old origin copies remain frozen/unread until deletion in Task 18. There must be only one live read/write authority, so refresh between Tasks 16 and 19 cannot drift two copies.
- [x] Preserve the complete old evidence/tuple JSON values in the consolidated schema-1 history envelope. During coexistence exclude both old evidence paths and the new exact path from executable identities; never exclude origins from whole-source hashes.
- [x] Seed the new revisions output from current valid values only as an unused transition artifact; Task 17 replaces it using the final algorithm before production selection switches. New facade tests use explicit fixture revisions where appropriate and cannot claim cutover readiness yet.
- [x] Test no origin/evidence runtime embed, cloned records, origin filename/family validation and lossless historical payload migration; update Python exclusion unit coverage without invoking certification.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused profiles/spec/buildcfg plus Python metadata utility checks pass. Ordinary active descriptors still work.

### Task 17: Implement final source revisions and support rendering

**Files:** profiles/generate.go, revisions_gen.go; spec/revision.go; buildspec generated table renderer and tests.

**References:** target §5 fingerprint scopes; §7 GenerateMetadata/renderSupport/renderRevisions; §11 maintenance/revision pseudocode.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 16.

- [x] Implement origin-independent SourceRevision with target domain, canonical chain/resolved data and conservative source inventory; exclude origins, evidence, tests/testdata and every generated revision output. Keep whole-source TranslatorDigest origin-sensitive.
- [x] Implement renderSupport and renderRevisions with on-disk definitions and no eager production lookup. Render exact supported intervals including gaps, selectedProfile constants and host/hostabi/selected guards. Validate retained ancestors separately from support.
- [x] Implement GenerateMetadata write/check ordering: support outputs first, then revisions; check mode additionally validates complete structural origin metadata without loading every compiler.
- [x] Exercise rendering/publication in temporary fixture repositories at this step. Do not overwrite the active old selected functions before Task 18 replaces them with shared methods. The new revision table may be produced independently; refresh active old revision constants as needed for intermediate tests.
- [x] Test origin-only edit leaves AdapterRevision unchanged but changes whole-source identity, evidence-only edits leave both executable hashes unchanged, ancestor/shared-source edits invalidate, and missing first origin permits write-mode revisions but fails final check.

#### Verification

- [x] V-BUILD and focused profiles/spec/buildcfg tests pass. Fixture support rendering is deterministic, idempotent and independent of stale compiled registry entries.

### Task 18: Switch production selection and remove duplicate metadata entry points

**Files:** selected/selection.go and selectors; buildspec/family.go/families_gen.go; cmd/gosim-gen profiles handler; Makefile go-generate; catalog callers/tests; old profile/genprofile removal.

**References:** target §§3, 5 admission, §7 selected/profiles/buildspec, §8 bootstrap; all associated method signatures.

**Budget:** ≤500 authored Go lines plus generated data/deletion. **Dependencies:** 17.

- [x] Move Profile/descriptorForArch/policyManifest into shared selection.go and use generated selectedProfile constants. Make policy errors explicit and preserve complete canonical selection comparisons.
- [x] Move FamilySupport to `{Family,Profile string}`, remove unused patch minimum/exclusions, and source the support table from definitions. Write the rendered families_gen.go in the SAME edit that removes the hand-maintained table, using Task 17's renderer/one-time output; retain a syntactically valid revisions_gen.go too. The tool cannot bootstrap a missing/corrupt file it must itself compile. A stale but valid generated file remains repairable. Update affected constructors/tests atomically, then run profiles and its check.
- [x] Wire `runProfiles` to GenerateMetadata. Now generate final support files/revisions in the real tree and verify agreement. The generator dependency closure must remain metadata-only.
- [x] Replace catalog and version-profile imports in production and compile-required fixtures with profiles functions and LoadOrigin from the checkout. Update currentGeneration/test helpers and origin reads together; no callback/forwarding package survives.
- [x] Update declaration_ledger_test.go/runtime_families_test.go to use final shared roots/resolved profiles, with nonempty inventories; update all routegen family/current/generate/go126 tests to final record/origin paths. Always re-run Task 15's one-time semantic comparison immediately before deleting the old trees, even when metadata appears unchanged: `python3 -B .gosim/compact-go-migration/convert.py --check` (compares without overwriting definitions), then `go run -ldflags=-checklinkname=0 -tags=linkname ./.gosim/compact-go-migration/verify.go`. Remove the migration-only opaque semantic hash oracle at cutover, retaining the real-compiler contracts and inheritance tests; do not update that oracle to accept drift. Remove catalog, genprofile and the old family profile source/data once all references are migrated. Retain the consolidated history and independent origins. Remove old revision files so the final source fingerprint cannot include stale generated outputs, then regenerate final revisions.
- [x] Update Makefile go-generate to run profiles write mode before family generators (new support bootstrap), both matching-compiler generators, then profiles write/check after generated source changes; preserve filesystem-model stringer generation. The real first-origin flow still uses explicit RefreshOrigin, not an automatic missing-origin bypass.
- [x] Remove legacy evidence exclusions only after their files are gone; keep the exact consolidated evidence exclusion and all origin inputs in TranslatorDigest/Python identities.

#### Verification

- [x] V-REVISIONS using only the new entry, both-family V-BUILD, profiles/spec/buildcfg/routegen focused tests, V-GENERATION and changed-file make lint pass. No code reference to the retired packages or active old descriptor remains.

### Task 19: Finish shared generation and first-origin bootstrap

**Files:** routegen/generate.go, check.go, adapters.go, layouts.go; cmd legacy generation wiring; generation tests.

**References:** target §7 routegen final signatures; §8 generation/bootstrap; §11 GenerateProfile/CheckProfile.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 18.

- [x] Finalize `renderProfileOutputs(tc,r,origin,arches,root)` using ResolvedFamily and selected authored destination files. Retain strict module/contracts/bindings checks and independent process identity.
- [x] Explicit RefreshOrigin may seed only a missing new-family origin with schema/family/current exact baseline, then replace it after successful checks. Normal generation/check requires a complete existing origin.
- [x] Use the same SourceRevision as profiles; delete any provisional generation-only revision algorithm. Rebuilt gosim-gen sees newly generated support tables before normal contract validation.
- [x] Write only selected shared outputs, this family's origin and ignored provenance records; refresh of one origin must not modify another. Shared writes are accepted only after every supported selecting family's check succeeds.
- [x] Exercise actual first-origin bootstrap with the CURRENT running compiler from ResolveRunningProcess, following routegen/family_test.go. In a temporary source copy remove only that family's origin, run profiles write mode, require profiles -check to reject missing provenance, run explicit -refresh-origin with the SAME compiler, regenerate profiles if source bytes changed, then require profiles/check and generation check to pass. Refresh records the running patch even when it differs from the repository baseline. Assert the other family's origin bytes are unchanged. Do not launch/download another family from an ordinary test. Keep hypothetical unchanged-family fixtures separate.

#### Verification

- [x] V-REVISIONS, both-family V-BUILD, routegen tests and V-GENERATION pass. Check mode needs no ignored artifact and performs no publication.

### Task 20: Remove migration admission and complete consumer-path cleanup

**Files:** spec/paths.go/profile.go/policy.go; translate/compose checks; remaining path/string fixtures and maintenance messages.

**References:** target §5 strict final boundaries; §7 consumer inventory; §9 retirement.

**Budget:** ≤400 authored Go/test lines. **Dependencies:** 19.

- [x] Remove transitional old-family path acceptance and old fingerprint branches. Only final shared categories and exact reviewed permissions remain.
- [x] Search all source/scripts/fixtures for old executable paths, catalog/genprofile, old profile filenames and physical-family selection assumptions. Historical review/evidence text is not executable migration debt and must not be rewritten as new evidence.
- [x] Complete call-site error handling for final VerifyShimBindings/Descriptor/Policy signatures; remove temporary private adapters only needed for old descriptors.
- [x] Assert no forwarding packages, expanded metadata copies, stale ceilings, old revision generators or unconsumed active definitions remain.

#### Verification

- [x] V-REVISIONS, both-family V-BUILD and focused spec/translate/compose tests pass. Final-path negative fixtures reject retired/unknown metadata imports.

### Task 21: Enforce the compiled graph's native-island boundary

**Files:** internal/buildrun/build.go and new focused graph tests; related translation fixture.

**References:** target §5 node/edge rules; §7 buildrun; §8 failure table; final peer review Q2.

**Budget:** ≤450 authored Go/test lines. **Dependencies:** 20.

- [x] Classify original and relocated compat node spellings explicitly. Allow native host/selected/profiles/spec only as native infrastructure; never infer translated admission from their native presence.
- [x] Allow relocated-to-original Gosim imports only to rt/runtime and rt/runtime/abi, preserve standard-library/external exact native permissions separately, and reject every original compat/hooks or compat/shims node.
- [x] Preserve existing provider/feature graph checks. Test forbidden hostabi/buildspec imports outside the compat subtree as well as forbidden metadata paths.
- [x] Add a focused ordinary translated-output fixture that exercises real generator output and legitimate native runtime dependencies, plus synthetic negative graph fixtures. Do not repurpose the large selected-translation suite or unset its guard.

#### Verification

- [x] V-REVISIONS, V-BUILD and buildrun focused tests pass; run the new fixture through ordinary prepared validation. A real valid graph must pass before accepting the positive allowlist.

### Task 22: Add narrow stable compiler inspection

**Files:** buildspec/command.go; buildcfg/inspection.go, discovery/fingerprint helpers and tests.

**References:** target §6 inspection boundary; §7 inspection signatures; §11 DiscoverInspection.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 21.

- [x] Implement SourceInspectionCommand restricted to required env/version/list queries with canonical stable versions, installation/environment controls and local exact toolchain execution. Reject build/run/test and arbitrary flags in this entry.
- [x] Implement InspectionCompiler state plus DiscoverInspection, Toolchain, Fingerprints and Recheck. Require running compiler equals inspected exact version and verified installation; an explicit override is an equality assertion.
- [x] Factor private discovery/fingerprint helpers without weakening normal Discover/Fingerprint/OutputCommand/ApplicationCommand family checks. Unknown stable inspection succeeds only on the inspection path; prerelease/devel/custom still fail.
- [x] Test mismatched executable/runtime identity, changed installation, unsupported family ordinary rejection and inspection-only acceptance using controlled fixtures. Task 23 adds full module-preparation coverage, not merely constructor tests. Do not simulate support by editing the real registry.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused buildspec/buildcfg tests pass; normal caller admission behavior remains unchanged.

### Task 23: Prepare private modules through the inspection command path

**Files:** buildspec/command.go, modules.go, modulefiles.go; spec/hooks.go private inspection workspace helper; buildcfg discovery/source fixtures.

**References:** target §6 inspection/private module boundary; §7 source inspection/module signatures; §8 comparison failure behavior; §11 Observe/Compare prerequisites.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 22.

- [x] Factor module graph decoding and command selection privately. Add fixed inspection wrappers `SourceInspectionApplicationCommand(ctx context.Context, r Resolution, args ...string) *exec.Cmd` and `ResolveInspectionModuleGraph(ctx context.Context, r Resolution) ([]Module,error)` in buildspec. Do not export an injectable command factory or weaken normal ApplicationCommand/ResolveModuleGraph.
- [x] Preserve caller GOWORK/module/read-only semantics for env GOMOD and list -m -json all; the generated private context sets GOWORK=off and permits only the existing needed source list -mod=mod behavior. Split ModuleFiles into a private renderer with canonical stable parsing and a strict public wrapper retaining ValidateVersion. Add fixed `SourceInspectionModuleFiles(r Resolution, modules []Module, modulePath string) (map[string][]byte,error)` which calls that renderer after stable-only validation; spec preparation uses this wrapper, since it cannot call an unexported buildspec function. Preserve generated go directive/version, selected replacements, sums and graph semantics. MatchModuleGraph remains shared unchanged. Validate selected graph equality before inspection.
- [x] Add fixed `spec.PrepareSourceInspectionContext(ctx context.Context, r buildspec.Resolution) (string,func() error,error)` and refactor hookInspectionContext internally so ordinary verification still uses strict commands/ModuleFiles while audit preparation uses the fixed inspection wrappers for EVERY caller query, private-module rendering and private-module query. The whole path must accept an unregistered stable family before source parsing without permitting build/run/test.
- [x] Add a focused source/dependency boundary test restricting production references to inspection wrappers (including SourceInspectionModuleFiles) to their buildspec implementation, spec inspection source/preparation, buildcfg inspection and audit code. Reject use from ordinary selection/build/contract paths; allow tests explicitly. This preserves strict admission as new callers appear.
- [x] Extend fakeDiscoveryGo-style fixtures through caller/private module graphs, list responses, a small source parse, workspace/local replacement, command/env capture and cleanup errors. Keep fake identity tests honest: a fake future version cannot pass production runtime.Version equality. Test mismatch rejection separately, and use the real current compiler for the full discovery→private-module→collector path. Task 24 first exercises the full collector chain using a real-compiler registry-omitted fixture; Task 28 completes the CLI assertion. No production version override or real-checkout registry change is allowed.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused buildspec/buildcfg/spec tests pass. A full preparation-chain fixture succeeds for an unregistered stable identity only on inspection entry points; normal calls still reject it and caller module files remain unchanged.

### Task 24.1: Collect bounded source observations with the matching compiler

**Files:** spec/source.go and observation types/collector; private module preparation from Task 23; origin inspection reuse; focused source tests.

**References:** target §§6–7 ObservationRequest/ObservationSet/CollectSourceObservations; §11 collector.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 23. Task 24 is split before execution to keep the collector and the real private-copy harness within separate authored-code budgets.

- [x] Add the exact observation/context schemas from target §6 in spec; no duplicate audit wire structs. Add ObservationRequest with resolved parent, origin, contexts, IncludeAPIs and IncludeStdlibInventory.
- [x] Refactor the source loader with private command selection. Export `CollectSourceObservations(ctx,tc,dir,req) (ObservationSet,error)` using fixed SourceInspectionCommand, independently enforcing stable/running identity and installation/environment rules.
- [x] Collect declared symbols, layouts/constants, package inventory and tracked/selected source hashes for explicit contexts; use the inspection-safe private module preparation implemented in the preceding task, preserving cleanup and no caller module rewrite. Connect the full discovery→caller module→private module→source list→parse regression through the collector.
- [x] Record context errors and missingness correctly: failed package loading is incomplete, never a missing-symbol record. Preserve native race cgo-on versus translated cgo-off selection without building those race contexts.
- [x] Test signature/layout/constant absence/change and source hash drift, errors/cancellation and deterministic records. Ordinary VerifyContracts continues using strict OutputCommand.


#### Verification

- [x] V-REVISIONS, V-BUILD and focused spec/buildcfg tests pass under each running family compiler; no in-process other-family parsing.

### Task 24.2: Verify observation through a real unregistered compiler fixture

**Files:** focused spec/buildcfg source fixtures and private-copy harness.

**References:** target §§6–7, §11 collector; Task 28 fixture consistency rules.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 24.1.

- [x] Run the real unregistered-admission smoke fixture as soon as this collector exists (Task 28 describes the fixture consistency rules). A fixture-built minimal inspection harness may call the collector before compare is wired. Use the running go1.26 compiler only, one non-race translated host-architecture context, no API snapshot and a 5-minute timeout with cancellation cleanup. Test an ordinary admission call inside the SAME fixture-built harness so it uses the omitted registry, not the parent test binary's registry. Record elapsed time and execute it in the go1.26 ordinary job; the go1.25 job explicitly skips this candidate-specific case.
- [x] Exercise complete discovery, caller/private module graph, source listing and parsing through the public collector; keep the caller module unchanged and ordinary admission strict.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused observation tests pass under both compilers; the real registry-omitted harness executes under Go 1.26 and is explicitly skipped under Go 1.25. Record elapsed time and actual versus fixture coverage.

### Task 25.1: Collect finite upstream API inventories

**Files:** spec API observation helpers and focused tests.

**References:** target §6 API scope and observation schemas; §7 collector; §11 Observe prerequisites.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 24.2. Task 25 is split before execution into three bounded, buildable commits.

- [x] Derive the finite API package inventory from all parent declarations and reviewed bridges, including whole-package replacements and symbol bridges. Exclude Gosim implementation packages and unrelated policy wildcards.
- [x] Enumerate exported declarations and value/pointer method sets with the current compiler, record APIPackages, preserve context errors and deterministic records. Remove the temporary API-request error from Task 24.1.
- [x] Verify actual inventories, changed/added/removed APIs, method sets, declaration-only skipping and failed API loads under both compilers.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused spec/API tests pass under both compilers; no in-process other-family parsing.

### Task 25.2.1: Bind inspector implementation and module inputs

**Files:** audit identity/workspace helpers and tests.

**References:** target §6 InspectorDigest/ModuleInputs and private-module boundary; §11 Observe/Compare input snapshots.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 25.1. Split hashing from workspace preparation before execution to preserve the authored-code limit.

- [x] Implement InspectorDigest over the non-test Go inspection implementation dependency closure, including request construction; exclude definition/origin/evidence data and generated registry/selector/guard/revision artifacts. Make it independent of the compiler building the inspector.
- [x] Bind resolved external module graph, module files and relevant replacement source inputs separately. Do not hash the whole Gosim tree into ModuleInputs or reintroduce excluded onboarding-data/support dependencies.
- [x] Prove inspector/source/module drift changes identity while target data/support-only edits do not; keep persisted module identity independent of generated private directories and compiler-specific go directives.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused audit identity/workspace tests pass; snapshots use inspection-only commands and never enable support.

### Task 25.2.2: Prepare and recheck audit workspaces

**Files:** audit workspace helper and focused tests.

**References:** target §6 private module and snapshot boundary; §11 Observe/Compare.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 25.2.1.

- [x] Resolve caller context through fixed inspection wrappers, preserve its selected graph, and prepare a private source module. Resolve inspection dependencies in that copy before snapshotting generated files, then require graph equality.
- [x] Bind inspector and external module inputs using Task 25.2.1; snapshot/recheck caller and private module inputs without adding compiler-specific private go directives to persisted parent/candidate identity.
- [x] Reject drift and propagate cleanup errors. Test workspace replacements, caller immutability, preparation failure cleanup and target-data/support-only stability.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused audit workspace tests pass under both compilers.

### Task 25.3: Capture and validate parent observations

**Files:** audit/compare.go snapshot types/Observe and tests.

**References:** target §6 ParentObservation and validation; §7 Observe; §11 Observe.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 25.2.2.

- [x] Implement the exact ParentObservation schema (reuse spec observation records) and ObserveRequest/Observe at the parent's recorded exact compiler.
- [x] Require complete requested API/source observations and unchanged compiler, inspector and module inputs before returning a usable snapshot; join cleanup failures.
- [x] Validate snapshot schema/completeness, family, exact baseline, digests and requested package/context inventories; reject stale or mismatched snapshots explicitly.
- [x] Test target definition/support-only changes preserve usability while parent/origin/inspector/module edits invalidate; Observe remains data-only.

#### Verification

- [x] V-REVISIONS, V-BUILD and spec/audit snapshot tests pass under both compilers, with complete actual parent observations.

### Task 26: Compute actionable comparison findings and draft operations

**Files:** audit/compare.go and focused diff/proposal tests.

**References:** target §6 report/draft schema and coverage; §7 Compare/CompareObservations; §11 comparison algorithms.

**Budget:** ≤550 authored Go/test lines (batched with 25.3 per user direction). **Dependencies:** 25.3.

- [x] Implement CompareRequest, ComparisonReport, Finding, DeltaProposal and DraftDelta exactly as target §6; use spec observation records without mirrors.
- [x] Implement Compare and CompareObservations, enforcing stable immediate next minor, complete parent resolution and optional validated parent snapshot; no hidden parent compiler download.
- [x] With a snapshot, report actual upstream API changes. Without one, inspect declared contracts/source and report APIDiff skipped-no-parent without an old-API finding flood. Record incomplete requested API scans distinctly.
- [x] Propose only unambiguous typed add/replace/remove operations. Mark missing implementations and source-only behavior review unresolved; no automatically approved origin, support declaration or executable patch.
- [x] Sort deterministic findings/proposals, keep completed differences separate from operational errors, and recheck compiler/module/source identities before assigning Complete.

#### Verification

- [x] V-REVISIONS, V-BUILD and audit tests pass for empty delta, inherited parent changes, incompatibilities, skipped API coverage, incomplete contexts, stale snapshots and non-installable draft decoding.

### Task 27: Publish audit artifacts and wire all generator handlers

**Files:** audit/output.go; cmd/gosim-gen/commands.go; focused publication/CLI tests.

**References:** target §6 publication/exits; §7 CLI/output signatures; §11 WriteComparison.

**Budget:** ≤500 authored Go/test lines. **Dependencies:** 26.

- [x] Implement RenderReport, WriteComparison and WriteObservation. Exclusively create destinations; report.md and delta.draft.json close before final report.json publication. An absent report.json means interrupted publication; incomplete scans may publish Complete=false.
- [x] Refuse existing files/directories, handle cancellation/write/close errors, and clean only invocation-owned outputs. No locks/report-digest/finding-ID protocol or overwrite mode.
- [x] Wire runObserve and runCompare to real inspection, validation, publication and target exit semantics. Print completeness/API coverage/finding counts. Remove every Task 1 placeholder.
- [x] Test complete differences exit 0, incomplete scan exit 1, invalid CLI exit 2, partial publication, preexisting destinations and observe refusing incomplete snapshots.

#### Verification

- [x] V-REVISIONS, V-BUILD, cmd/audit tests, changed-file make lint and V-AUDIT pass. Verify no profile/support/source mutation from either audit command.

### Task 28: Verify real version changes and no-change onboarding

**Files:** audit/profiles/routegen focused integration fixtures and tests; ignored audit artifacts.

**References:** target §§1, 8 onboarding, 9 acceptance; §11 end-to-end algorithms.

**Budget:** ≤450 authored Go/test lines. **Dependencies:** 27.

- [x] Exercise the known 1.25→1.26 comparison in separate compiler processes, expecting selected signature/API changes and crypto source review, while unchanged layouts/constants stay unchanged.
- [x] In a private copy, register a no-change next family, run profiles, then the focused buildspec/buildcfg/spec/profiles/hostabi suites; no edits to negative admission tests should be needed.
- [x] Test hypothetical unchanged next-family metadata with empty delta through support/gate/revision rendering and explicit origin bootstrap fixtures; assert zero copied implementation files and cumulative inherited variants.
- [x] Extend Task 24's private fixture under the RUNNING go1.26 ordinary job: omit 1.26 from its generated support table AND set its go126.json supported=false (retain parent 1.25), keeping fixture registry/definition state consistent. Rebuild the metadata-only tool with that same real compiler executable and GOTOOLCHAIN=local; never download another toolchain. Run compare -parent=go125 with one host architecture, no parent observation, an explicit 5-minute timeout and cleanup; require api_diff=skipped-no-parent and evidence that actual source parsing occurred. Assert ordinary admission rejects 1.26 through THIS fixture-built binary's legacy flag-only generation path reaching buildcfg.Resolve, not an in-process call in the original test binary. Do not mutate the working checkout or spoof runtime.Version. Record runtime; the go1.25 job explicitly skips this candidate-specific case and the go1.26 final job must execute it.
- [x] Test unchanged family authoring does not stale a captured parent snapshot merely through new data/support artifacts; actual source edits do invalidate it.
- [x] Verify adding/retiring support does not allow runtime switching or gaps in ancestry, and normal unregistered compiler admission stays rejected. Do not add real 1.27 support or claim execution with unavailable compiler fixtures.

#### Verification

- [x] V-REVISIONS, V-BUILD and focused onboarding tests pass; archive V-AUDIT results and state actual versus fixture-based coverage separately.

### Task 29: Migrate worker, artifact and metatesting fixtures

**Files:** internal/{workerboot,gosimtool,candidate,buildcfg,translate} fixtures/tests; metatesting/protocol_test.go; .ci metadata utilities where affected.

**References:** target §5 complete identities/caches; §7 consumer inventory; §8 ordinary cache/worker flows.

**Budget:** ≤450 authored Go/test lines; split fixture-only batches if needed. **Dependencies:** 28.

- [ ] Finish fixture substitutions for shared paths, generated registry/revisions and exact compiler/worker identities. Preserve patch/features/race manifest matching and launcher-selected worker behavior.
- [ ] Add/retain meaningful regressions for cache invalidation from ancestor/shared source, origin-only TranslatorDigest changes, evidence-only stability and corrupt/stale cache rejection with unchanged schemas.
- [ ] Preserve SQLite table/index/value formats, seven-day retention/hourly touches and content-addressed contract cache behavior. Do not introduce old-format decode or a new persistent audit cache.
- [ ] Update candidate/CLI fixtures without executing large-gated cases; record such migrated fixtures as unexecuted. Run worker-dependent ordinary tests only through prepared source-matched workflow.

#### Verification

- [ ] V-REVISIONS, V-BUILD and focused non-worker tests pass; V-ORDINARY covers applicable worker/protocol tests. Record restricted skips without promoting them to passes.

### Task 30: Document the supported workflow and measure compaction

**Files:** README.md, AGENTS.md/CLAUDE.md where present, docs/design.md, relevant existing diagrams/command guidance; this plan's accounting.

**References:** target §§1–2, 6 commands, 9 file/storage accounting, 10 exclusions.

**Budget:** ≤100 authored Go lines (normally zero); documentation only. **Dependencies:** 29.

- [ ] Update package ownership, cumulative constraints/deltas, independent origins, sole profiles maintenance entry and ordinary stable-family admission; remove retired catalog/genprofile and old capabilities-file links.
- [ ] Document recommended observe→compare→reviewed definition/code→profiles→matching compiler origin generation→checks workflow, skipped API semantics and partial-output interpretation.
- [ ] Preserve all existing race/large/certification/lint restrictions. Add comments for intentionally unsupported automatic rebase/apply/semantic-proof behavior at the relevant tool boundary only; no runtime warning or stub.
- [ ] Measure production file/byte totals across compat, hostabi, support registry, generator and audit; report test/doc and ignored report/generated bytes separately. Compare against 195 files/1,785,064 bytes in the original family trees without hiding the new subsystem costs.
- [ ] Update existing architecture documentation to implemented facts and resolve stale shared-shim retirement wording; do not create a separate proposal document.

#### Verification

- [ ] V-DOCS passes; V-BUILD remains green if any command documentation required code adjustment. All advertised command names/paths exist.

### Task 31: Run final ordinary and generation validation

**Files:** only fixes justified by failed gates; plan progress/evidence.

**References:** target §9 acceptance and limitations; §5 identities; §8 success/failure flows.

**Budget:** ≤400 authored Go/test lines for fixes; split any larger correction. **Dependencies:** 30.

- [ ] Run V-REVISIONS check, V-GENERATION for both exact baselines and V-BUILD under both families on the completed source.
- [ ] Run V-ORDINARY separately for both families with distinct report roots, read reports/timelines/logs and record exact tested source, exits, totals and skips. Run V-AUDIT if not already verified against the final inspector source; stale earlier reports are not final evidence.
- [ ] Fix every observed failure before completion; log unrelated fixes in NOT PLANNED CHANGES. Repeat only the gates needed to cover the correction and any source snapshot invalidation.
- [ ] Verify no ordinary workflow triggers certification, race/large suites, crossarch execution or Docker. Keep unavailable/excluded coverage explicit; do not obtain green status by weakening guards or expectations.

#### Verification

- [ ] All selected gates pass on the final source. Applicable ordinary tests pass; excluded suites remain explicitly unexecuted. Record report locations and checksum/source identity evidence.

### Task 32: Architectural delta verification and actual-use audit

**Files:** completed implementation and this plan's gap/evidence records.

**References:** target revision 3.1 — **ALL sections**, including full schemas and every method inventory/pseudocode.

**Budget:** ≤400 authored Go/test lines for small corrections; split larger fixes before completing. **Dependencies:** 31.

#### Verification requirement

Compare implemented code, data and real entry-point flows with the target, not just test outcomes. Any correction repeats its relevant focused/build/identity checks and final ordinary coverage when affected. Put every discrepancy in NOT IMPLEMENTED GAPS, fix it, and link the fix/evidence; `[_]` means unresolved and prevents claiming full completion. Explicit user-approved deferrals must be recorded as such. The final checkbox below is the wiring audit and must be completed last.

- [ ] Verify shared hooks/shims/native host, authored cumulative constraints, signature/interface variants, test union and direct/support guards match target §2; no full per-family source copies, forwarding packages or runtime family dispatch remain.
- [ ] Verify schema/strict decoding/natural-key operations/ancestry/deep copies/derived both-architecture contracts match §3 and all associated §7/§11 methods; account explicitly for nine added contracts.
- [ ] Verify independent on-disk origins, lossless historical envelope, absence of runtime origin/evidence embeds and provenance refresh isolation match §4.
- [ ] Verify path classifier, original/relocated nodes, positive native-island edges, unconditional native-hook/shim rejection and provider/feature boundaries match §5 with a real output-graph fixture.
- [ ] Verify Identity schema, AdapterRevision/TranslatorDigest/InspectorDigest scopes, SQLite DDL/CacheEntryV2/contract-cache schemas, TTL/locking/publication and exact compiler separation match §5; no database/ORM/cache schema change was introduced.
- [ ] Verify stable inspection, snapshot schemas/scope, actual API inventory diffs, report/draft completeness and publication, exit codes and command flags match §6; source-only inspections are not reported as execution validation.
- [ ] Verify every created/updated §7 signature/component has an implementation and every retired entry/consumer has migrated; no exported command injection or incomplete command placeholder remains.
- [ ] Verify normal startup/cache hit/miss, generation/check, bootstrap, observe/compare/adoption, cancellation/cleanup and failure paths match §8/§11, including clean-checkout generation without ignored records.
- [ ] Verify §9 validation evidence and storage accounting cover the actual final source; documentation matches implemented behavior and §10 exclusions remain intentional without successful stubs.
- [ ] Reconcile every NOT IMPLEMENTED GAPS item now: fix it to the target, document an equivalent working approach, or record an explicit user-approved deferral. Mark unresolved hard items `[_]`; do not silently remove or check them off.
- [ ] **Verify all methods/components are wired for actual use:** cmd/gosim-gen routes invoke real profiles/observe/compare/generation handlers; selected/hostabi/runtime use generated support and shared code; compose/translate/buildrun consume resolved permissions; launcher/worker/cache identities include intended inputs; origin/audit writers are reachable from real tools. Verify this through real entry-point flows and call/dependency inspection, not only isolated tests. No target component is merely defined or test-only.

## NOT IMPLEMENTED GAPS

Review findings were corrected before execution (see review thread 2 and progress log); no unresolved architectural gap is currently accepted. Unchecked tasks above are pending implementation, not completed gaps. Add discrepancies found during execution here with task/target reference, evidence, fix and verification; use `[_]` for an unresolved item.

## NOT PLANNED CHANGES

- Tasks 27–28: publish closed audit temporary files with an atomic exclusive hard link followed by temporary-name removal, instead of rename. Plain rename could overwrite a concurrently created destination. The final manifest remains last, all writes/close/cleanup failures propagate, and only invocation-owned paths are removed. Target §6/§11 updated to this equivalent no-overwrite publication. No output locks or overwrite mode added.

- Peer checkpoint 3: derive unregistered test families from the registry, preserve contiguous synthetic ancestry, restrict caller inspection to read-only lists, reject stray revision inputs and narrow the generated-file exclusion, atomically replace metadata, add independent generation checks to Makefile, strengthen workspace/audit boundary tests, and correct stale live AGENTS.md paths. Detailed findings, diffs and verification are in review thread 5 and the correction commit. Task 28 now includes the no-change next-family focused-suite acceptance check.

- Task 21: full ordinary validation exposed a stale diagnostic-name assertion in `internal/hostabi/coro_test.go` after Task 18 generated support guards. Updated only the expected marker; explicit source-selection assertions remain unchanged. Diff: `GOSIM_UNSUPPORTED_GO_FAMILY_USE_GO125_OR_GO126` → `GOSIM_UNSUPPORTED_GO_FAMILY`. Focused hostabi tests pass under both baselines; final ordinary reruns are recorded in the execution log.

- Task 10: the ordinary formatting gate exposed unsorted Task 9 guard imports in both `internal/compat/go12{5,6}/shims/{guard,guards}_test.go` and missing gofumpt formatting in `internal/compat/spec/resolve_test.go`. Applied import sorting and a blank line/expanded composite literal only (no behavioral changes). Verified repository formatting and both ordinary suites.
- Task 12 fixture constraint: simulated disks start without `/tmp`; the artifact fixture provisions it. Existing `internal/fs/ops.go:dirInode` explicitly rejects directory-relative descriptors, so recursive RemoveAll of populated directories is outside modeled filesystem support. Simulation tests check artifact I/O, remove contents with supported absolute paths, then verify automatic directory cleanup. The preserved native shim test still covers populated-directory cleanup; no filesystem behavior is expanded.
- Peer checkpoint 1: add direct native compatibility/hostabi builds to move-task validation, since host command closures omit adapters. Correct strict decoding, transitional definition/evidence hashing and rewrite admission; update stale fixtures; pull signal context wiring forward; combine identical hostabi instrumentation facades. These are verified review corrections, not new runtime behavior. See progress/review thread 3.

## NOT CONSIDERED & TODO

- [postponed] Automatic baseline rebase/removal of retained ancestors. Keep ancestry; add a maintenance-boundary TODO comment if helpful, not a runtime fallback.
- [too-complex] Whole-standard-library semantic equivalence proof. Reports state declared/API/source coverage and require review for behavioral source changes.
- [potential-improvement] Interactive draft installation, automatic implementation generation and persistent audit cache. Drafts remain non-installable data and existing destinations cannot be overwritten.
- [postponed] Actual support for a new real Go family, beta/RC/custom compilers, or extra host platforms. This plan preserves 1.25/1.26 and tests future metadata only with fixtures; prereleases remain rejected.
- [postponed] Race/large suites and certification/crossarch execution require explicit separate instructions. Their exclusion is not a code defect and must not be replaced by a successful stub or bypass.
- User selection of this target for go-plan authorizes planning from revision 3; previous brainstorm confirmation bookkeeping is not an implementation feature. Plan review is closed and final findings are incorporated; no implementation is started here.

## Progress and verification log

| Date | Task | Change / evidence | Result |
| --- | --- | --- | --- |
| 2026-10-21 | Final review corrections | Kept strict ModuleFiles wrapper with inspection-only renderer entry; bounded fixture-built admission check; current-compiler origin bootstrap; seeded generator dependencies; early collector fixture and wrapper-consumer boundary | Documentation checks only; review closed, no implementation execution |
| 2026-10-21 | Review corrections | Fixed hook/runtimebridge ordering, coordinated origin authority, invariant-test migration and one-time equivalence evidence; added a bounded private-module preparation task, lint/Makefile gates and real bootstrap test | Documentation only; 32 tasks, focused re-review requested |
| 2026-10-21 | Planning | Read target revision 3.1, current source/README/design/repository guidance; created bounded ordered tasks and peer request | No implementation/build/test/generation performed |

## Plan consistency checklist

- Every task has target references, dependencies, a bounded authored-code budget and a verification block or final verification requirement.
- New interfaces/schemas are the target's contracts; staging helpers are private and removed when their old consumers disappear.
- Top-level CLI contracts precede implementation; shared-source moves keep metadata/import/generator changes coherent; production cutover waits for data, facade and maintenance support.
- Three distinct hashes preserve intended invalidation: adapter implementation, full build source including origin, inspector implementation excluding onboarding data.
- Final task audits every subsystem and ends with actual-use wiring. Gap tracking never equates unresolved work or skipped tests with completion.
