# Compact Go versions execution log

- Plan: ../plans/20261021-compact-go-versions.md
- Target: ../brainstorms/20261021-compact-go-versions-target.md (revision 3.1)
- Branch: feat/compact-go-versions
- Started: 2026-10-21
- Review cadence: every eight implementation commits. Final ordinary tests precede architecture audit; final peer review precedes native Darwin ARM64 certification. No race/large suites authorized.

## Task 1 — Generator command dispatch

Date: 2026-10-21. Implementation commit 1 (this commit).

Implemented local FlagSet dispatch, injected legacy-generation tests, help/error exit contracts, and explicit unavailable maintenance/inspection handlers. Subcommands dispatch before compiler discovery. Legacy generation arguments and refresh/check exclusion are preserved. 240 new command/test lines plus the small main replacement are within the 300-line authored budget.

Verified:
- Native build of gosim, gosim-worker and gosim-gen passed with linkname flags.
- cmd/gosim-gen tests passed (dispatch, help, invalid arguments, legacy argument/error forwarding).
- make lint passed after wrapping one long test initializer.
- Existing genprofile maintenance passed; revisions unchanged because CLI files are outside AdapterRevision inputs.

No simulation execution or certification was performed. Placeholder handlers are scheduled for Tasks 18/27 and fail explicitly until then. Unrelated docs/translation.md is untouched.

## Task 2 — Strict family definitions

Date: 2026-10-21. Implementation commit 2 (this commit).

Added the schema types, strict single-value JSON decoder with recursive duplicate detection, required-field checks, canonical family/path validation, natural removal-key shapes, architecture/native-symbol/probe set normalization, and directory loader with filename/family matching. Production descriptor consumers are unchanged. Authored Go: 485 lines; two generated revision constants updated.

Verified native command builds and the full internal/compat/spec package with linkname flags. Refreshed both active revisions with genprofile. Tests cover nested malformed/unknown fields, null and missing supported, drafts/trailing input, bad paths/tuples/arches, origin/evidence exclusion, ordered layout preservation, large integer precision, and stable normalization. Initial tests caught reuse of the populated decoding target (which retained deleted common-scope arches); resetting before normalized decoding fixed it. The complete valid probe fixture now supplies its required source path.

No restricted suite, simulation or certification ran.

## Task 3 — Cumulative resolution and immutable ancestry

Date: 2026-10-21. Implementation commit 3 (this commit).

Implemented resolved metadata, immediate-predecessor traversal from the sole 1.25 root, retained unsupported ancestors, tuple-keyed generic record operations, expanded architecture/host-context collision checks, canonical serialization, chain digests and nested deep copies. Baseline Replace/Remove fails against the empty initial record set. Normalized operation/rewrite ordering makes input set order irrelevant to identity. Decoder path checks also reject control characters; a NUL regression covers this strengthening. Authored Go: 448 added lines including tests and decoder refinements; two generated revision constants updated.

Verified native command builds and all spec tests with linkname flags after genprofile refresh. Tests cover three-layer and empty-layer inheritance, retirement, missing/wrong/cyclic parents, map-key mismatch, duplicate/conflicting/redundant operations, common-scope splitting, host-race overlap, persistent removal/re-add, map/set-order identity and nested layout/probe isolation. A separate regression confirms removing the last architecture constant persists into the next layer.

The pure resolver does not call the supported-family registry. Effective profile/policy validation is supplied by the next materialization task; current production descriptors remain unchanged. No restricted suite or certification ran.

## Task 4 — Materialized profiles, policies and contracts

Date: 2026-10-21. Implementation commit 4 (this commit).

Added independent profile/policy/shim views, declaration context/category checks, both architecture manifests, and diagnostic Chain/DefinitionDigest fields. Factored profile structure validation from strict registry admission: pure future/retired metadata can materialize, while Profile.Validate and selection still reject unregistered support. Legacy descriptors remain active and keep their existing identity shapes. ContractDigest still hashes resolved hooks, policies and both manifests, without adding diagnostic fields to its schema. Authored Go: 290 lines added/changed including the fixture tests (under 500).

Verified command builds and the complete spec package separately with Go 1.25.14 and Go 1.26.9, all using linkname flags; both passed. Refreshed both active revisions. Tests include all nine intentionally added tls/metrics/debug shim contracts, host/callable/shim separation, architecture-specific signatures, mutable-state isolation, invalid category/context/architecture rejection, pure future materialization versus production rejection, and hashing the other architecture's contracts. This confirms derived coverage; real migration equivalence/source verification remains scheduled in Tasks 15/31.

No restricted suite or certification ran. Four implementation commits now count toward the first eight-commit Claude review checkpoint.

## Task 5 — Shared path admission and transitional fingerprints

Date: 2026-10-21. Implementation commit 5 (this commit).

Added category recognition for original and relocated shared paths, with temporary recognition of the two existing family roots. Source/profile paths still reject inactive families, relocated targets and metadata/tooling escapes. Native import permissions stay exact, and compatibility implementations cannot be exempted from translation. Composition derives testing/kernel aliases from the selected runtime package inventory; missing/duplicate roles fail. Active revision inputs now include shared directories as they appear and exclude generated revision outputs. Authored Go approximately 330 lines including tests; no executable files moved yet.

Verified genprofile, native command build, complete spec tests, focused translate policy/boundary tests, and compose generation/publication/tamper/role tests. All passed with linkname flags. These focused checks do not execute a translated simulation; the real compiled graph gate remains Task 21.

