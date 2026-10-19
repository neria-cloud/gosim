# Repository guidance

Gosim is a deterministic Go simulation testing framework. It translates application code, then runs a native executable with simulated Linux syscalls. Use the actual package graph in [docs/design.md](docs/design.md) and the commands in [README.md](README.md).

## Build and validation

The supported Go families are stable `go1.25.*` and `go1.26.*` releases. Go tooling selects the compiler; exact compiler/source identity still separates build artifacts. Certification is an optional, explicitly invoked manual operation. It is not a CI step, an ordinary-test implementation detail, or a prerequisite for library use.

### Running tests and reading results

Use `make test` for ordinary local native and simulation tests. It runs in an
isolated source snapshot, uses the compiler resolved by Go tooling, and starts no Docker, Colima,
VMs, or cross-architecture jobs. For a single-test fix, run only the relevant
test unless broader validation is explicitly requested.

**Run `make test-race` or `make test-large` only when the user's instruction
explicitly includes an item to run and check that suite.** A general request to
test, fix tests, finish a task, or perform final verification does not authorize
either suite. This restriction also applies to aliases and equivalent commands
such as `make test-unit-race`, `./test.sh --race`, direct race commands, and direct
invocation of the large suites. Do not enable them through environment variables
or invoke a matrix to bypass this restriction.

| Command | Scope | Latest results |
| --- | --- | --- |
| `make test` | Ordinary local tests; expensive suites remain skipped | `test-reports/unit/latest/` |
| `make test-race` | Local race suite; requires explicit instruction | `test-reports/unit-race/latest/` |
| `make test-large` | Candidate feature sweep, selected-translation toolchain sweep, and CLI script fixtures; requires explicit instruction | `test-reports/large/latest/` |
| `make test-report-tools` | Reporting and orchestration unit checks | Terminal output |

Each reported run is preserved in `test-reports/<suite>/<run>/`; `latest` points
to the latest run for that suite. Read `report.md` for totals and failures,
`timeline.md` or `timeline.json` for timings and parallel execution, and the
reported per-package `.log` files for failure details. Runs also retain raw
`events.json`, per-package JSON and JUnit XML, `console.log`, and command logs.
The terminal prints the exact report directory and failing log paths. Check the
exit status and report for failures/errors; distinguish skips from passed tests.

Use `UNIT_PACKAGES` and `UNIT_PARALLEL` to set package and test concurrency
(currently 2 and 4), `GOSIM_TOOLCHAIN` for an optional exact compiler override, and
`TEST_REPORTS_DIR` to change the report root. For example:

```sh
make test UNIT_PACKAGES=2 UNIT_PARALLEL=4 GOSIM_TOOLCHAIN=go1.25.14
```

Local results apply to the startup snapshot. Concurrent checkout edits are
recorded in command logs and `.gosim/verification/<patch>/gate-*/verification.json`;
changes inside the tested snapshot fail verification. The standalone verification
entry also rejects changes to the working checkout.

### Lower-level commands

```sh
# Build an exact worker; retain linkname flags.
GOTOOLCHAIN=go1.25.14 go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim

# Complete ordinary checks in a private source-matched copy.
GOSIM_TOOLCHAIN=go1.25.14 ./.ci/verify-step.sh --ordinary
GOSIM_TOOLCHAIN=go1.26.9 ./.ci/verify-step.sh --ordinary

# Local race verification; only with explicit instruction to run and check race tests.
GOSIM_TOOLCHAIN=go1.25.14 ./test.sh --race
GOSIM_TOOLCHAIN=go1.26.9 ./test.sh --race

# Focused native checks.
GOTOOLCHAIN=go1.25.14 go test -ldflags=-checklinkname=0 -tags=linkname ./internal/kernel ./internal/contracts

# Focused simulation checks with a matching worker.
./.gosim/gosimtool internal-candidate test -go go1.25.14 -features rt,fs,net -seeds 1-20 -run TestName ./internal/tests/integration

# Formatting and orchestration checks.
./.ci/gofmt check
./.ci/gofmt fix
python3 -B -m unittest discover -s .ci/crossarch-tests -p '*_test.py'
```

The verification entry creates an isolated copy and verifies equal executable inputs without rewriting status/evidence metadata. It runs only the selected ordinary or race mode; no certification report is produced. Independent copies keep concurrent patches from overwriting selftest artifacts. Final verification does not override the explicit-instruction requirement for race and large suites.

Do not run bare native `go test ./...` against stale or missing prebuilt simulation workers. Metatesting requires matching patch/features/race manifests and a matching CLI. `build-tests` and `prepare-selftest` use the same explicit build selection; the verification entry handles preparation.

