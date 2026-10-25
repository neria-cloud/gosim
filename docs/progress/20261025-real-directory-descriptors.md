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