Baseline migration evidence: `git ls-tree -rl 550d5e3 -- internal/compat/go125` gives 96 files / 877411 bytes; the corresponding go126 command gives 99 / 907653. Counts/sizes are computed from tracked blobs (sum field 4), independent of ignored local state. Task 15 reads that base commit for its one-time metadata comparison; no ordinary test depends on Git history or ignored captures.

No restricted suite or certification ran. Five implementation commits toward the first peer checkpoint.

## Task 6 — Active shim source bindings and inventory checks

Date: 2026-10-21. Implementation commit 6 (this commit).

Changed VerifyShimBindings to the resolved-family/source-context interface. It selects files with explicit target release tags and approved translated/native predicates, ignores tests/inactive filenames, excludes C imports when cgo is off, and retains duplicate declaration/signature checks. Temporary legacy shim target admission supports old descriptors until cutover without a second public verifier. Updated callers atomically.

Replaced per-file bounded-family assertions with nonempty selected package inventories for each family/architecture/source mode, duplicate detection and separate unsupported guard selection checks. The immutable declaration ledger payload/digests are unchanged; its path lookup falls back to shared directories as packages move. Runtime tests now check target categories plus validated metadata instead of demanding physical family ownership. The inventory includes constants/variables so the constant-only maps shim is represented. Authored edits: 345 added/removed lines including new tests, below the 450-line budget.

Verified both native command builds and full spec suites under Go 1.25.14 / Go 1.26.9 after genprofile refresh. Tests prove an inactive file cannot satisfy a missing binding or trigger a duplicate, selected duplicates fail, methods and architecture filenames work, Go 1.26 variants inherit into a hypothetical 1.27, and release tags do not leak from build.Default. Initial inventory failures exposed relative filepath.Rel inputs and the constant-only maps package; both were corrected.

Race/cgo and alternate-architecture cases above are source inspection only. No race build/execution, restricted suite or certification ran. Six implementation commits toward the first eight-commit peer checkpoint.

## Task 7 — Rendering, destination selection and publication

Date: 2026-10-21. Implementation commit 7 (this commit).

Separated validation/rendered output maps from publication. CheckProfile uses the same map without staging writes and skips ignored generation records. Current records now live under .gosim/generation/<profile>/; committed legacy routes JSON remains frozen solely for old embeds until Task 18. Exact source provenance remains independent. The temporary prepareProfileOutputs entry still reads the old catalog; Tasks 18/19 replace this authority with the planned resolved-family render entry.

Generation/source loading now validates the running compiler and installation before parsing. Adapter targets/packages come from reviewed profile records. Destination selection matches existing authored constraints against explicit family/architecture tags, rejects missing/ambiguous destinations and preserves constraints verbatim. Input source root selection is independent of the publication root. Twelve generated syscall files changed only to version-neutral headers; layout bodies and frozen route records did not change. Tests no longer strip headers or erase record digests to compare outputs. Authored Go edits: 446 added/removed lines including new tests; generated headers/revisions accounted separately.

Verified regeneration, full routegen tests, independent -check generation, and native command builds under Go 1.25.14 and Go 1.26.9, all passing. Refreshed active revisions. Tests cover empty output trees, missing/ambiguous/inherited destinations, wrong compiler rejection before writes, other-origin and architecture isolation, complete-byte equality, stable mtimes, stale-file rejection without repair, and checks after deleting the ignored generation-record directory.

No runtime behavior changed, restricted suite or certification ran. Seven implementation commits; the first Claude review follows Task 8's commit.

## Task 8 — Shared native host ABI

Date: 2026-10-21. Implementation commit 8 (this commit).

Verified normalized equality of coro/race/norace bodies in both host trees and corresponding facade pairs, then retained one copy under compat/host and shared hostabi facade filenames. Preserved unsafe/linkname/race behavior, added a direct host unsupported guard, and removed union ceilings from the six specified shared facade/test files. Updated both profiles' native contract targets and only their origin Hook.Target annotations. Six old implementation/facade copies were removed; three files moved in each area plus one direct guard. Authored edits remain below 100 lines aside from mechanical moves, constraints, target rewrites and generated revision constants.

One-time provenance assertions compared every parsed JSON field, allowing only the exact host Target prefix rewrite. A second independent canonical hash with Hook.Target blanked remained identical before/after: go125 = 950188f58d59949d8c96a991402b276448a176d18fca85c87d92d0caac76d402; go126 = ff24c71b8662b6bb36c95cc67d466aa75c5d53b8744bb1943a415f32be919b1e. Thus upstream paths, signatures, content digests, schema, family and exact baseline stayed unchanged. The temporary helper is ignored maintainer evidence, never an ordinary test input.

Verified genprofile refresh, native command builds, hostabi coroutine/upcall/identity tests, complete spec tests, and independent generation -check under both Go 1.25.14 and Go 1.26.9. All passed. Runtime dependency inspection now requires shared compat/host and rejects old host paths. Guard tests inspect explicit family source contexts; removed the old manual release-tag negative build. Native host/selected/hostabi guards cover unsupported families. Inspected maps/synctest leaves: neither imports native metadata; no new guard dependency was added there.

Race and hypothetical compiler checks were source inspection only. No race/large suite or certification ran. Eight implementation commits completed; Claude review requested next, before Task 9.

## Task 9 — Shared runtime hooks and runtimebridge

Date: 2026-10-21. Implementation commit 9 (this commit).

