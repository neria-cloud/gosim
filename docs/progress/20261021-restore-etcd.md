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
