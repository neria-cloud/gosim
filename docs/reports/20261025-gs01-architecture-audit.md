# GS-01 architecture and execution audit

The implementation matches the verified target within the user-approved Darwin
ARM64 execution scope. The audit found two gaps in explicit regression coverage,
now addressed: the complete Root-to-directory-NewFile chain under both Go
families, and the invalid-dirfd absolute-path matrix for the new link and metadata
calls. No production behavior changed during this audit. Refreshed final-source validation passed and is recorded below; earlier reports
remain tied to their original snapshots.

This is an implementation self-audit, not a new colleague review or full issue
#12 acceptance. Native Linux/AMD64 qualification, simulated archive/statx and
hardlink acceptance, and real lock exclusion remain separate open gates.

## Authority and source

- Target: `docs/brainstorms/20261025-real-directory-descriptors-target.md`, SHA-256
  `7d5716c709dd3d93cca085a4ffaf6e0e0c55d12cd586d82512fe0759f91e31cc`.
- Plan: [20261025-real-directory-descriptors.md](../plans/20261025-real-directory-descriptors.md).
- Execution details: [progress record](../progress/20261025-real-directory-descriptors.md).
- Baseline: d8f82ea. Production implementation through f4d3204; Task 8 evidence
  commit 3d75e59 changes documentation only. Task 9 adds portable regressions and
  this audit. The target itself was not edited.
- Later explicit user instructions authorize plan execution, restrict current
  execution to Darwin ARM64, defer AMD64 testing, and include the translator
  repair in Task 6. These supersede the frozen design's historical authorization
  text and original native-runner schedule, without waiving Linux/macOS source
  compatibility or changing the filesystem target.

## Requirement-to-evidence review

