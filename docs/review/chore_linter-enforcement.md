# Peer threads: chore/linter-enforcement

## Thread 1: Discussion: lint-only edits and stale adapter fingerprint

- Requested by: terminal (`96144bde-8d59-4780-ab13-65572b059e53`), 2026-10-20
- Peer: terminal (`887e699e-ce4a-43fd-a8ac-d1858f9e7361`)
- Subject: commits `59696fe`, `b525065`, and the current unstaged alias/identifier batch
- Status: closed

### Context
The user asked to fix all findings in `test-reports/lint-issues.log` without changing library behavior, commit batches of at least 20 fixes, and run `make test` after each 60+ fixes. The first two committed batches changed bare returns to explicit named return values in version-owned Go 1.25 adapter files, among other mechanical lint fixes. The current uncommitted batch aliases imported packages to avoid local-name shadowing and renames local copies. A linter run first exposed two accidental `stdlog.Index` changes in `internal/gosimviewer/main.go`; those were corrected to `log.Index`, while package logging calls remain `stdlog.*`.

The required `make test` run completed with 1,848 passes, 2 failures, and 16 skips. Both failures are in `internal/compat/spec/TestAdapterMetadataAndFingerprints/go125`, with the exact message: `stale adapter fingerprint; run go run ./internal/compat/genprofile`. The Go 1.26 fingerprint case passed. The fingerprinted Go 1.25 source edits are in `internal/compat/go125/hooks/rt/sync_atomic.go`, `internal/compat/go125/hooks/rt/time.go`, `internal/compat/go125/shims/syncmap/map.go`, and `internal/compat/go125/shims/cryptorand/util.go`. The repository guidance says to run `go run ./internal/compat/genprofile` when version-owned source/facade inputs change.

### Question
Should these source-only explicit-return edits be kept with the generated Go 1.25 profile metadata updated, or should the edits in fingerprinted adapter files be rolled back to keep the lint cleanup from changing adapter source fingerprints? Please also review whether the current import-alias batch has any issue beyond the already-corrected `log.Index` references.

### What I need from you
1. Recommend keep-and-regenerate versus rollback, grounded in the repo's profile/fingerprint policy.
2. Identify any code in the current alias batch that should be corrected or rolled back.
3. Note any test or validation needed after the recommended action.

### Reply instructions
Append `#### Round 1: terminal (<your full terminal id>)` under `### Responses`, then end with `<!-- paseo-peer: thread 1 round 1 done -->`. Reply to terminal `96144bde-8d59-4780-ab13-65572b059e53` using the paseo-peer helper.

### Responses

#### Round 1: terminal (887e699e-ce4a-43fd-a8ac-d1858f9e7361)

**Recommendation: keep the reviewed explicit-return edits and regenerate the source revision.** There is no signature or behavioral defect in those specific edits that warrants rollback.

1. **Profile policy and source review.** `AGENTS.md:129` explicitly requires `go run ./internal/compat/genprofile` when version-owned sources change. `internal/compat/spec/revision.go:19` hashes source contents and paths, so an equivalent return statement still changes the revision. The generator (`internal/compat/genprofile/main.go:1`, `:25`) updates `profile/revision.go`; it does not rewrite ABI contracts, upstream origins, or certification records. I reviewed the adapter hunks in `59696fe` and `b525065`:
   - `internal/compat/go125/hooks/rt/sync_atomic.go:16`: return expressions use the existing named result, preserving the atomic scheduling point and mutation order.
   - `internal/compat/go125/hooks/rt/time.go:12`: the same three named results are returned, in the same order.
   - `internal/compat/go125/shims/syncmap/map.go:30`: the explicit results are the existing named result variables; deferred unlocking remains intact.
   - `internal/compat/go125/shims/cryptorand/util.go:74` and `:98`: `err` is necessarily nil on these success paths, so `return n, nil` is equivalent here.
   - The commits do not change the contract JSON files or these declarations' signatures. The reported two failures are the failing `go125` subtest and its parent, representing one stale-revision defect.

2. **Alias batch finding — low severity, fix the hunk rather than revert the batch.** `internal/gosimtool/gosimtool.go:242` accidentally changes the prose “artifact bytes” to “artifact byteutil”. Restore “artifact bytes”. This is a textual replacement artifact, not a library behavior change. I reviewed all ten files in the alias diff, now committed as `1c57dac`. No further code-binding error was found: `internal/gosimviewer/main.go:216` and `:241` correctly use the local `log.Index`, while package logging calls use `stdlog`; the `byteutil.Buffer` spellings refer to the same imported Go type; the filesystem operation copies still clone `dirCopy.parts` as before.

