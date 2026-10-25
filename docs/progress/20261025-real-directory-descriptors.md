# Execution record: real directory descriptors and os.Root

Plan: docs/plans/20261025-real-directory-descriptors.md
Target: docs/brainstorms/20261025-real-directory-descriptors-target.md
Branch: feat/real-directory-descriptors
Started: 2026-10-25

## Execution authorization and starting state

The user directly requested execution of the plan as the active goal. All nine tasks initially have unchecked items. The incomplete prototype is present and will be reconciled in place. The requested peer plan review did not complete because the peer reached its session limit; no review verdict is inferred. Race/large/certification and infrastructure starts remain excluded. Native evidence cannot be marked complete merely because a runner is unavailable.

Task 1 starts by retaining the draft's real provider/model and hook wiring and removing only its unapproved integer-fcntl/NONBLOCK expansion. Its commit must include the preexisting provider/model source needed by those interfaces, so it is buildable independently; detailed model correctness remains Tasks 2–3.

## Goal addendum — execution scope and alignment

User steering: "arm64 Darwin for the plan. Amd64 we will test in future"; "Use the verified target architecture for aligning take to the target state. Add this info to the goal".

The verified target architecture is the authority for implementation and final delta/wiring verification. Current execution is native Darwin ARM64 plus translated Linux-source regressions under both Gosim families. Native Linux/AMD64 execution is recorded as user-deferred; Linux/macOS code compatibility and reference fixtures remain required. The goal API cannot edit an active objective (only status), so this addendum is recorded in the goal-referenced plan and this execution log. No completion/pause/status reset was performed.

Before the user interruption, adapter generation for both recorded families and model stringer/profile generation finished successfully. No native or translated verification process had yet been started; Task 1 verification continues next.

## Task 1 — typed interfaces, adapters and descriptor flag scope

Date: 2026-10-25. Commit: the task commit containing this entry.

Implemented:
- Adopted the existing provider/model draft and its inode/symlink wiring so this commit builds without uncommitted dependencies. The detailed model/namespace target remains explicitly unchecked in Tasks 2–3.
- Retained routes 84/85, private unix Fstatat/Mkdirat/Renameat/Readlinkat/Symlinkat and syscall/x/sys symlink adapters, feature gates, borrowed views, catalogue counts and mutation-delay wiring.
- Removed the prototype's optional integer fcntl/status-flag expansion and O_NONBLOCK acceptance, keeping validated ENOSYS and Go's disk-file fallback. Kept the inode-bound File representation and ignored CLOEXEC contract.
- Added tests for private syscall.Stat_t selector conversion, unchanged source signature, unrelated-selector preservation and explicit new-route coverage. Regenerated both-family adapters/profiles and the model stringer; formatted touched draft files.

Verified:
- go1.25.14 and go1.26.9: go test -p 2 -parallel 4 -ldflags=-checklinkname=0 -tags=linkname for contracts, kernel, fs, fs/model, routegen, compat/spec and compat/profiles — all seven packages passed in each run.
- Both exact-family generators with -arches=amd64,arm64 -check — exit 0. profiles -check — exit 0.
- Launcher rt,fs tests: TestDirectoryDescriptorRenameAndClose, TestDirectoryNoFollowSemantics, TestRootSymlinkConfinement and TestSyscallMonitorFilesystem, seeds 1–3, under each compiler — 12 passes per compiler.
- .ci/gofmt check and git diff --check — clean.

Evidence: .gosim/validation/gs01/task1/{native-go125,native-go126,generation-go125,generation-go126,sim-go125,sim-go126}.log. Host Darwin ARM64. Source generation for two architectures is not native Linux/AMD64 execution. No restricted suites were run.

Task 1 formatting correction: the initial combined shell command masked a gofumpt failure behind a successful git diff --check. Inspected the actual output, formatted only the two architecture-constant files, and reran .ci/gofmt check separately: exit 0. This was whitespace-only, after the semantic test runs. The task commit is amended with this correction before Task 2.

## Task 2 — component resolution and inode-bound operations

Date: 2026-10-25. Commit: the task commit containing this entry.

Implemented:
- Replaced boolean final lookup options with the verified four final policies and explicit lexical root/dot/dotdot handling. Open creation continues through the same resolver and link counter after existence/exclusive checks, enforcing a single 40-link budget across prefix and final links.
- Distinguished mkdir/symlink existing-entry and trailing-slash rules from open/create and remove/rename rules. Enforced directory/no-follow/create/truncate checks before effects, including RDONLY|TRUNC write access and ignored EXCL without CREATE.
- Added empty/path-length validation before relative FD lookup and target validation before Symlinkat's dirfd. Absolute paths ignore invalid/closed/non-directory descriptors. Kept the accepted early read-only gate.
- Completed symlink stat/dirent metadata, ownership, immutable clone and unlink/rename collection behavior. Directory allocations are detached until publication; held removed directories reject new entries and Getcwd fails.
- Added nine model/provider regression tests with semantic subcases: component traversal, failure atomicity, shared symlink budget, detached directories, persistent symlink/clone isolation, all *at descriptor bases, identity/reference rollback, foreign instances, metadata/read-only denial/re-enable.