| Target sections | Inspected implementation and evidence | Finding |
| --- | --- | --- |
| Mandatory portability; accepted decisions | Linux constants in flags_arm64.go and flags_amd64.go; native Darwin constants in portable/Darwin test files; unchanged consumer native platform sources; generation checks for both Linux source architectures | Preserved. Native Linux/AMD64 execution is deferred, not inferred from simulation or generation. |
| Contract/ownership; handle representation | internal/fs/file.go, ops.go dirInode/Openat; injected DescriptorTable; lifecycle.go; provider descriptor/epoch/rollback tests | File.inode survives rename/replacement; no path handle or second FD table. Closed/foreign-instance descriptors fail; live non-directory resources give ENOTDIR. Absolute operands ignore dirfd. |
| Data structures; symlink persistence | backingSymlink, backingDir.unlinked, allocOp kind/target, StatResp/ReadDirEntry.IsSymlink, clone/metadata/link-count/GC branches | Exact target structures and methods are present. Targets are immutable metadata, not file chunks; quota remains file-data accounting. Detached directories cannot acquire entries and remain owned until teardown. |
| Provider semantics; flags/errors | Openat, Statat, Mkdirat, Renameat, Unlinkat, Chmodat, Chownat, Symlinkat, Readlinkat, FcntlInt | Flag checks precede effects; DIRECTORY/NOFOLLOW/EXCL, final-link metadata and removal types are explicit. Readlink truncates without NUL. No-follow chmod returns EOPNOTSUPP. Integer fcntl ENOSYS and early read-only denial remain deliberate limits. |
| Model inventory; resolve/open/rename/link algorithms | path.go resolve/resolveWithLinks; filesystem.go OpenFileWithOptions/Remove/Rename; symlink.go | One walker, four final policies, lexical dot/dotdot/root/slash preserved, shared 40-link budget; no walkpath or filepath.Clean remains. Prefix/access checks, existence and directory checks precede mutation. The detailed rename algorithm's same-inode no-op precedes parent write checks. |
| Directory topology/crash ordering | dirOp.addDeps, directoryAncestorsInclusive, prepareDirOp, pendingops.go flushInode/snapshot, crash.go retainOp | Only directory moves publish Topo(D), using store-as. Reads include Q and its non-root ancestors; no mkdir/file/source-chain topology dependencies or direct topology fsync roots. Empty removal/replacement depends on WriteDir(X). Owned slices are immutable; crash plans retain dependency keys separately. |
| Topology soundness and durability counterexamples | model topology_test.go and translated directory_crash_test.go | B-out-of-A/A-into-B cycle excluded; unsynced mkdir link and unrelated data/renames remain uncertain. Syncing moved D alone differs from syncing D after E moves into it. Removed/replaced empty directories cannot recover children. Model checks include unreachable objects and parent pointers, with bounded enumeration. |
| Directory listing and rewind | Getdents64 preflight/encoding, directoryRecordLength, Seek; provider/kernel tests and fatal guard fixtures | One complete sorted batch or fatal UnsupportedOperation before buffer/cursor mutation. Current epoch/PC retained. Rewind resets listing state; other directory seeks, including SEEK_CUR, remain EINVAL. Pagination/alignment/inode-cookie improvements remain deferred. |
| Typed transport; compatibility | FileOps; routes.json; routegen adapters/dispatch; generated architecture hooks and kernel request/dispatch types; profile baseline/inheritance tests | All 83 original route records are unchanged; only Symlinkat=84 and Readlinkat=85 are appended. Private Fstatat uses reviewed Stat_t conversion. Both families inherit shared hooks; generation checks pass. No Statx route, mount ID or redundant Root method hook was added. |
| Failure transport and lifecycle | syscallRouteResult generator branch; Dispatcher.Submit/AbortRun; TestDirectoryOverflowAbortsWithCallerAndClearsBorrowedView; session/lifecycle tests | Typed failures retain classification before errno conversion. Unmodeled errors retain their invariant path. Abort clears invocation views, pending state and descriptor ownership; cannot be ignored/recovered. Staged prepare/abort, epoch guards and domain randomness are unchanged. |
| Monitoring/capability/docs | syscall_stats.go/catalogue tests; capabilities.go; effective family records; README/design/features/syscalls | 8 rt, 36 rt/fs, 17 rt/net, 45 full names. Dirfd remains context, not primary-FD attribution. Symlinkat has mutation latency; Readlinkat is immediate. Root rows and all declared limitations match underlying routes. Historical audit/evidence remains historical. |
| Complete Root surface | common directory tests and Root FS interface checks; new RootDirectoryDescriptorChain; rejected-method guards | Name/Close, open/create/nested-root, metadata, mkdir/remove, stat/lstat, readlink/symlink/rename, ReadFile/WriteFile and FS methods are reached through real APIs. Link/Chtimes reach fatal unsupported calls. Repeated close, independent child lifetime and exact NewFile directory fallback now have explicit both-family coverage. |
| Verification applicability/determinism | file constraints and selection inventory; ordinary FS metatests; native Darwin F_GETFL/F_FULLFSYNC/Fstatfs and long-listing checks | Common/native-Linux/sim-only/Darwin cases remain separate. Linux reference-only files do not enter simulation. No race/large/certification or additional infrastructure was invoked. Final report counts distinguish skips. |
| Consumer acceptance and disposable harness | .ci/gs01-consumer; pinned Git objects; native/sim hashes; body/header transforms; manifests and logs | Original production code, test bodies, parallelism and assertions preserved. Functional atom substitution preserves both polarities/boolean structure. Gate A is required and executed; B/C and locks remain separate. Native consumer uses original tags/platform paths. |
| Approved translator extension | classifyPackage/testOwner, owning-context typed reload, indirect_test.go | Indirect test dependencies translate full production source, retain original IDs/import edges and reload through the owning test root. Both-family regression covers test-only exports, globals/init, batch roots and cached reruns. Consumer workaround imports or weakened assertions were not used. |
| Exclusions and execution boundary | explicit errors/TODOs, capability records, plan gap ledger | No hard links, mount model, real locks, dup/OFD sharing, special files, O_PATH/O_TMPFILE/openat2, integer status flags, full timestamps/directory link counts or pagination is implied. Hardware power-loss equivalence is not claimed. |

## Real-entrypoint wiring

The composition generator imports and constructs fs.NewProvider only when FS is
selected. Provider.Open creates the session; Prepare injects the existing shared
FD table; Prepared.Bindings exposes the live Instance as FileOps before the
existing staged Publish. Generated syscall, x/sys and private unix hooks require
FS, submit typed requests, and decode results. Generated dispatch obtains the
bound Filesystem service and calls the actual Symlinkat/Readlinkat and existing
*at methods. The provider selects inode bases and calls the single model walker;
Go's unchanged Root code supplies confinement.

Actual native/translated public tests and unchanged consumer journals traverse
this path. The overflow fixture additionally proves the return path to AbortRun,
caller attribution and cleanup. Production import inspection confirms contracts
imports only the runtime ABI within the repository; kernel imports neither
concrete provider, and fs/net do not import each other. Kernel tests compose real
providers only as test fixtures. No extra public control/config or provider
random stream was added.

## Audit corrections