Moved hooks/rt and its runtimebridge dependency together after normalized body comparisons. Preserved the full test union; the additional timer-prefix test now runs for both families. Extracted unchanged StoreReluintptr bodies into bounded 1.25 and cumulative 1.26 files, with typed result/no-result tests. Shared runtimebridge linkname tests, root guard imports, profile runtime packages, hooks/contracts/bridges and exact native permissions all follow the moved packages. Common time comments now describe the shared behavior. No simulator scheduling/atomic algorithm changed.

Updated physical-family assertions in spec and translate tests to validated hook categories, and extended the native-import audit to shared selected sources (the old glob would silently omit moved files). A source-only regression proves 1.27 inherits the 1.26 store signature without enabling support. Authored changes are below 200 lines beyond verified mechanical moves/metadata rewrites; the 400-line task limit is retained.

Verified both compiler builds, shared rt/runtimebridge native suites, both corresponding root shim guard suites, complete spec tests, and focused translate policy/boundary tests, all with linkname flags. Refreshed both revisions. Fixed an accidentally overbroad test edit and a remaining architecture-test ownership assertion found by the spec suite; final runs pass. No sim-only test was counted as native coverage. Full source-matched ordinary execution is required in Task 10 for the coupled hook/composition transition.

Claude's first checkpoint review is pending against immutable commits through 0abb62a while this next task proceeds. No restricted suite or certification ran.

## Task 10 — Shared kernel, filesystem and network hooks

Date: 2026-10-21. Implementation commit 10 (this commit).

Moved the three coupled packages together, including architecture-selected generated syscall/layout files. Normalized comparisons confirmed identical production bodies except the extracted Go 1.26 clearenv addition and version-neutral transport diagnostic. Shared native tests use the selected host identity and retain the union of statfs/GID assertions. The Go 1.26 clearenv map/host isolation test remains bounded by its API introduction; explicit source inspection proves inheritance into hypothetical 1.27. Both profiles, exact targets, runtime package composition and generator fixture paths now point to shared hooks. Authored changes are below 200 lines beyond mechanical moves, constraints, metadata replacements and generated revisions.

Verified both baseline command builds, direct native compatibility/hostabi builds, focused kernel/spec/routegen suites, revision refresh and independent generation -check. Repository formatting initially failed on Task 9 import ordering and a resolver-test gofumpt issue; fixed those formatting-only issues, then reran cleanly. Linux-only statfs/GID execution is not claimed on this Darwin host; the assertions and architecture selection are preserved.

Both source-matched ordinary runs passed, exit 0, with input digest ca458701661912eaf7f5f2ebe2c0c7f55fc73a43202cf9571e3601cff7624e6a and no worktree changes:
- Go 1.25.14: 82 reports, 1,949 tests, 1,933 passed, 0 failed, 0 errors, 16 skipped; 270.580 s.
- Go 1.26.9: 82 reports, 1,953 tests, 1,937 passed, 0 failed, 0 errors, 16 skipped; 273.008 s.
Reports/timelines: .gosim/compact-go-validation/{go125,go126}/unit/latest/. Skips include large/race gates; no restricted suite or certification ran.

Claude's first implementation checkpoint found no P1/P2 and six P3/simplification points. The direct adapter build gap is now closed and recorded in V-BUILD. Remaining small verified findings will receive a separate corrective commit before Task 11. Two implementation commits since the first eight-commit checkpoint.

## First peer checkpoint — corrections

Date: 2026-10-21. Implementation commit 11 (this commit).

Closed review thread 3 after verifying and fixing all six P3/simplification points: reject explicit empty arches; include all shared definitions in both transitional revisions and exclude exactly relocated evidence; reject native host/metadata rewrites at manifest validation; signal-cancel generator command context; update stale revision fixture paths; merge identical hostabi forwarding files into instrumentation.go with unchanged host implementation selection. Mutation/admission regressions cover the concrete bugs. CLI cleanup handlers still arrive with Task 27. The optional facade merge removes one file; no race instrumentation or execution was used.

Verified spec/hostabi/gosim-gen focused suites and command/compatibility/hostabi builds under both baseline compilers, refreshed revisions, passed cmd-scoped make lint and full repository formatting. Task 10's ordinary reports predate these small review fixes; no new full-suite coverage is claimed here. Review disposition and evidence limitations are in the local peer thread. Three implementation commits since the first eight-commit checkpoint; next scheduled checkpoint after five more.

## Task 11 — Shared reflect and inherited interface surface

Date: 2026-10-21. Implementation commit 12 (this commit).

Compared every common reflect file after constraint/diagnostic normalization: bodies match. Moved one shared implementation and all boundary assertions, with cumulative Go 1.26 Type/Value iterators. Type embeds a small typeExtension: empty for Go 1.25, four iterator methods from Go 1.26 onward. Both profiles' bridges, shim targets and exact native permissions now reference shared reflect; guard imports moved atomically. Native boundary rejection semantics are unchanged, with family-neutral diagnostic labels.

The embedded-interface strategy is verified, not assumed. A translated Go 1.26 fixture boxes/unboxes an embedded reflect.Type interface, invokes all iterator surfaces, checks type identity and reflected simulated-map fields, early-stop/mutation, bound/unbound method calls and input/output types. The normal launcher builds a matching worker and translated test binary; TestReflectInheritedIterators passed with features rt, seed 1. All seven pre-existing TestReflect cases pass under both baselines; Go 1.26 runs the eighth new case. The Go 1.25 file selection excludes the new API fixture.

Verified both native command/compatibility/hostabi builds; complete shared reflect native tests (including iterator and boundary tests), spec and ordinary translate suites; repository formatting. Updated the pinned public-interface source comparison to follow selected local embedded interfaces and compare all exported signatures against the actual compiler; a native method-name check also confirms exact method surfaces. Fixed two obsolete physical-family assertions and an unused fixture variable exposed during verification. No translator semantic changes, restricted suite or certification. Authored edits remain below 350 lines beyond mechanical moves/metadata/revisions (450 budget). Four implementation commits since the first checkpoint.

