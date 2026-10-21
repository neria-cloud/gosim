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
