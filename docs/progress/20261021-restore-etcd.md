# Restore etcd implementation progress

Plan: `docs/plans/20261021-restore-etcd.md`
Target: A1–A9 embedded in that plan, with `docs/design.md` and `docs/MAINTAINER_GUIDE.md`
Branch: `feat/compact-go-versions`
Started: 2026-10-21

## Tasks 1–2 — finite contract and guarded crypto admission

Date: 2026-10-21. Pair commit contains this entry.

Implemented

- Recorded the pinned etcd/JWT/jsoniter/reflect2/trace admission frontier in `internal/compat/spec/testdata/etcd_contract.md`; `go mod verify` passed under Go 1.25.14 and 1.26.9.
- Added shared ECDSA, elliptic-curve and RSA declaration facades with fatal algorithm guards. Go 1.25 owns package/bridge/declaration/capability/probe records; Go 1.26 inherits them without a copied implementation or metadata delta.
- Completed the guarded X.509 signatures needed by pinned transport/JWT source, including typed EC/RSA key results.
- Independently refreshed both compiler origins and regenerated profile revisions. The larger origin files record the new upstream source closure.
- Added a translated fixture for key identity/metadata and a separate expected-abort fixture. The unreviewed packages remain rejected.
- Pruned generated nested `.gosim` cache directories from `.ci/gofmt` discovery after the ordinary gate exposed a false formatting failure.

Verified

- `make build GOSIM_TOOLCHAIN=go1.25.14` and `go1.26.9`: pass.
- Profile, shim and upstream generator checks for AMD64/ARM64 under both compiler baselines: pass.
- Focused `internal/compat/spec`, `profiles`, `shims` and feature tests under both compilers: pass.
- Translated `TestCryptoDeclarations` under both compilers: pass. Explicit translated `TestCryptoGuard`: exit 1 with `UnsupportedOperation` for `crypto/ecdsa.GenerateKey` under both compilers; attempted application recovery did not continue.
- Focused etcd translation moved from `crypto/ecdsa` rejection to the expected `github.com/modern-go/reflect2` rejection. This is the next task boundary, not positive etcd support.
- `./.ci/gofmt check`: pass. `make test`: 84 reports, 1,998 pass, 20 skip, no failures/errors; ordinary compiler selected by tooling was Go 1.25.5.

Notes

- The initial full test run found stale revisions after the guard test changed. Regeneration and the final ordinary run cleared them.
- jsoniter v1.1.11 refers to exported concrete reflect2 `Unsafe*Type` names. A4 and Task 1 were corrected before Task 3; pointer execution still needs an explicit modeled implementation or fatal guard.

## Tasks 3–4 — selected reflect2 metadata and disabled observability

Date: 2026-10-21. Pair commit contains this entry.

Implemented

- Replaced the exact pinned reflect2 package with a shared translated metadata facade. It preserves public interface and concrete type names, kind-specific wrappers, per-config canonical type keys, and machine-local mutable state. Pointer and value operations outside this subset abort fatally.
- Added a disabled `runtime/trace` facade: `IsEnabled` returns false; active tasks, regions, logging and trace capture abort. Added the dormant `runtime/debug.Stack` declaration with a fatal guard.
- Corrected the existing `runtime.ReadMemStats` bridge to the exact `*runtime.MemStats` signature required by pinned Prometheus. A reviewed native type import is used solely for the signature; actual collection still aborts.
- Added narrow baseline declarations, capabilities, module-source bindings and GOROOT probes; Go 1.26 inherits the additions. Refreshed both independent compiler origins and profile revisions. The current probe schema cannot name a module path, so reflect2 execution is checked by translated fixtures.

Verified