## Task 12 — Shared testing state and small API variants

Date: 2026-10-21. Implementation commit 13 (this commit).

Shared the testing body, lifecycle helpers and union of tests. The common struct retains both Gosim artifact fields in all families. Small cumulative version files select an empty/ArtifactDir TB extension and temporary-root helper; Go 1.26's ArtifactDir body is unchanged. Both metadata sets/composition targets point to the shared package. Normalized common-source comparisons passed; diagnostics/comments are family-neutral. The interface strategy proven in Task 11 also passes pinned TB signature checks and translated interface calls here.

Verified revisions, both baseline command/compatibility/hostabi builds, shared testing/spec/compose suites (including disabled-feature abort coverage), formatting, focused native filesystem fixtures, and prepared launcher simulations. Go 1.25 ignores GOTMPDIR and cleans its child directory; Go 1.26 honors the root, reuses each test's artifact directory, separates siblings, supports read/write and cancels context before cleanup. Existing runtime cleanup/nested-name tests pass in rt-only simulations under both compilers.

Initial fixture failures exposed existing boundaries rather than a migration regression: simulated disks do not pre-create /tmp; native Go adds intermediate directory names; recursive RemoveAll uses directory-relative descriptors explicitly unsupported by fs.dirInode. Fixtures now provision the parent, assert containment, and remove written files via supported absolute paths before asserting automatic directory cleanup. The preserved native shim test still exercises populated-directory cleanup. This does not claim recursive simulation deletion support, and no host fallback or filesystem implementation change was added. Authored edits are below 200 lines plus verified mechanical moves/metadata/revisions. No restricted suite or certification. Five implementation commits since checkpoint 1.

## Task 13 — Shared deterministic crypto/rand

Date: 2026-10-21. Implementation commit 14 (this commit).

Compared common rand/text/prime/Int sources; the algorithm is unchanged. Shared Prime calls a small version-selected reader helper after the existing invalid-bit check. Go 1.25 returns its caller reader; Go 1.26 preserves last-setting-wins cryptocustomrand parsing from translated machine environment and otherwise returns the internal deterministic reader. No native entropy fallback. Both profile replacements/bindings/contracts now reference shared cryptorand. Preserved the full native test union and added a bounded Go 1.25 default-reader assertion.

Verified revision refresh, both baseline command/compatibility/hostabi builds, shared cryptorand/spec suites and repository formatting. Prepared rt-only simulations passed custom-reader and crypto Read tests for seeds 1 and 2 under both compilers; Go 1.26 additionally passed the internal-default regression with failing public and supplied readers. Existing replay tests verify same-seed determinism; cross-family Read log bytes also matched for both seeds. Authored changes below 150 lines plus mechanical moves/metadata/revisions. No algorithm/linter-driven edits, restricted suite or certification. Six implementation commits since checkpoint 1.

## Task 14 — Complete executable shim consolidation

Date: 2026-10-21. Implementation commit 15 (this commit).

Verified normalized equality and moved debug, exec, httppprof, maps, metrics, ocsp, pprof, signal, syncmap, synctest, tls, user and x509 one package at a time. Each batch updated both active metadata sets/root guard imports and passed its native package checks under both baselines before proceeding. Consolidated identical root guard tests and preserved all synctest assertions. No executable hooks/shims/host files remain in either old family tree; profile metadata remains active until the planned cutover. Most changes are mechanical; fewer than 30 authored lines change test path expectations/normalization and neutral diagnostics.

Verified revisions, both baseline command/compatibility/hostabi builds, full shared shim/spec suites, repository formatting and independent generation -check. The first full ordinary runs found three stale translator golden paths for testing/syncmap. Replaced SELECTED paths with literal shared paths and removed the obsolete family-path normalizer, then passed focused TestTranslate under both compilers and repeated both complete ordinary runs.

Final reports/timelines in .gosim/compact-go-validation/{go125,go126}/unit/latest/:
- Go 1.25.14: 82 reports, 1,964 tests, 1,948 passed, 0 failed, 0 errors, 16 skipped; exit 0, 212.430 s.
- Go 1.26.9: 82 reports, 1,978 tests, 1,962 passed, 0 failed, 0 errors, 16 skipped; exit 0, 213.576 s.
Both tested input digest 8540538ec79ea23c6bc3e5a5e20395e1891941a9fe770b40889931a52bbe8be6 with no checkout changes. Native and simulation checks cover the fully shared executable tree; large/race gates remain skipped and no certification ran.

Read-only preparation for metadata conversion confirmed all current profile JSON except intentionally frozen generation records equals base commit 550d5e3 after compatibility target-path normalization. Both architectures still omit exactly the same nine known shim contracts; Task 15 accounts for that intended coverage addition. Seven implementation commits since checkpoint 1; Task 15 will complete the next eight-commit review range.

## Task 15 — Baseline and cumulative metadata definitions

Date: 2026-10-21. Implementation commit 16 (this commit).

Generated go125.json as the full baseline and go126.json as only its delta: 367,659 and 8,894 bytes respectively. Reviewed every delta operation. One-time ignored conversion/comparison helpers read base commit 550d5e3 and compared all derived metadata against current shared paths. Both commands exit 0; exact commands, normalization rules and differences are recorded under Task 15 in the plan. Independent origin/evidence payloads remain intact. The only added contract coverage is the nine previously omitted shim bindings specified by the target; ABI interface{}/any spelling differences remain explicit replacements.