`TestRootDirectoryDescriptorChain` explicitly executes Root.Open(".") → Fd →
directory Openat with DIRECTORY/NOFOLLOW/CLOEXEC → os.NewFile → Sync → Close.
It verifies identity after rename, the child's independent lifetime after parent
and Root close, repeated Root.Close, and closed-descriptor errors. The absolute
operand test now covers Symlinkat, Readlinkat, Fchmodat and Fchownat with an invalid
dirfd, in addition to open/stat/mkdir/rename/unlink and mixed rename operands.

The initial Task 9 ordinary snapshots started before the absolute-matrix
extension. Their owned process groups were deliberately interrupted and terminal
exit 130 was observed; they are not final passes. Fresh final snapshots and
focused checks use the completed test source. No production fix was necessary.

## Final execution evidence

All final commands exited successfully. Both ordinary report.md and timeline.md
files were inspected, together with raw failure/skip events and snapshot records.

| Check | Go 1.25.14 | Go 1.26.9 |
| --- | --- | --- |
| Ordinary make test | 2,240 passed / 18 skipped; zero failures/errors | 2,256 passed / 16 skipped; zero failures/errors |
| Native Darwin descriptor/Root cases | 9 passed, zero skips | 9 passed, zero skips |
| Focused translated FS/Root/crash cases | 17 tests × 3 seeds; 51 top-level / 63 total passes | 17 tests × 3 seeds; 51 top-level / 63 total passes |
| Fatal boundary fixtures | 9 passed | 9 passed |
| Existing FS determinism/seed metatests | Passed, including the added directory chain | Passed, including the added directory chain |

The ordinary skips are five large-only entries, two race-disabled tests, one
subprocess helper, and eight expected-failure/logging fixtures excluded by the
positive runner; Go 1.25 additionally skips two Go-1.26-only audit cases. None is
a GS-01 semantic case. Counts include parent tests/subtests; skips are not passes.
No race, large suite, certification, VM, Docker or remote runner was invoked.

Both snapshots bind input digest
`75c95a5c22c17a07412e63f93549005f1e87180f50b00fc934a27ddba1561d8b`
(705 inputs), with status passed and no checkout changes. The final portable
test-file SHA-256 is
`679071eead13d6aa4205b1cb218b0ffbbd855ef74ee8990f90638911ccf281a4`.
Guard manifests bind source hash
`17e17640951a8f3922db6d0f1054a9ab4061a83754e23e15c2962c46a54184d5`;
every recorded source-file hash was compared with the final worktree.

Reports are in
[test-reports/gs01/audit-go125/unit/latest/report.md](../../test-reports/gs01/audit-go125/unit/latest/report.md)
and the corresponding
[Go 1.26 report](../../test-reports/gs01/audit-go126/unit/latest/report.md).
Exact commands, timings, skips and selections remain in the adjacent reports and
`.gosim/validation/gs01/task9/summary.json`. Final guard evidence is under
`test-reports/gs01/audit-guards/{go125,go126}/report.json`.

The final-source consumer v6 harness passed all four unchanged journal tests at
seeds 1–3 under Go 1.26.5 (12 top-level / 36 total passes, zero skips), then passed
integrity verification. See `.gosim/gs01-consumer/go1265-v6/manifest.json`.
Native consumer evidence from Task 8 remains applicable: 13 original functional
tests / 33 cases passed on Darwin ARM64, including A, native archive/artifact and
all staging alias shapes. All 1,421 original native source hashes, the revision
and compiler are identical between v5 and v6; no native consumer source changed,
so those successful native tests were not redundantly rerun. Exact native
selection is in `.gosim/validation/gs01/task8/consumer-native.json`.

Both-family generation/profile checks from Task 7 remain current: generated
adapters, origins and compatibility source have not changed since those checks.
The only executable-input change after Task 8 is the portable regression file.
Final formatting and diff checks pass. No required local implementation or wiring
gap remains.

## Remaining qualification boundaries

Native Linux ARM64 and Linux AMD64 remain user-deferred, including native Linux
errno and held-removed-directory dotdot comparisons. Both source architectures
are generated/checked; that is not execution. Native Darwin A/B/C passes do not
satisfy simulated B/statx (GS-03 increment 2), C/hardlink (GS-03 increment 1), or
GS-02 lock contention/lifetime. The journal tests inject returned errors and
reopen files; separate CrashStates tests establish model ordering, not actual
hardware power-cut equivalence. Full issue #12 acceptance therefore remains open
beyond this completed implementation scope.
