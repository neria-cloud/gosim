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