Added 133 lines of self-contained spec tests: frozen canonical semantic digests for both family/architecture views, empty 1.27 inheritance through real 1.26 changes without production registration, and actual running-compiler contract checks for both architecture/source race contexts. These are source checks, not race or cross-architecture execution. No committed test reads .git, old profile JSON or ignored migration output. Generated JSON is separate from the authored Go budget; one-time helpers are ignored maintainer evidence, not a permanent generator.

Verified complete spec suites under Go 1.25.14 (3.656 s) and Go 1.26.9 (3.744 s), command/compatibility/hostabi builds under both, revision refresh, repeated semantic comparison and repository formatting; all exit 0. Fixed one test-file formatting issue. Task 14's full ordinary runs cover the shared executable tree before this metadata addition; no new full ordinary coverage is claimed for Task 15. Existing callers still use old profile metadata until the planned facade/cutover. No restricted suite or certification ran.

Eight implementation commits since checkpoint 1, including the review correction; checkpoint 2 is requested next against 0abb62a..HEAD. Task 16 remains pending.

## Task 16 — Profiles facade and independent provenance

Date: 2026-10-21. Implementation commit 17 (this commit).

Added the lazy definitions facade with independent returned records, supported descriptor/registry checks and the specified accessors. Its embed pattern contains only definitions. Revision constants are explicitly unused transition seeds; final hashing/revisions arrive in Task 17 before production selection changes. Added filesystem LoadOrigin with canonical profile/family, stable baseline, inventory path and digest validation. Copied both origins byte-for-byte, removed their legacy embeds, and changed all legacy callbacks/callers to explicit source-root reads. Route generation discovers the module root before loading provenance and writes refreshes only to profiles/goNNN.origin.json. Legacy copies are frozen and unread pending deletion.

Consolidated both complete evidence/tuple payloads into schema-1 evidence.json. One-time parsed comparisons prove lossless payloads and copied origin bytes; a durable frozen history digest protects the envelope without a retired-tree dependency. Go and Python whole-source identities exclude exactly the relocated history while retaining origins; existing legacy exclusions remain until cutover. Tests cover cloning, definition-only embeds, filename/family validation, independent refresh and exact exclusion scope. About 380 authored Go lines, plus lossless JSON copies/envelope and 35 Python test lines; below the task budget.

Verified both baseline command/compatibility/hostabi builds, complete profiles/spec/buildcfg/routegen suites, genprofile refresh, Python source utility test and repository formatting. The initial routegen test failed because its protected-other-family detector still assumed a directory name; changed it to the exact relocated origin path and reran the full routegen suite under both compilers (5.439/5.629 s). An additional explicit RefreshOrigin isolation regression passes under both. All final commands exit 0. No origin refresh in the working checkout, restricted suite, cross-architecture execution or certification occurred. One implementation commit since checkpoint 2; Claude's review of the previous immutable range is pending.

## Task 17 — Final source revisions and support rendering

Date: 2026-10-21. Implementation commit 18 (this commit).

Implemented the length-delimited gosim-compat-source-v2 hash over target family, canonical ancestry/resolved metadata and sorted conservative compat/hostabi/buildspec inputs. Removed all family-filename filtering; source/header/metadata/path changes invalidate, symlinks fail. Origins, exact history, tests/testdata, generated revisions and ignored artifacts are excluded; temporary old-origin/evidence exclusions remain until Task 18. Whole-source Go/Python identities stay origin-sensitive as verified in Task 16.

Implemented on-disk GenerateMetadata with validated ancestry, exact supported intervals/gaps, generated registry/selectors/three guards, support-before-revision ordering, structural origin readiness checks and context cancellation. Obsolete generated selectors are removed; authored files are protected. Unchanged bytes preserve mtimes. Fixture repositories prove a supported 1.27 absent from the compiled registry, retired intermediate ancestry, a support gap, origin-free write bootstrap, check rejection without provenance, deterministic repair, retirement cleanup and inherited go126 source invalidation. Real support selectors remain unchanged until Task 18. Refreshed the new revision table independently through ignored migration/revisions.go and refreshed active old descriptors with genprofile; a durable test checks the new table against SourceRevision. About 480 authored Go/test lines, below 500.

Both baseline command/compatibility/hostabi builds and profiles/spec/buildcfg suites pass; formatting and diff checks pass, exit 0. No restricted suite or certification ran. Two implementation commits since checkpoint 2.

Handled Claude thread 4: no P1/P2, independent metadata/move comparison confirms equivalence. Accepted the filename regression and scheduled removal of the opaque migration hash test at cutover. Strengthened semantic comparison into an unconditional Task 18 pre-deletion gate; added --check to the ignored converter so that validation cannot overwrite a drifting definition. Retained readable JSON formatting. Review thread closed with limitations and each disposition recorded; Task 18 obligations remain unchecked.

## Task 18 — Production cutover and duplicate metadata removal

Date: 2026-10-21. Implementation commit 19 (this commit).

Passed the mandatory pre-deletion comparison twice, including immediately before removal: python3 -B .gosim/compact-go-migration/convert.py --check and go run -ldflags=-checklinkname=0 -tags=linkname ./.gosim/compact-go-migration/verify.go. Both exit 0 and preserve definitions. All four semantic digests match Task 15; the exact nine added contracts and normalization remain the only differences. Removed the migration-only opaque oracle afterwards, retaining real-compiler/inheritance checks and adding explicit assertions for the nine previously omitted contracts.