## Architecture

| Location | Responsibility |
| --- | --- |
| Root `gosim` package | Unstarted machine creation and lifecycle controls |
| `fs`, `net`, `rt` | Public feature-specific controls and native ErrNotSim behavior |
| `rt/runtime`, `rt/runtime/abi` | Untranslated deterministic engine and primitive memory boundary |
| `internal/contracts` | Translated lifecycle, service, descriptor and port interfaces; imports ABI only |
| `internal/control`, `internal/kernel` | Translated transport, lifecycle transactions, dispatcher, descriptors and polling |
| `internal/fs`, `internal/fs/model` | Filesystem provider and storage algorithms |
| `internal/net`, `internal/net/model` | Network provider and TCP/fault algorithms |
| `internal/compat/go125`, `internal/compat/go126` | Version-owned hooks, shims, host ABI and profile metadata |
| `internal/hostabi`, `internal/compat/selected` | Compile-time selection; unsupported families fail |
| `internal/compose`, `internal/translate` | Selected composition before package loading, then language translation |
| `internal/buildspec`, `internal/buildcfg`, `internal/buildrun`, `internal/gosimtool` | Host build identities, pinned compilers, cache and artifact handling |
| `internal/tests/{rt,fs,net,integration}`, `nemesis`, `examples` | Applicable positive suites; full integration needs all features |

The four valid immutable feature sets are `rt`, `rt,fs`, `rt,net`, `rt,fs,net`. Kernel/contracts/public controls cannot import concrete providers; providers cannot import each other. The old `gosimruntime` import path, monolithic `internal/simulation`, old hook trees and shared compatibility shims have been removed.

## Implementation invariants

- Preserve determinism for identical complete identity/replay revision/seed. Run IDs are stale-handle guards, not randomness or trace inputs. Domain random streams are independent.
- `NewMachine` does not start init. Configure optional services, then `Start`; `Restart` follows a completed epoch. Wait observes the epoch captured at entry.
- Lifecycle preparation is staged and non-yielding. Abort cannot flush disks, publish packets or consume another provider's stream. Commit publishes before init; teardown continues through cleanup errors.
- Memory views are borrowed for an invocation only. Cancellation/completion clears them. Poll and packet callbacks carry epoch/generation checks.
- Disabled features abort the simulation even if application code ignores errors or recovers. Do not add host-I/O fallbacks or generic successful stubs.
- Keep Go-sensitive code in its selected family or document and verify a stable shared helper. Never select executable family code through runtime switches or manual release tags.
- Preserve `-ldflags=-checklinkname=0 -tags=linkname` on host builds/tests. Simulated source is loaded with Linux source ABI and application cgo disabled; native host race/SQLite tooling may use cgo.
- Source/profile/config changes invalidate artifacts and certification. Do not promote skipped, compile-only or stale results. Status/evidence metadata alone is excluded from executable digests.

## Generation and manual certification

Use `cmd/gosim-gen`, not a retired syscall generator:

```sh
GOTOOLCHAIN=go1.25.14 go run ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=. -check
GOTOOLCHAIN=go1.26.9 go run ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=. -check
./.ci/crossarch-tests/run-matrix.sh --platform all --dry-run
```

Omit `-check` only when intentionally regenerating adapters. Run `go run ./internal/compat/genprofile` when version-owned source/facade inputs change. The filesystem model's separate stringer generation remains supported.

Explicit manual certification currently defaults to 16 tuples: 2 exact patches × 2 ARM64 host tuples (Linux and Darwin) × 4 feature sets, without race instrumentation. CI runs ordinary native tests and publishes `test-reports`; it never invokes certification. AMD64 and race certification remain disabled in the manual matrix runner and certification workers/commands. Ordinary validation and explicitly requested race tests remain available; they do not certify these excluded tuples. Use `run-matrix.sh --native --platform OS/arm64` on an actual matching native worker. The Colima route uses `--context colima --platform linux/arm64` from Darwin ARM64. Preserve observed architecture and image metadata. The runner copies artifacts through Docker's API and does not recreate the VM. Historical AMD64/race evidence does not authorize new certification jobs.

## Test conventions

`//go:build sim` selects translated-only tests; `!sim` selects native/metatests. `linkname` is the approved host-hook build tag. Feature applicability is defined in `internal/tests/features`; exclude disabled models before package loading rather than hiding them behind test-name filters.

The `.gosim` directory is ignored and contains generated/candidate/certified artifacts and validation evidence. Artifact directories include both identity and package-content digests. Do not add old-format cache decoding, compatibility forwarding packages, or temporary retirement tags.