- `make build` under Go 1.25.14 and 1.26.9: pass. Both compiler profile/adapter generation checks for AMD64/ARM64: pass.
- Focused metadata, feature and changed-shim native tests under both compilers: pass. `./.ci/gofmt check` and `git diff --check`: pass.
- Translated jsoniter/reflect2 initialization, type identity and two-machine state tests under both compilers: pass. `TestUnsafeReflectGuard` exits with `UnsupportedOperation` for `reflect2.PtrOf` under both compilers, including an attempted application recovery.
- Translated OpenTelemetry and Prometheus initialization under `rt,fs,net` on both compilers: pass. `TestActiveTraceGuard` and `TestHostMetricsGuard` exit with the expected fatal diagnostics under both compilers despite recovery attempts.
- `make test`: 86 reports, 1,998 passed, 20 skipped, no failures/errors; wall time 230 seconds. The ordinary suite does not include the testdata fixtures; their translated invocations above provide the execution evidence.

Next boundary

- Focused etcd translation advances past crypto, reflect2 and trace to `go.etcd.io/etcd/pkg/v3/osutil: unsupported syscall.Syscall6`. The current example imports `osutil` for process interrupt handling; Task 5 removes that path through explicit embedded lifecycle. This is not a positive etcd result yet.

## Tasks 5–6 — embedded lifecycle and translated HTTP

Date: 2026-10-21. Pair commit contains this entry.

Implemented

- Replaced process signal, fatal logging and exit orchestration with `RunEmbedded(ctx,cfg,ready)`. The example now configures three isolated machines before start, reports buffered startup/completion outcomes, observes readiness and graceful exits, and closes the client before the servers. Invalid configuration and pre-start cancellation publish one startup error; canceled running servers close normally.
- Restored `TestNetTcpHttp` to the positive feature inventory. It checks exact request bodies, connection reuse, streamed response assembly, explicit cancellation, deadline propagation and graceful HTTP shutdown over simulated TCP. The existing gRPC test remains unchanged.
- Added the translated protobuf private map alias required by pinned grpc-gateway, using the existing alias generator. Added a narrow disabled-query/fatal-enablement bridge for etcd's dormant mutex profiling utility; active host metrics remain unsupported.

Verified

- Focused `TestNetTcpHttp` and `TestNetTcpGrpc` with seeds 1–3 under Go 1.25.14 and 1.26.9: all six test/seed records pass for each compiler. No native HTTP transport was used.
- Focused `TestEtcd` seed 1 under both compilers starts the real three-node cluster, completes the existing KV loop and observes normal shutdown. This is still the historical weak smoke assertion; Task 8 must prove progress before and after repair, exact persistence and seeds 1–3.
- Focused translated invalid-config and canceled-before-readiness fixtures pass under both compilers. Disabled mutex profiling query initializes; active profiling exits with `UnsupportedOperation` despite an attempted recover, under both compilers.
- `make build`, focused metadata/translator/runtime bridge tests, and independent AMD64/ARM64 generator checks pass on both exact compilers. `make lint`, `./.ci/gofmt check`, and `git diff --check` pass.
- `make test`: 86 reports, 2,002 passed, 20 skipped, no failures/errors; wall time 227 seconds. It includes the restored HTTP test in ordinary simulation tests. Positive etcd remains outside the ordinary feature registry until Task 9.

Next boundary

- Task 7 checks protobuf descriptor isolation across repeated runs in one worker. Task 8 strengthens the cluster phase assertions and snapshot/WAL gates; the current smoke pass is insufficient for final support.

## Tasks 7–8 — descriptor replay and phased cluster acceptance

Date: 2026-10-21. Pair commit contains this entry.

Implemented

- Added a real gRPC status/detail descriptor fixture using `google.rpc.Status.code`, `errdetails.BadRequest`, protobuf reflection and JSON encoding in two sequential simulated machines. A native metatest runs seeds 1, 2, 3, then 1 in one matching worker and compares seed-1 checksums and identity fields.
- No descriptor ownership defect reproduced in this focused scenario on either compiler, so no production cache or runtime lifetime code was changed. The full-cluster same-worker check remains in Task 9.
- Replaced the weak single aggregate KV assertion with explicit before, partition and after phases. Every completed put/get checks exact value equality; successful progress is required before partition and within the bounded after-repair window. The existing seeded `PartitionMachines` fault is awaited through repair. Client, server completion and graceful machine exits are observed.
- Kept exact snapshot metadata/data and WAL state/entry reopen comparisons; changed a WAL save error from process-fatal logging to a test failure. Recorded simulated elapsed time and semantic phase results.