Seeded support outputs with the already-built metadata generator in the same edit that removed the handwritten table. Shared selected methods now call profiles with explicit errors, family files contain constants only, FamilySupport contains only Family/Profile, and gosim-gen profiles is wired to the real maintenance flow. Removed old family profile trees, catalog and genprofile after migrating all source/fixture callers. Shim/declaration inventory tests require the shared roots and preserve historical ledger meaning. Origins/history remain independent. Removed old identity exclusions and tooling classifier entries; updated Go/Python utility tests. Makefile generation runs maintenance before/after family generators and retains model stringer generation/linkname flags. No callback or forwarding package remains.

Verified both baseline command/compatibility/hostabi builds, profiles/spec/buildcfg/buildspec/selected/routegen/gosim-gen tests, both independent exact-compiler generation -check commands, new profiles write/check, generator dependency closure, cmd-scoped lint, repository formatting and Python identity utility checks. All final gates pass. The generator closure has no executable adapters/runtime. Fixed a mechanical test-import edit that broke one expression, CLI-test lint findings, and final import/gofumpt issues; refreshed revisions after production formatting. No restricted suite or certification. About 250 authored Go/test lines plus call-site substitutions/generated outputs and deletion of duplicate metadata; below 500. Three implementation commits since checkpoint 2; first-origin bootstrap remains Task 19.

## Task 19 — Shared generation and first-origin bootstrap

Date: 2026-10-21. Implementation commit 20 (this commit).

Generation now resolves current checkout definitions, verifies agreement with the compiled support registry and uses the same SourceRevision as maintenance. Final renderProfileOutputs consumes ResolvedFamily, validates the exact running compiler, modules, actual contracts and selected shim bindings, and renders selected authored shared destinations. Missing provenance is seeded only under explicit RefreshOrigin; malformed existing metadata still fails. Publication stays scoped to shared selected outputs, this family's independent origin and ignored records.

Added a bounded five-minute private-copy end-to-end fixture using only ResolveRunningProcess's compiler with GOTOOLCHAIN=local. It builds the real metadata-only generator, removes only the current origin, proves maintenance write bootstrap and readiness/normal-generation rejection, checks malformed-origin rejection and live disk-definition admission (despite a stale embedded definition), refreshes successfully, checks the exact running baseline and unchanged other-family origin, verifies stable AdapterRevision, and runs final check after deleting ignored records. No process cwd/environment mutation or new-family compiler download. The copy/binary are temporary and cleaned by testing.

Complete routegen suites pass with Go 1.25.14 (8.734 s) and Go 1.26.9 (8.801 s), including the real bootstrap fixture. Both command/compatibility/hostabi builds and both independent generator -check commands pass; profiles write/check, formatting and diff checks pass. Shared outputs were therefore accepted independently for both current selecting families. About 280 authored Go/test lines; below 500. No restricted suite/certification. Four implementation commits since checkpoint 2.

## Task 20 — Retire migration admission and consumer assumptions

Date: 2026-10-21. Implementation commit 21 (this commit).

Removed old-family classifier prefixes, singular profile metadata classification, physical-family target/import filters and legacy shim-binding exemptions. Shared categories and exact native permissions now exclusively control admission. Updated positive fixtures to shared paths and added/retained negative fixtures for retired family targets, metadata/tooling and composition roles. Replaced the temporary old-descriptor shim conversion with actual resolved-family binding tests, preserving upstream and signature-mutation checks. Native import audits require shared inventories only; historical ledger mapping remains explicitly historical.

Searched source/scripts/fixtures and reviewed remaining matches: old executable paths occur only in rejection tests/historical ledger mappings; scripts use the new profiles origin/definition paths. Generated supported selectors/guards are the only go1.27 ceilings. No forwarding package, duplicate expanded metadata, old source-revision branch or retired maintenance entry remains. Updated two maintenance/shared-shim comments; no behavior algorithm changed. Under 150 authored Go/test lines, below 400.

Both baseline spec/translate/compose suites pass (Go 1.25.14: 7.507/18.968/8.750 s; Go 1.26.9: 7.442/19.904/9.171 s). Both command/compatibility/hostabi builds, profiles write/check, cmd-scoped lint, repository formatting and diff checks pass. Large-gated translator cases retain their normal skips; no race/large/certification or cross-architecture execution ran. Five implementation commits since checkpoint 2. Real compiled-graph enforcement and its ordinary translated fixture follow in Task 21.

## Task 21 — Compiled graph native-island boundary

Date: 2026-10-21. Implementation commit 22 (this commit).

Classified native/relocated compatibility nodes and admitted native host/metadata only within the actual dependency closure of rt/runtime and rt/runtime/abi. Relocated Gosim imports may cross only those two islands; native hooks/shims, relocated metadata/tooling, retired paths and direct hostabi/buildspec imports fail. Existing provider isolation, feature exclusion and required-provider checks remain. Synthetic tests cover nodes, edges and test-variant spelling; the new ordinary translated fixture exercises shared reflect, runtime scheduling and channels through real generated output. About 180 authored Go/test lines, below 450.

Both baseline builds, focused buildrun/hostabi suites, profiles -check, formatting and diff checks pass. Initial ordinary runs exposed only a stale unsupported-guard diagnostic assertion from cutover; corrected its expected marker without changing source-selection assertions (recorded under NOT PLANNED CHANGES), then reran both complete ordinary suites.