Verified:
- go1.25.14 and go1.26.9: native fs/model and fs suites passed.
- Both compilers on native Darwin ARM64: TestDirectoryDescriptorRenameAndClose, TestRootSymlinkConfinement, TestDirectoryNoFollowSemantics passed.
- Both compilers, rt,fs simulation, seeds 1–3: the three public tests plus TestSyscallMonitorFilesystem passed (12 runs each).
- .ci/gofmt check and git diff --check passed. No new public API/configuration, concrete-provider boundary crossing, host fallback or random stream was added.

Evidence: .gosim/validation/gs01/task2/. Native Linux errno/removed-directory dotdot qualification remains explicitly deferred under the user's Darwin-only scope. Full namespace topology and ordering follow in Task 3; no such coverage is inferred from these tests.

## Task 3 — namespace validation and minimal crash dependencies

Date: 2026-10-25. Commit: the task commit containing this entry.

Implemented:
- Finished namespace type/permission/special-name/ancestor validation and held replacement behavior. Same-inode rename is a no-op before parent write checks; invalid operations produce no pending mutation.
- Only directory renames publish depKeyDirTopology with depStoreAs. Dependencies include destination Q itself and its non-root ancestor chain. Removing/replacing empty X depends on WriteDir(X).
- Kept mkdir, ordinary file/symlink operations and source ancestry free of topology edges; topology is not a direct fsync root. No extra dependency-only inode retention was added.
- prepareDirOp owns its input slices, which are immutable after insertion. Existing crash-plan dependency snapshots and part copying remain sufficient. Added cycle guards for ancestor and Getpath traversal and regenerated the dependency stringer.

Verified:
- Native fs/model and fs suites passed with go1.25.14 and go1.26.9 on Darwin ARM64.
- B-out-of-A/A-into-B enumerates exactly the three legal states, without a cycle.
- Child fsync still permits its unsynced parent link to disappear. Syncing a moved directory alone leaves its own move uncertain; a later flushed child-directory move deliberately retains that move.
- Unrelated file/sibling renames and file bytes retain both old/new outcomes. Removed/replaced empty directories cannot recover their previously deleted children.
- Tests cover namespace failure atomicity, open overwritten handles, same-inode permission ordering, owned preparation inputs and corrupt-parent invariant detection. All crash fixtures have a 256-state hard test bound.
- .ci/gofmt check and git diff --check passed. Logs: .gosim/validation/gs01/task3/.

No new randomness, explicit topology fsync root, provider cross-import or general directory GC was introduced. Translated public CrashStates coverage remains Task 5, rather than inferred from these model tests.

## Task 4 — bounded directory enumeration and fatal failure transport

Date: 2026-10-25. Commit: the task commit containing this entry.

Implemented:
- Preflight the complete sorted dirent batch using the same length helper as encoding. Overflow returns UnsupportedOperation with fs/getdents64 and the actual invocation epoch/PC before changing bytes or EOF state.
- Directory Seek(0, SeekStart) rewinds the descriptor; other seeks remain EINVAL. Validity/readability/type checks cannot be bypassed by previous EOF. Regular-file seeking is unchanged.
- The authored route generator now preserves *contracts.Failure before errno conversion. Regenerated dispatch/profiles; ordinary errno and unmodeled-error classification remain intact.

Verified:
- Provider tests: zero/partial/one-byte-short overflow, exact fit, file/directory/symlink dirent types, unchanged buffer/cursor, empty directory, closed/unreadable/non-directory resources, rewind and independent handles.
- Actual kernel/engine test: overflow cannot return or recover; UnsupportedOperation diagnostic carries correct run, machine, epoch and PC; all borrowed invocation fields and descriptors are cleared, with no partial bytes written.
- fs, kernel and routegen passed under go1.25.14 and go1.26.9 on Darwin ARM64. Both generators passed -check for amd64/arm64; profiles -check, .ci/gofmt check and git diff --check passed.

Evidence: native logs in .gosim/validation/gs01/task4/; generator/profile/format checks returned exit 0. The full translated expected-failure harness follows in Task 5. No restricted suites ran and no Linux getdents pagination claim is made.

## Task 5 — public APIs, portability, guards and translated crash states

Date: 2026-10-25. Commit: the task commit containing this entry.