Verified

- Translated protobuf fixture with seeds 1–3 in one invocation under each compiler: all pass. Native same-worker seed sequence `1,2,3,1` passes under Go 1.25.14 and 1.26.9 with nonempty checksums, matching identity/replay fields and identical repeated seed-1 checksum.
- The exact `V-ETCD` example command with seeds 1–3 passes all 15 current test/seed records on each compiler, including nine records for the original `TestEtcd`, `TestSnapshotter` and `TestWal` scenarios. No scenarios were skipped.
- Across the three cluster seeds, Go 1.25.14 recorded 6–7 successful pre-partition KV round trips and 16 successful after-repair round trips; Go 1.26.9 recorded 6 before and 16 after. The partition window produced expected unsuccessful attempts without data mismatches. Simulated cluster durations ranged from about 69 to 79 seconds.
- `make build`, focused metadata/translator/feature tests, `make lint`, `./.ci/gofmt check`, `git diff --check`, and profile revision checks pass. A8 cache schema/retention and `ReplayRevision` were untouched; `MetaT` verified exact BuildDigest/ReplayRevision at each run.
- `make test`: 86 reports, 2,007 passed, 20 skipped, no failures/errors; wall time 224 seconds. The ordinary integration suite includes the new fixture and replay metatest. Positive etcd is still excluded from ordinary feature selection until Task 9.

## Tasks 9–10 — ordinary etcd selection, replay and scope documentation

Date: 2026-10-21. Pair commit contains this entry.

Implemented

- Registered `./examples/etcd` as a sim-only suite requiring all three features and added it to the explicit local simulation package list. Reduced-feature package selection excludes it before loading; native metatest package lists continue to omit the sim-only example.
- Added an integration native metatest that uses `ForOtherPackage` and a matching prebuilt etcd worker. It runs `TestEtcd` seeds 1, 2, 3, then 1 in one worker, and seed 1 in a fresh worker. It checks failure/error fields, nonempty checksums, exact build/replay identity and parsed phase counts.
- The first replay exposed a non-deterministic pointer string in the pinned etcd client's optional retry target log. The example now uses a no-op logger for that client; KV phase results and errors remain asserted. Repeat and fresh-worker checksum comparisons pass on both compilers. No runtime checksum, scheduler or replay revision behavior changed.
- Updated the example, feature, root, design and maintainer documentation for the working plaintext cluster and exact guarded optional surfaces. The 207-entry historical test inventory remains intact; its HTTP constraint was restored in Task 6. There was no current `unsupported-etcd` candidate assertion to remove.

Verified

- Focused feature registry/native selection, profile/spec/shim tests, build, independent generation checks and profile revision checks pass under Go 1.25.14 and 1.26.9. The baseline owns the new facades and five replacement roots; Go 1.26 has no copied records or implementation tree. Independent origins remain distinct.
- Focused exact-compiler same-worker/fresh-worker etcd metatests pass. The artifact lookup rejected an older prebuilt identity after source changed, then passed after a source-matched build.
- `make lint`, `./.ci/gofmt check`, `git diff --check` and local documentation link checks pass. Existing TLS, crypto, reflection, signal and feature guards remain in place; no large, race or certification suite was enabled.
- `make test`: 87 reports, 2,013 passed, 20 skipped, no failures/errors; wall time 282 seconds. The report includes five positive etcd simulation tests and the native `TestEtcdReplayAfterInterveningRuns` plus protobuf replay metatest. The ordinary compiler was selected by Go tooling; Task 11 runs separate exact-compiler ordinary gates.

## Tasks 11–12 — exact-family acceptance and architecture audit

Date: 2026-10-21. Pair commit contains this entry and closes the plan.

Final executable source was commit `67eea50` before this evidence/documentation-only pair. Ordinary verification used isolated source snapshots and recorded these identities:

| Compiler | Input digest | Ordinary report | Pass | Fail | Error | Skip | Wall |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: |
| Go 1.25.14 | `b0cf247a2444b76305187d09a8455613a69b94a7cec8c69fd86a1d2b47e56363` | `.gosim/etcd-validation/go125/unit/latest/report.md` | 2,031 | 0 | 0 | 18 | 296.508s |
| Go 1.26.9 | `f222c32ab774eba919ae8480afec435a4fe56406dc0a552ad3e18435f432d4de` | `.gosim/etcd-validation/go126/unit/latest/report.md` | 2,047 | 0 | 0 | 16 | 321.942s |

Both `GOSIM_TOOLCHAIN=<compiler> TEST_REPORTS_DIR="$PWD/.gosim/etcd-validation/<family>" make test` commands exited 0 with 87 reports each. The reports and timelines identify actual positive `TestEtcd`, `TestSnapshotter`, `TestWal`, `TestNetTcpHttp`, `TestEtcdReplayAfterInterveningRuns` and `TestProtobufStatusReplayAfterInterveningRuns` execution, not skips. The Go 1.25 verification record notes unrelated checkout changes to `.gitignore` and two `.DS_Store` files outside its source snapshot; its snapshot inputs stayed unchanged and status is `passed`. The Go 1.26 record has no concurrent checkout changes and status `passed`.

On the same final executable source, `gosimtool test -features rt,net -seeds 1-3 -v -run '^TestNetTcp(Http|Grpc)$' ./internal/tests/net` passed 6/6 records per compiler. `gosimtool test -features rt,fs,net -seeds 1-3 -v ./examples/etcd` passed 15/15 records per compiler, including every cluster, snapshot and WAL seed. The ordinary native metatests additionally ran seeds 1, 2, 3, 1 in one worker and a fresh seed 1, with matching replay identity and seed-1 checksums on both families.

Existing example regression commands passed seeds 1–3 for `examples/simple` (`rt`), `examples/bolt` (`rt,fs`), `examples/network/machines` and `examples/network/latency` (`rt,net`), nested `examples/network/nats` (`rt,fs,net`) and nested `examples/network/grpc` (`rt,net`) on both exact compilers. The standalone `examples/filesystem` command passed with its documented Go 1.25.2 compiler. Individual command logs are under `.gosim/etcd-validation/examples/`.

Final Go 1.25.14 and Go 1.26.9 `make build` commands passed. Independent AMD64/ARM64 adapter checks, profile revision checks and focused spec/profile/feature tests passed under both compilers; logs are under `.gosim/etcd-validation/checks/`. `make lint`, `./.ci/gofmt check` and `git diff --check` passed before this documentation-only audit. Fatal guard fixtures for ECDSA generation, reflect2 raw pointers, active runtime tracing, host metrics and mutex profiling each exited 1 with the named `UnsupportedOperation` diagnostic under both compilers; no attempted recovery reached its success path. Ordinary feature applicability tests passed and exclude etcd before loading when `fs` or `net` is disabled.

Architecture audit: the launcher resolves the selected compiler and feature identity; the effective Go 1.25 baseline adds the five limited replacements, which Go 1.26 inherits without copied implementations. Composition imports the selected runtime packages; typed loading and translated fixture execution establish their reachability. Real etcd invokes embedded lifecycle, translated HTTP/gRPC/TCP and simulated filesystem storage; the ordinary registry includes the positive package only for `rt,fs,net`, and preparation builds matching native replay workers. No historical protobuf descriptor lifetime bug reproduced, so no production cache change or `ReplayRevision` increment was justified. Existing translation DB/cache schemas and provider contracts remain unchanged. The finite inventory document now describes the implemented state. All required gap entries in the plan are resolved or bounded by executed evidence.

No large suite, race suite, manual certification, Docker/Colima job or other-platform test was run. These passing local commands do not certify those tuples. The only source changes in this pair are documentation and plan/progress evidence; unrelated staged and working-tree user edits were left alone.