Final reports/timelines in .gosim/compact-go-validation/{go125,go126}/unit/latest/: Go 1.25.14: 79 reports, 1,981 tests, 1,965 passed, 0 failures/errors, 16 skipped, exit 0, 247.304 s. Go 1.26.9: 79 reports, 1,995 tests, 1,979 passed, 0 failures/errors, 16 skipped, exit 0, 258.984 s. Both verification records report passed, input digest 56e6056cf39fb948f285ed7b659e2e1e6bb4c9988a22332d397e919c452b3aed and no checkout changes. TestCompiledCompatibilityGraph passed in both simulation logs. Race/large gates stayed skipped; no certification or cross-architecture execution. Six implementation commits since checkpoint 2.

## Task 22 — Narrow stable compiler inspection

Date: 2026-10-21. Implementation commit 23 (this commit).

Added SourceInspectionCommand with a finite env/version/list argument allowlist, canonical stable versions, exact GOROOT/bin/go, local toolchain/environment controls and no build/run/test permission. Normal OutputCommand/ApplicationCommand retain supported-family validation. Shared private discovery decoding and worker identity checks preserve ordinary behavior. DiscoverInspection selects the running installation, treats an explicit version as an equality assertion, verifies executable/runtime/root identity, fingerprints installed tools and sources and returns private state with value accessors and Recheck. Fingerprint walking now observes cancellation without changing normal fingerprint identities.

Tests cover required query forms, unsupported stable command admission only on inspection, arbitrary flags/commands/environment and prerelease rejection, fake future runtime mismatch, fake current GOROOT mismatch, executable/env identity mismatch, ordinary unsupported discovery/fingerprint rejection, real discovery/recheck, changed stored fingerprint and cancellation. Existing distribution mutation tests cover changed installed bytes; no real compiler installation or support registry was altered. Full module preparation and real unregistered collector fixtures remain Tasks 23/24. About 330 authored Go/test lines, below 500.

Both baseline buildspec/buildcfg suites passed (Go 1.25.14: 2.788/3.998 s; Go 1.26.9: 2.935/4.139 s). Both native command builds, profiles write/check, repository formatting and diff checks pass. No ordinary simulation behavior changed or restricted suite/certification ran. Seven implementation commits since checkpoint 2; Task 23 completes the next review range.

## Task 23 — Inspection-only private module preparation

Date: 2026-10-21. Implementation commit 24 (this commit).

Added fixed inspection application/module-graph/module-file wrappers with private shared implementations; ordinary wrappers retain supported-family validation. The renderer validates canonical stable versions and preserves selected replacements, sums, go directive and pruned graph semantics. PrepareSourceInspectionContext uses only inspection wrappers for caller discovery, rendering and private graph verification, with GOWORK=off in the private context. When only a directory is supplied, it discovers caller GOMOD/GOWORK through the approved env query, preserving ambient workspace selection without exposing mutable InspectionCompiler state. Ordinary hook preparation still uses strict wrappers. No public injectable command factory.

A controlled future-stable fixture covers both module and workspace contexts, exact command/environment capture, local replacements, sums, small source parsing, ordinary rejection, private graph failure cleanup and returned cleanup errors. It does not claim runtime equality for its fake compiler; Task 22 rejects that mismatch. A real current-compiler discovery→preparation→source-list→recheck fixture preserves a caller workspace replacement and all input bytes. An AST boundary test restricts production inspection-wrapper references and fails if its inventory is empty. Full real registry-omitted collector/CLI fixtures remain Tasks 24/28. About 390 authored Go/test lines, below 500.

Initial new fixtures failed because the shell canonicalized macOS temporary paths and the boundary walk skipped its own relative root. Corrected the fixtures; both final complete buildspec/buildcfg/spec suites pass (Go 1.25.14: 2.187/6.339/5.137 s; Go 1.26.9: 2.194/6.457/5.387 s). Both native command builds, profiles write/check, repository formatting and diff checks pass. No restricted suite/certification ran. Eight implementation commits since checkpoint 2; request checkpoint 3 on 6da4f67..HEAD before proceeding with Task 24.

## Peer checkpoint 3 — Review corrections

Date: 2026-10-21. Implementation commit 25 (this commit).

Verified Claude's complete thread 5 round 1: no P1/P2, five P3s. Fixed registry-dependent negative tests/guard expectations with a shared test-only helper and contiguous synthetic ancestry; restricted caller inspection to read-only module queries; rejected hidden/backup revision inputs diagnostically and narrowed the revision-file exclusion; replaced metadata atomically with cancellation cleanup; and added both independent family -check passes to make go-generate. Added auto-discovered workspace coverage and indirect audit-import enforcement. Corrected two stale AGENTS.md paths; broader docs remain Task 30. No ordinary admission relaxation or real future support. About 240 authored Go/test lines plus substitutions/generated outputs.

Both baseline buildspec/buildcfg/spec/profiles/hostabi suites passed (1.25.14: 2.284/7.372/5.256/0.735/0.361 s; 1.26.9: 2.474/7.332/5.233/0.738/0.372 s), as did both command builds. The full updated make go-generate passed with both independent checks, metadata checks and stringer generation. Formatting was corrected for new imports, then passed. A temporary copied checkout registered empty-delta 1.27 and regenerated metadata; its first focused run exposed one remaining synthetic ancestry assumption, which was fixed. All five focused packages then passed in the rebuilt private copy (9.018 s total); no source/support edits remained in the working checkout. Task 28 carries the durable onboarding acceptance requirement. No race/large/certification/cross-architecture execution.

Review thread 5 closed with each disposition and limitations recorded. This is the first commit since checkpoint 3; Task 24 is next.

## Task 24.1 — Bounded source observation collector

Date: 2026-10-21. Implementation commit 26 (this commit).