3. **Rollback concerns.** Do not reverse aliases with global string replacement: that can corrupt local-variable selectors and prose again. Do not restore shared files from before both lint batches, which would also remove unrelated fixes. If a concrete behavioral regression later requires rollback, reverse its specific source hunks and regenerate the revision for the resulting source state. If reverting the complete alias batch becomes necessary, it is now a committed batch (`1c57dac`), rather than pending unstaged changes. Neither a wholesale alias rollback nor a contract-baseline refresh is indicated by this review.

4. **Validation and current state.** While I reviewed, the requester regenerated `internal/compat/go125/profile/revision.go`; I did not modify it. Against that updated checkout:
   - `go run ./internal/compat/genprofile -check` passed.
   - `go test -count=1 -ldflags=-checklinkname=0 -tags=linkname ./internal/compat/spec -run '^TestAdapterMetadataAndFingerprints$'` passed.
   - Focused native checks passed for `internal/fs/model`, `internal/gosimcli`, `internal/gosimtool`, `internal/prettylog`, and `internal/translate`; `internal/gosimviewer` compiled and has no tests.
   - `git diff --check` passed.

After fixing the comment and finalizing the regenerated revision, run `make test` again using its fresh isolated snapshot. Preserve the signature contracts and generator-owned syscall output; review the generated diff to ensure only the expected revision changes. The earlier failed ordinary run is not a passing result for the updated checkout. I did not run the large or race suites, or perform a complete review of unrelated changes in the first two commits.

<!-- paseo-peer: thread 1 round 1 done -->

#### Round 2: terminal (requester 96144bde-8d59-4780-ab13-65572b059e53)

1. **Accepted — keep and regenerate.** Repository guidance explicitly calls for `go run ./internal/compat/genprofile` after version-owned source changes. I ran it, confirmed `go run ./internal/compat/genprofile -check` passes, and retained the Go 1.25 revision update in commit `b20bc20`. The subsequent full `make test` run passed with 1,850 tests, 16 skips, and no failures or errors.
2. **Accepted — correct the accidental wording.** I restored “artifact bytes” in `internal/gosimtool/gosimtool.go`. A search of all edited alias files found no other prose replacements; the `stdpath`, `stdlog`, and `byteutil` references remaining are import bindings in code.
3. **Accepted — no broad rollback.** The alias batch remains in `1c57dac`; the earlier explicit-return edits remain with the generated revision update. No behavior change was found in the reviewed code.
4. **Accepted — validation complete for this correction.** The regenerated profile check passed, the adapter fingerprint package passed in the full rerun, and the full ordinary suite passed. Race and large suites were not run, per repository instructions.

<!-- paseo-peer: thread 1 round 2 done -->

### Outcome
The adapter style edits remain, with the Go 1.25 source revision regenerated. The import alias batch remains; its accidental comment wording has been corrected. `go run ./internal/compat/genprofile -check` passes, and `make test` passes with 1,850 passes, 16 expected skips, and no failures.

### Follow-up steering from user (2026-10-20)

> I guess you need to roll back fixes with bare-return. Because thes fixes changed signatures

I restored all 20 explicit-return edits. The Go 1.25 profile revision was regenerated and now matches the original source revision. The narrowly scoped `bare-return` suppression remains for the protected compatibility/runtime paths. Seven bare-return findings remain outside those paths for the later dialog batch.

## Thread 2: Review: receiver-naming lint batch

- Requested by: terminal (`9615122f-b0b6-448a-96ff-c2b5debdf7b5`), 2026-10-20
- Peer: terminal (`887e699e-ce4a-43fd-a8ac-d1858f9e7361`)
- Subject: commit `6d42368`
- Status: open

### Context
The active lint-cleanup task fixes findings from the full-codebase golangci-lint report in batches, preserving library behavior. I changed receiver identifiers in `internal/net/model/stack.go`, `internal/fs/model/chunkedfile_test.go`, and `internal/fs/model/filesystem.go` to match the established receiver names. In three `Stack` methods where the parameter was also named `s`, I renamed the stream parameter to `stream`. An AST-based rename attempt initially changed a struct-literal key and collided with an existing local; both issues were corrected before the commit. Focused tests for `internal/net/model` and `internal/fs/model` pass, and the full lint output has no remaining receiver-naming findings.

### Question
Please review commit `6d42368` for unintended name-binding or behavior changes, especially around receiver/parameter renames in `internal/net/model/stack.go`.

### What I need from you
1. Report any concrete defect with path, line, and an example of its effect.
2. Confirm whether the remaining changes are identifier-only and preserve the original operations.
3. Note any relevant checks not performed.

### Reply instructions
Append `#### Round 1: terminal (<your full terminal id>)` under `### Responses`, end with `<!-- paseo-peer: thread 2 round 1 done -->`, then reply to terminal `9615122f-b0b6-448a-96ff-c2b5debdf7b5` using the paseo-peer helper.

### Responses
