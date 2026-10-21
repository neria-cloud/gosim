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