Split Task 24 before execution into collector/focused coverage (24.1) and real registry-omitted compiler harness (24.2), each within 500 authored Go/test lines. Added exact spec observation/context/compiler/location schemas and ObservationRequest. CollectSourceObservations independently validates canonical stable/running compiler, host/root/executable/VERSION identity, build settings and controlled environment. Private source-loader command selection keeps ordinary contracts on strict OutputCommand and observation on fixed SourceInspectionCommand. The collector consumes the prepared private module, observes context-selected declarations, layouts/constants, package inventory and tracked/selected source hashes, sorts deterministic records and discards incomplete context records rather than fabricating absence. API requests fail explicitly until the scheduled Task 25 inventory implementation.

Real current-compiler discovery→private preparation→listing→parsing tests cover signature/layout/constant change and absence, source hash drift, deterministic output, parse/load failure, cancellation, compiler mismatch, standard-library inventory and unchanged caller module. Source-only native race contexts select cgo files; translated race contexts exclude them. No race build/test was performed. About 483 authored Go/test lines including loader/boundary changes, below 500.

Both complete spec/buildcfg suites pass (Go 1.25.14: 7.051/7.498 s; Go 1.26.9: 7.128/7.551 s). Both native command builds, profiles write/check, repository formatting and diff checks pass. No restricted suite/certification. Real unregistered-family harness remains the immediately following Task 24.2; no claim of that coverage yet. Two implementation commits since checkpoint 3.

## Task 24.2 — Real unregistered compiler observation harness

Date: 2026-10-21. Implementation commit 27 (this commit).

Added a bounded five-minute private-copy fixture under the running Go 1.26 compiler. It marks go126 unsupported, regenerates the actual support registry, then builds a metadata-only harness with that exact compiler and GOTOOLCHAIN=local. Inside that same omitted-registry binary, ValidateVersion, ordinary Discover and OutputCommand reject 1.26; DiscoverInspection, private module preparation and CollectSourceObservations succeed against the resolved 1.25 parent. One translated non-race host-architecture context, no API snapshot or alternate compiler download. Compiler recheck and caller go.mod/go.sum preservation are required. Child temporary modules are contained in test cleanup even on timeout. Reused the bootstrap source copier through testutil instead of duplicating it.

The actual go1.26.9 harness passed in 2.999 s (3.03 s test): 476 present declarations, 24 layouts, 72 constants, 44 packages and 623 source files. The go1.25.14 fixture explicitly skipped as candidate-specific. Both existing first-origin bootstrap fixtures still pass after the copier move (5.688/5.739 s). Both complete buildcfg/spec suites and native command builds passed; profiles -check, repository formatting and diff checks pass. Full ordinary runs at final validation will execute this fixture naturally in the Go 1.26 job. About 170 new authored Go/test lines plus mechanical movement of the existing 46-line copier, below 500. No restricted suite/certification. Three implementation commits since checkpoint 3.

## Task 25.1 — Finite upstream API inventories

Date: 2026-10-21. Implementation commit 28 (this commit).

Split Task 25 before execution into bounded API, identity and snapshot tasks. API scope derives from parent declarations and reviewed bridges, including whole-package replacements with no declaration rows; Gosim implementation paths and unrelated policy wildcards are excluded. Translated contexts inspect the full upstream union, while native contexts inspect their relevant native contract packages. APIPackages records the common requested union. Collected exported declarations, constants and value/pointer method sets through the running compiler's parser/types, retaining deterministic records and incomplete context errors. Removed the temporary API-request rejection.

Synthetic source tests prove added/removed/changed APIs, constants, pointer/value method sets, unexported exclusion, deterministic records, declaration-only skipping and failed-load behavior. Actual parent inventories under each compiler include tls.Config, x509.Certificate, metrics.Description and debug.BuildInfo; native and native-race source contexts also pass without executing race builds/tests. The first full test run accidentally assigned native contexts to a bridge-only fixture with no native contracts; corrected that test's context set. Both final complete spec suites pass (Go 1.25.14 6.705 s, Go 1.26.9 6.750 s), as do command builds, profiles write/check, formatting and diff checks. About 235 authored Go/test lines, below 500. No restricted suite/certification. Four implementation commits since checkpoint 3; snapshot input identities follow.

## Task 25.2.1 — Inspector and module identity boundaries

Date: 2026-10-21. Implementation commit 29 (this commit).

Split Task 25.2 before execution into hashing and workspace preparation. InspectorDigest walks imports from the generator request construction and audit package across every non-test Go source variant, independent of active compiler build tags. It excludes data and exact generated support/revision artifacts, hashes sorted path/content inventories, rejects nonregular/unstable reads and observes cancellation. ModuleInputs binds a portable selected external graph, caller/workspace and framework module files, and the actual relevant external module source trees. It excludes the framework source tree and private generated compiler-specific directives, which belong to separate implementation/transient identities. Source directory selection is checked against the resolved graph.

Tests verify future-tagged dependency changes and request construction invalidate, while target definitions/provenance/history/support/tests/docs do not; authored similarly named selectors are retained. Replacement content, graph versions and caller module bytes invalidate; source hashes are independent of checkout location; unselected/different module directories, symlink implementation and cancellation fail. About 380 authored Go/test lines, below 500. Both baseline audit tests and focused spec boundary/revision tests pass (audit 0.283/0.282 s, spec 0.811/0.818 s), as do native command/audit builds, profiles write/check, formatting and diff checks. No restricted suite/certification. Workspace construction and before/after checks remain Task 25.2.2. Five implementation commits since checkpoint 3.