Implemented:
- Expanded common descriptor/Root coverage through actual os/x/sys calls: cwd changes, absolute/mixed operands, raw intermediate-link escape versus Root confinement, nested Root and FS wrapper methods, metadata, held replacement and rewind.
- Added deliberately shared Linux/sim terminal/rename policy tests; native-only Linux status/seek/permission/removed-parent references use linux && !sim. Darwin-only checks preserve native F_GETFL, F_FULLFSYNC, Fstatfs and complete over-buffer enumeration. Sim-only tests retain the approved fcntl/NONBLOCK/read-only/seek limitations.
- Added five public CrashStates scenarios with a 128-state bound: directory-cycle avoidance, missing-parent sync plus symlink retention, own-link/dependent-child moves, emptied removal/replacement and unrelated namespace/data uncertainty. All snapshots, iterators and machines are closed/destroyed.
- Added source-selection checks for both Go families and native/simulated Linux/Darwin contexts, and reused existing fs determinism/seed metatests.
- Added a focused guard helper with nine cases: six disabled symlink/readlink/Root init/test cases, unsupported Root.Link and Chtimes, and fatal over-buffer enumeration. It uses the normal launcher and records identities, source/fixture hashes, commands and diagnostics; no candidate/large/race/matrix path is invoked.

Verified:
- Both compilers on native Darwin ARM64: eight filesystem top-level tests plus the selection-inventory test (eight selection subcases) passed.
- Both compilers in rt,fs simulation: 16 top-level tests × seeds 1–3, plus nested subcases, passed.
- Fresh matching build-tests artifacts and rt,fs config: existing CheckDeterministic and CheckSeeds passed for the new common/simulated cases. After adding the final Linux rename/reference assertions, reran the updated Linux cases through those same metatests; passed under both families.
- Guard reports for go1.25.14 and go1.26.9: 9/9 cases passed each, including three package-init failures explicitly labeled as init execution rather than test-body passes. Two helper unit tests reject setup/compile/timeout/zero-exit/wrong-diagnostic/recovered/host-effect outcomes.
- .ci/gofmt check and git diff --check passed. Logs: .gosim/validation/gs01/task5/ and test-reports/gs01/guards/{go125,go126}/report.json.

Findings corrected during this task:
- Go os.Rename itself refuses an existing directory target before the syscall. The held-replacement fixtures now use the target's actual unix.Renameat contract; no production workaround was added.
- Minimal guard modules lacked the full reviewed shim dependency graph. Their isolated go.mod/go.sum now inherit exact framework requirements, followed by Go dependency preparation; translator read-only graph checks remain unchanged. Preparation failures were reported as failures, never expected abort passes.
- Package-init aborts precede the testing banner; the classifier now distinguishes init from test phase, while requiring the exact failure kind/feature/operation, machine/epoch and nonzero PC. Native Linux/AMD64 execution remains deferred by user scope; no results were invented for it.

## Task 6 — pinned consumer harness (incomplete: translator blocker)
Date: 2026-10-25. No Task 6 commit; completed Tasks 1–5 remain committed.

Implemented
- `.ci/gs01-consumer/`: host Go helper with exact Git-object preparation, fresh-output refusal, safe extraction, functional→sim expression rewriting, unchanged production/test-body checks, independent native/sim source selection, approved bootstrap/config and module preparation.
- Manifest records original/final hashes, compiler/source identities, edits, gates/seeds, commands, outcomes and separate B/C/lock limitations. run-a validates execution/pass evidence and copy integrity; it cannot promote compilation or skips to passes.
- Four helper test functions cover expression/legacy handling, extraction hazards, synthetic offline Git/module preparation, selection equivalence, overwrite/mutation detection and false-pass rejection.

Verified and diagnosed
- go1.26.9 helper tests passed. Pinned gquorum 10c78f90aab45267d4dc29265c59f410c1d87c40 prepared with go1.26.5 on Darwin ARM64: 1,421 files, 483 constraint edits, 714 unchanged production files, 564 unchanged test bodies. Linux functional/sim selections match: 30 files and 25 tests.
- Initial worker-only dependency preparation left missing full-graph go.mod checksums. Added explicit `go mod download all` in the disposable simulation copy; preparation and readonly graph resolution then succeeded. Original consumer requirement minima and Go directive remain verified.
- Gate A cannot reach execution: the external disk test imports public gquorum, which is recompiled for disk.test. classifyPackage treats that indirect variant as a direct test package; selectGraph fails at `internal/translate/graph.go:152`: `test package github.com/asalimonov/gquorum [.../disk.test] lacks base github.com/asalimonov/gquorum`.
- Independent minimal parent/leaf/external-test fixture passes both native tests on Darwin ARM64/go1.26.5, but rt-only translation gives the same missing-base failure. This is independent of FS routes, statx, listing limits or consumer test bodies. No consumer test ran; none passed or was skipped.
- Evidence: `.gosim/gs01-consumer/go1265-v3/manifest.json` and command logs; `.gosim/validation/gs01/task6/{helper-tests,prepare-v3,gate-a-run-v3,testvariant-native,testvariant-sim}.log`; minimal fixture in `testvariant-repro/` beside those logs.

