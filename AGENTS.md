# Repository guidance

Gosim is a deterministic Go simulation testing framework. It translates application code, then runs a native executable with simulated Linux syscalls. Use the actual package graph in [docs/design.md](docs/design.md) and the commands in [README.md](README.md).

## Build and validation

The exact initial toolchain targets are `go1.25.14` and `go1.26.9`. The module's Go 1.25 language minimum does not authorize other patches. Normal CLI commands require matching certified tuple/source evidence; internal candidate commands require declared pending tuples.

```sh
# Build an exact worker; retain linkname flags.
GOTOOLCHAIN=go1.25.14 go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim

# Complete ordinary checks in a private source-matched candidate copy.
GOSIM_TOOLCHAIN=go1.25.14 ./.ci/verify-step.sh --ordinary
GOSIM_TOOLCHAIN=go1.26.9 ./.ci/verify-step.sh --ordinary

# Complete ordinary/race verification; use for final verification.
GOSIM_TOOLCHAIN=go1.25.14 ./test.sh
GOSIM_TOOLCHAIN=go1.26.9 ./test.sh

# Focused native checks.
GOTOOLCHAIN=go1.25.14 go test -ldflags=-checklinkname=0 -tags=linkname ./internal/kernel ./internal/contracts

# Focused simulation checks on a declared pending checkout.
./.gosim/gosimtool internal-candidate test -go go1.25.14 -features rt,fs,net -seeds 1-20 -run TestName ./internal/tests/integration

# Formatting and orchestration checks.
./.ci/gofmt check
./.ci/gofmt fix
python3 -B -m unittest discover -s .ci/crossarch-tests -p '*_test.py'
```

The verification entry creates an isolated copy, verifies equal executable inputs and resets only the copy's excluded status/evidence metadata to pending. It leaves the original profiles unchanged and does not promote results. Independent copies keep concurrent patches from overwriting selftest artifacts. During the modularization plan, full race suites are deferred to final verification by user instruction.

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

## Generation and final matrix

Use `cmd/gosim-gen`, not a retired syscall generator:

```sh
GOTOOLCHAIN=go1.25.14 go run ./cmd/gosim-gen -toolchain=go1.25.14 -profile=go125 -arches=amd64,arm64 -output=. -check
GOTOOLCHAIN=go1.26.9 go run ./cmd/gosim-gen -toolchain=go1.26.9 -profile=go126 -arches=amd64,arm64 -output=. -check
./.ci/crossarch-tests/run-matrix.sh --platform all --dry-run
```

Omit `-check` only when intentionally regenerating adapters. Run `go run ./internal/compat/genprofile` when version-owned source/facade inputs change. The filesystem model's separate stringer generation remains supported.

Final certification requires 2 exact patches × 3 host tuples × 4 feature sets × 2 race modes. Use `run-matrix.sh --native --platform OS/ARCH` on an actual matching native worker. Colima routes use `--context colima --platform linux/amd64` or `linux/arm64` from Darwin ARM64. Linux AMD64 must not silently run ARM64 jobs. Preserve observed architecture/emulation and image metadata. The runner copies artifacts through Docker's API and does not recreate the VM.

## Test conventions

`//go:build sim` selects translated-only tests; `!sim` selects native/metatests. `linkname` is the approved host-hook build tag. Feature applicability is defined in `internal/tests/features`; exclude disabled models before package loading rather than hiding them behind test-name filters.

The `.gosim` directory is ignored and contains generated/candidate/certified artifacts and validation evidence. Artifact directories include both identity and package-content digests. Do not add old-format cache decoding, compatibility forwarding packages, or temporary retirement tags.
