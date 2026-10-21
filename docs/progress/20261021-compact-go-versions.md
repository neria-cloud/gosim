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