Scope boundary
- Task 6 explicitly requires returning newly required capabilities outside the accepted target for scope resolution. The proposed next change is translator support for indirect test-dependency variants, including graph selection, typed reload and identity-preserving regression coverage. No translator code was changed, no consumer source/imports/assertions were bypassed, and the verified target was not edited.
- Tasks 7–9 are not started. Consumer B/statx, C/hardlink and lock-exclusion dependencies remain separate; native consumer checks remain pending, and native Linux/AMD64 execution remains user-deferred. Task 6 checkboxes are partial, with acceptance unchecked.
- Final helper checks: go1.26.5 verification of the v3 native/sim copies passed after the failed translation; `.ci/gofmt check` and `git diff --check` passed. The v3 manifest reports A as blocked before test execution. A scope-decision question was sent to the user; the active goal remains open.

## Task 6 resumed — translator scope approved
Date: 2026-10-25. User decision: “Include the translator fix in Task 6.”

Proceed with indirect test-dependency classification, graph handling, typed reload and focused regression coverage; retain the unchanged consumer source/assertions and rerun gate A. The existing filesystem target remains authoritative, with this explicit user-authorized extension.

## Task 6 — completed after approved translator repair
Date: 2026-10-25. Commit recorded by the Task 6 feature commit.

Implemented
- Induced ordinary dependencies retain their full production declarations, original package IDs and import edges. Direct package-under-test variants still translate only test additions. Typed reload requests the owning test context and dependency syntax when an indirect variant is a cache miss; it does not fabricate a base or alter consumer imports.
- Added classification/graph/cache regressions and an ordinary focused CLI fixture covering globals/initializers, imports of test-only exports, multiple roots, native execution and two translated runs with identical identity.

Verified
- Three new translator tests plus existing relevant graph/cache checks passed under go1.25.14 and go1.26.9. The CLI fixture executes three native tests and three translated tests × two seeds × two runs per compiler. No large/race suite was invoked.
- Fresh pinned consumer harness v4 with go1.26.5: all four gate-A journal tests passed at seeds 1–3. Logs contain 12 top-level passes / 36 passes including subtests, zero skips. Parallel assertions and real Root/*at operations remain unchanged. The harness verified native/sim source hashes, production bytes and all test bodies after execution.
- Evidence: `.gosim/gs01-consumer/go1265-v4/manifest.json`, identity in command-03.log, acceptance in command-04.log; translator and preparation logs under `.gosim/validation/gs01/task6/`. Helper tests, formatting and diff checks passed.

Limitations
- The repaired initial graph failure is no longer a blocker. Simulation archive/statx (B), hardlink staging (C/hardlink) and lock-exclusion remain their distinct GS-03/GS-02 limitations. Other staging shapes were not selected in this gate-A run. Native consumer and final-source ordinary validation remain Task 8 obligations.

## Task 7 — capability metadata and current architecture
Date: 2026-10-25. Commit recorded by the Task 7 documentation/metadata commit.

Implemented
- Replaced obsolete cwd-only/no-symlink capabilities with inode-bound descriptors, final-link policies, fatal complete-batch enumeration and directory rewind. Added a full Root semantic method inventory without additional Root hooks or static method rejections.
- Updated only the shared Go 1.25 capability reasons; Go 1.26 inherits them. Added checks for inherited limitations and the complete supported/rejected Root inventory.
- README, design, features and syscall monitoring now describe real semantics, topology fsync effects including move-E-into-D, 36 rt/fs and 45 full names, and the separate native/simulation consumer gates. Historical audit and certification evidence were not rewritten. Existing unsupported boundaries retain explicit TODOs; early read-only precedence and unsupported open modes are now annotated.

Verified
- FS, spec, profiles and routegen packages passed under go1.25.14 and go1.26.9. An initial compile error in the new external spec test was corrected by qualifying spec.Capability; both full spec reruns passed.
- Profiles generation updated revisions_gen.go only. Independent adapter/source checks passed for go125/go1.25.14 and go126/go1.26.9 on both Linux source architectures; final profiles check passed. No adapter bytes, origins, baseline versions or historical evidence changed; no stringer regeneration was needed.
- `.ci/gofmt check` and `git diff --check` passed. Reviewed current docs for obsolete directory-descriptor/symlink/catalogue claims; none remain in the changed current-state documents.
- Logs: `.gosim/validation/gs01/task7/`. Source/metadata changes invalidate prior final-identity evidence; Task 8 performs the final ordinary/native/simulated qualification within the user’s Darwin ARM64 scope.
