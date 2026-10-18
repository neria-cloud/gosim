# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

Gosim is a simulation testing framework for Go. It runs Go code in a deterministic, simulated environment with fake network, filesystem, and multi-machine support. Tests run with `gosim test` instead of `go test`, but results look the same.

## Build and Test Commands

```bash
# Build the gosim CLI tool
mkdir -p ./.gosim && go build -o ./.gosim/gosimtool ./cmd/gosim

# Run all tests (format check + tests + race tests)
./test.sh

# Run tests directly with task runner
go run github.com/go-task/task/v3/cmd/task test

# Run gosim tests for specific packages
./.gosim/gosimtool test -v -run TestName ./internal/tests/behavior

# Run with multiple seeds to find rare failures
./.gosim/gosimtool test -seeds=1-20 -run TestName ./package

# Run standard go tests (non-sim packages)
go test -ldflags=-checklinkname=0 -tags=linkname ./...

# Debug a failing test at a specific step
./.gosim/gosimtool debug -package=./path/to/pkg -test=TestName -step=11

# Check formatting
./.ci/gofmt check

# Fix formatting
./.ci/gofmt fix
```

The task runner (`Taskfile.yml`) uses `./.gosim/gosimtool` as the CLI. The test packages under simulation are `./internal/tests/behavior` and `./nemesis`.

## Architecture: Three Layers

Gosim has a three-layer architecture (see `docs/design.md` for full details):

**Layer 1 — Deterministic Runtime (`gosimruntime/`)**
- Implements goroutines via coroutines (using `iter.Pull` API)
- Cooperative scheduling with a seeded PRNG for determinism
- Simulated time, deterministic maps, seeded randomness
- Each machine has isolated global variables via `G()` function

**Layer 2 — Standard Library Hooks (`internal/hooks/go123/`, `internal/hooks/go125/`)**
- Uses `//go:linkname` to redirect stdlib calls (e.g., `sync.Mutex`, `time.AfterFunc`) into `gosimruntime`
- Version-specific per Go release; currently go123 and go125 variants
- Requires `-ldflags=-checklinkname=0 -tags=linkname` when building

**Layer 3 — Simulated OS (`internal/simulation/`)**
- Linux syscall emulation (network, filesystem, fsync semantics)
- Machine isolation: crashes lose non-fsynced writes
- Machines communicate only through simulated syscalls

**Code Translation (`internal/translate/`)**
- AST-based: transforms user code before compilation
- Replaces `go func(){}` → `gosimruntime.Go()`, channel ops, map literals, etc.
- Caches results in `.gosim/translated/`
- Translations must be kept in sync with runtime API changes

## Key Packages

| Package | Purpose |
|---|---|
| `github.com/glycerine/gosim` (root) | Public API: `NewMachine()`, `IsSim()`, `SetConnected()`, `SetDelay()`, `SetSimulationTimeout()` |
| `gosimruntime/` | Core deterministic runtime (scheduler, channels, maps, time) |
| `cmd/gosim/` | CLI: `test`, `debug`, `translate`, `build-tests`, `prepare-selftest` |
| `metatesting/` | Run gosim tests from normal `go test` using pre-built binaries |
| `nemesis/` | Chaos scenarios: `PartitionMachines`, `RestartRandomly`, `Sleep`, `Repeat()`, `Sequence()` |
| `internal/translate/` | AST translator |
| `internal/simulation/` | Linux OS simulation |
| `internal/tests/` | Internal test suites (behavior, race) |

## Build Tags

- `//go:build sim` — included only in simulated (translated) builds
- `//go:build !sim` — excluded from simulated builds (metatests go here)
- `//go:build linkname` — stdlib hook files; used with `-tags=linkname`

## Test File Conventions

- `*_test.go` — normal Go tests
- Files with `//go:build sim` — gosim simulation tests
- Files with `//go:build !sim` — metatests (run with standard `go test`, invoke gosim internally)

## Important Implementation Notes

- **Determinism**: Same seed = same execution. Use `-seeds=N-M` to test multiple seeds.
- **Globals isolation**: Each simulated machine gets its own copy of globals; accessed via `G()` in the runtime.
- **Linkname**: `//go:linkname` hooks require `-ldflags=-checklinkname=0`; never skip this flag.
- **Translation cache**: `.gosim/` is gitignored. Delete it to force retranslation.
- **`go generate ./...`**: Regenerates syscall wrappers and other generated code.
- **Cross-arch tests**: Available in `.ci/crossarch-tests/` using Docker.