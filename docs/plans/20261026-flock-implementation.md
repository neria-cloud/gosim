# GS-02 implementation and review record

Target: `docs/brainstorms/20261025-flock-ownership-target.md` (local design).
Architecture review: [thread 1](../review/feat_flocks.md).
Baseline: `08cb5b5`; implementation authorized 2026-10-26.

## Decisions and review dispositions

- F1: user approved immediately grantable LOCK_EX on 2026-10-26.
  Accept free/same-owner exclusive acquisition; abort when a blocking call would
  wait. Preserve nonblocking EWOULDBLOCK and reject shared/record locks.
- F2: accepted. Protect individual cleanup operations, continue across mappings,
  machines and recovery handles, clear ownership, then report aggregate failures.
- F3: accepted. Descriptor destruction is the release boundary; delayed Go I/O
  may retain a descriptor after os.File.Close returns. Add a translated regression.
- F4: accepted. Document absent GC-driven close; do not add nondeterministic release.
- F5: accepted as a determinism clarification: translated map operations may use
  scheduler randomness. No explicit lock RNG or change to map/runtime internals.
- F6: accepted. Separate mapping index maintenance from resource release so bulk
  detach does not repeatedly scan the provider's mapping-order slice.
- F7: accepted. Update all actual constructor sites and old stub assertions.
- F8: correct operator spelling when implementing; the spaced binary AND/unary
  complement expression is valid Go too, so this is not an independent defect.

## Implementation iteration

- [x] Host-selected provider policy, ownership, mapping and cleanup lifetime.
- [x] Provider, composition and kernel regressions; update obsolete stub tests.
- [x] Native Go and translated acceptance/lifetime/replay regressions.
- [x] Capability metadata, generated revisions, current architecture and limits.
- [x] Focused validation and `make test`; inspect reports, record skipped work.

## Review and completion

- [x] Request the same colleague's review of the completed implementation.
- [x] Verify findings, fix in collaboration and re-review material fixes.
- [x] Run appropriate focused checks and ordinary validation after fixes.
- [x] Final commit (this change; authorized); no push.

No race, large suite, manual certification or container work is authorized.
Native Linux execution requires an available native Linux worker; absence must
be reported rather than replaced by cross-compilation or Darwin results.

## Progress

- 2026-10-26: verified review against kernel cleanup, provider session teardown,
  translated-map insertion and selected Go internal/poll Close implementation.
  Committee was incomplete due to account limits: one external review plus
  independent coordinator verification, not two-member consensus.

- First ordinary snapshot: 2,262 passes, zero test failures, 20 skips and two
  package-error report entries for one missed fp.NewProvider fixture argument.
  Fixed that fixture and verified its package. Also migrated Linux-only old
  record-lock stub assertions; native Linux execution is still unavailable.
- User approved the F1 refinement; implementing it before the final ordinary
  rerun and etcd WAL check. The implementation review is running in thread 2.

- Go 1.26.9 focused provider/kernel/composition/capability checks pass on Darwin
  ARM64. A focused translated Go 1.26.9 run is in progress. The real bbolt
  TestBbolt example passes with close/reopen. Second ordinary run and etcd
  TestWal are in progress after the approved F1 change.

- Second ordinary snapshot passed: 2,297 passes, zero failures/errors, 20 skips
  in 232.938 seconds. Final review fixes were made concurrently in the working
  checkout, so a third fresh ordinary snapshot is now running.
- Implementation review Round 3 confirmed every finding fixed, with no new
  production defect. The final changes clarify all summaries/consumer outcome
  text, mark the native child helper skipped, and add a Linux-policy kernel
  mapped-crash/restart regression. That regression passes on Go 1.25.5 and
  Go 1.26.9. Consumer-helper synthetic tests require Go 1.26 modules: the initial
  Go 1.25.5 invocation failed to build its fixture; all helper tests pass when
  invoked with Go 1.26.9. No real consumer qualification is claimed.
- Profiles generator check, formatting and git diff whitespace checks pass.
  Final ordinary validation is the only remaining gate before the commit.

## Final validation and disposition

Completed on Darwin ARM64. Final `make test` used Go 1.25.5 and passed in
230.417 seconds: 2,318 reported cases, 2,297 passes, zero failures, zero errors,
21 skips. Counts include parent/subtests and repeated invocations, not unique
semantic scenarios. Reports are in `test-reports/unit/latest/`; report.md and
timeline.md were inspected. The child-process helper is explicitly skipped
when not invoked by its parent. No skipped case is counted as a pass.

Final-source focused native provider/kernel/composition/capability checks and
translated TestFlock cases passed with Go 1.26.9 (seeds 1–3). The real etcd
TestWal and bbolt TestBbolt examples passed again after the last metadata
regeneration. Native Darwin syscall/dup/mmap/process-exit checks passed; the
Linux lifetime policy additionally passed kernel crash/restart tests on both
Go families. The profiles generator check, Go formatting and staged whitespace
checks passed. Consumer-helper synthetic tests passed with Go 1.26.9.

The colleague verified all implementation-review corrections in thread 2 round 3
and reported no remaining findings. User-approved immediately grantable LOCK_EX
is included; waiting, shared locks, record locks, dup and shared volumes remain
unsupported. No native Linux execution or full pinned gquorum qualification is
claimed. No race/large suite, certification, container or cross-architecture job
ran. Architecture brainstorms remain local in the repository's ignored directory.
