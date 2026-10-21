# Restore etcd simulation: historical investigation

Date: 2026-10-21. Current branch: `feat/compact-go-versions`.
Historical tree: `ffd3a613542675755e4cbf8186b5edaf404ed95c` (2024-12-10).
Current implementation studied: `453947f`.

## Request and scope

The user recalls etcd support under Go 1.23 and wants it restored, preferably in
the Go 1.25 baseline so later families inherit it. Inspect the entire historical
tree, not only the README change in the named commit.

This investigation reads historical translator, hooks, reflection, scheduler,
filesystem/network support, examples, dependency versions, CI/test selection,
and relevant precursor/regression commits. It reproduces historical behavior in
an isolated archive and maps current admission and execution gaps. It does not
change current executable source or declare restored support.

## Finding

Etcd was a working simulation scenario in the requested tree. The current
unsupported designation represents lost application coverage. Its recovery
belongs primarily in shared implementation and baseline metadata, with genuine
Go-version differences expressed as cumulative deltas.

The historic success is specifically plaintext etcd 3.5.17 with a three-node
cluster and a network partition, plus snapshot/WAL round trips. It is not proof
of complete etcd, TLS, authentication, process-signal, locking or crash-durability
semantics.

## Reproduction

Archived the complete commit into `.gosim/etcd-history-ffd3a61/`; the active
checkout was not switched. Used native Darwin ARM64 and the exact historical
`go.mod` compiler, Go 1.23.2. No race, large suite, container, VM, cross-platform
execution or new certification was run.

```sh
GOTOOLCHAIN=go1.23.2 go build -ldflags=-checklinkname=0 -tags=linkname \
  -o .gosim/gosimtool ./cmd/gosim
GOTOOLCHAIN=go1.23.2 ./.gosim/gosimtool test -v -seeds=1 ./examples/etcd
GOTOOLCHAIN=go1.23.2 ./.gosim/gosimtool test -v -seeds=2 ./examples/etcd
GOTOOLCHAIN=go1.23.2 ./.gosim/gosimtool test -v -seeds=3 ./examples/etcd
```

These commands run from the archived tree, not the current checkout.

| Historical check | Actual result |
| --- | --- |
| Build | Passed, 16.694 seconds |
| Fresh process, seed 1 | All three tests passed; cluster 45/60 successful put/get attempts, 75 simulated seconds |
| Fresh process, seed 2 | All three tests passed; cluster 46/60, 74 simulated seconds |
| Fresh process, seed 3 | All three tests passed; cluster 47/60, 73 simulated seconds |
| Combined `-seeds=1-3` | Exit 1: cluster seed 1 passed; cluster seeds 2 and 3 panicked; all six snapshot/WAL executions passed |
| Historical `TestNetTcpHttp` and `TestNetTcpGrpc`, seeds 1–3 | All six passed in one invocation |

The combined-run panic is `mismatching field: google.rpc.Status.code` from
protobuf `internal/impl.(*MessageInfo).checkField`. Fresh-process successes make
retained cross-run state/descriptor identity a concrete hypothesis; the exact
responsible cache/global has not been isolated. Restoration must include a
multi-seed, single-process regression, not hide this by always launching a new
process. The historical translator also logged an unknown
`runtime/debug.runtime_setCrashFD` linkname while compiling; that warning did
not prevent the successful scenario executions and is not an approved modern
ABI strategy.

Evidence: `.gosim/etcd-history-evidence/` contains `build.log`, `etcd.log`,
`etcd-seed{1,2,3}-fresh.log`, `fresh-seeds.json`, `http-grpc.log`, `http-grpc.json`
and command timing/results JSON. Current rejection evidence is retained under
`.gosim/examples-verification/go1.{25.14,26.9}-etcd.log`.

## Where support lived in the old tree

| Responsibility | Historical implementation | Current counterpart |
| --- | --- | --- |
| Positive scenarios | `examples/etcd/{etcd_test,etcdmain_test,wal_test}.go` | Same scenario sources, adapted to explicit machine start and feature controls |
| Dependency translation | `internal/translate/main.go`: `collectImports`, `skippedPackagesGo123`, `keepAsmPackagesGo123` | `internal/translate/profile.go` plus cumulative profile policies |
| Stdlib/private hooks | `internal/translate/hooks_go123.go`, `internal/hooks/go123/` | `internal/compat/hooks/`, authored baseline declarations |
| Reflection/protobuf | `internal/reflect/`, translator global/type transformations and export hacks | `internal/compat/shims/reflect/`, current translator boundary/type logic |
| Simulated syscalls | `internal/simulation/os_linux.go`, generated wrappers and syscall transport | Shared compatibility adapters, contracts, kernel routes, FS/net providers |
| Scheduler, per-machine globals, randomness | `gosimruntime/` plus translated global access | `rt/runtime`, ABI, current translated global machinery |
| HTTP/gRPC tests | `internal/tests/behavior/net_test.go` | gRPC test active; historical HTTP test retained with `//go:build ignore` |

There was no special etcd adapter. The old translator walked dependency imports
and translated packages except a finite native skip list. It translated
`crypto/ecdsa`, `crypto/elliptic`, `crypto/rsa`, TLS/HTTP and application code,
with specific crypto assembly files copied into translated packages. Linux
`GetRandom` reached the simulator's deterministic random source.

It also permitted behavior that current boundaries deliberately restrict:

- `runtime`, metrics and profiling packages could remain native.
- `os/signal` registration hooks were no-ops and receiving blocked forever;
  this was not a functioning simulated signal service.
- `internal/reflect/no.go` contained panic-only private reflect2 declarations,
  explicitly described as allowing etcd to compile without supporting their use.
- `SysFlock` and `SysFcntlFlock` returned success without modeling contention.
- `mmap` was a bounded simulated file view, with `MAP_POPULATE` ignored;
  `madvise` returned success without modeling its effects.

These distinctions explain why copying the old allowlists is insufficient.

### Historical support commits already present in ffd3a61

- `057c053`: simulated mmap and supporting syscalls for bbolt.
- `985b55a`: translator fixes for etcd: named channels, implicit conversions,
  and invoking global function variables in goroutines.
- `9437bff`: legacy protobuf export workaround (`proto.enumsByName`).
- `f1bc358`: private reflect2 declarations that panic when called.
- `a00216b`: load dependency source without also loading unrelated broken tests.
- `a531e13`: bounded mmap and ignored `MAP_POPULATE`, explicitly for etcd.
- `54dddca`: positive etcd, snapshot, WAL and Bolt examples.

The historical and current root modules both pin etcd **3.5.17**, bbolt
**1.3.11**, gRPC **1.62.1**, and protobuf **1.33.0**. Dependency upgrading is not
required to recover the historical example. The newer standalone gRPC example
uses different versions and does not cover the complete etcd dependency graph.

## Where the current boundary changed

`6dff8c1` introduced the explicit Go 1.25 policy rejecting `crypto/...`, private
reflect2 and other unreviewed dependencies. `566acc1` performed the corresponding
Go 1.26 audit. `b7d5176` explicitly added an `unsupported-etcd` candidate assertion
expecting HTTP/gRPC rejection. These changes predate this compact-version work;
`324da1e` contains the consolidated compatibility/lifecycle work in current
history. `1c5068c` later restored selected plaintext gRPC coverage while retaining
the HTTP scenario under an ignored build constraint.

The present first error on both compilers is:

```text
examples/etcd -> etcd/server/v3/embed -> etcd/client/pkg/v3/transport
  -> crypto/ecdsa: unsupported application package
```

The existing etcd README's generic HTTP/gRPC wording is too broad to identify
this current failure. Plaintext gRPC is supported for the already tested cases.

Historical `Taskfile.yml` selected behavior/nemesis simulation suites, not the
etcd example. Native `go test` did not execute its `sim`-tagged scenarios. The
current positive feature registry also excludes etcd and instead retains
rejection coverage. Thus a passing ordinary suite or certification did not
establish continuing positive etcd execution.

## Current restoration gaps

Source-only `go list -deps -test -tags=sim` inspected the Linux/ARM64 dependency
graph under both current compilers: 565 records for Go 1.25 and 571 for Go 1.26.
A policy traversal stopped at native/replacement bridges and rejected packages;
this is a source inventory, not an execution result or full typed translation.

Five common rejected package boundaries are:

| Package | Caller / reason it is present | Recovery work |
| --- | --- | --- |
| `crypto/ecdsa` | etcd transport self-signed certificates and auth/JWT types | Review the precise API surface and separate dormant optional paths from actual operations; preserve type identity and deterministic entropy |
| `crypto/elliptic` | transport certificate generation (`P521`) and ECDSA dependencies | Review exact curve/type operations; do not grant general native execution |
| `crypto/rsa` | etcd auth and JWT signing/verification code | Review keys, verification and entropy-taking signing APIs individually |
| `github.com/modern-go/reflect2` | etcd v2 discovery/client JSON support | Establish a safe translated/reflection boundary or a compatible JSON path; old panic-only declarations do not implement reflect2 |
| `runtime/trace` | OpenTelemetry's `IsEnabled`, `NewTask`, `Task.End` path | Model disabled tracing explicitly and keep unimplemented active tracing rejected |

The raw Go 1.26 inventory also encounters `internal/runtime/syscall/linux` below
`syscall`. That is not demonstrated as a new etcd blocker: the real translator
replaces syscall bodies and prunes obsolete imports. The inventory does not
perform that transformation. See `policy-frontier.json` and the inspection
script in the evidence directory for this limitation.

Further concrete execution/type boundaries:

1. **Signal registration.** The existing example calls
   `osutil.HandleInterrupts`, which invokes `signal.Notify`. Today's signal shim
   aborts every operation. The embedded example should use explicit simulator
   machine lifecycle for shutdown, or a real modeled signal service must be
   implemented. Reintroducing silent registration no-ops would not restore
   signal semantics.
2. **HTTP raft traffic.** Actual HTTP serving, streaming, cancellation and
   connection reuse must work with translated net/context/time types. Restore
   the ignored HTTP regression and exercise etcd's peer transport; the working
   gRPC echo example alone does not cover this.
3. **Protobuf descriptor/global lifetime.** Exercise partition/error paths and
   several seeds in one process. Investigate the historical `Status.code`
   mismatch if it recurs; do not share arbitrary mutable native registries to
   bypass it.
4. **Observability.** Prometheus registers a Go collector which references
   `runtime.ReadMemStats` and other host metrics. Current adapters abort those
   calls. Trace and metric initialization must be checked separately from actual
   collection; a source reference alone does not prove the operation executes.
5. **Persistence.** Re-run snapshot/WAL and cluster storage scenarios against
   current providers. Historical success does not establish flock/fcntl
   contention correctness or stronger durability semantics.

Go-version differences are real even when public signatures match. Inspection
of the installed compiler sources found that Go 1.26 ECDSA/RSA GenerateKey
normally ignores the supplied reader and obtains secure randomness; Go 1.25
also explicitly does not promise deterministic output from a supplied reader.
Therefore forwarding those APIs to native crypto with a simulated reader is
not sufficient proof of replay determinism. The plaintext example does not
request generated certificates, so full TLS/key-generation support should not
be inferred from recovery of its scenario.

## Proposed restoration direction

1. Use the existing Go 1.25 baseline for common compatibility declarations,
   package policies and capability records. Put executable behavior in shared
   hooks/shims or the owning kernel/provider. Let Go 1.26 inherit that work;
   introduce a delta or bounded source variant only for an actual difference.
2. Recover the historical **plaintext three-node cluster + partition +
   snapshot + WAL** behavior on the current pinned dependencies first. Resolve
   each demonstrated compile/execution boundary with a concrete implementation
   or precise optional-operation guard. Do not broadly admit crypto/native I/O.
3. Keep the embedded fixture focused on simulator-managed lifecycle; define
   disabled observability behavior explicitly. Full process signals, TLS and
   private reflection are separate capabilities unless an executed path proves
   they are necessary for this target scenario.
4. Restore HTTP positive coverage, then snapshot/WAL and cluster coverage on
   both compiler families. Require seeds 1–3 in a single process and repeat a
   selected seed with the same complete identity. Assert usable cluster
   progress before/after partition, not merely the existing weak condition of
   one successful request anywhere in the test.
5. Replace the obsolete whole-etcd rejection expectation with positive feature
   applicability for `rt,fs,net`. Retain targeted negative tests for operations
   still unsupported. Update example/support documentation and independently
   regenerate/check metadata for both families.

This is the recovery scope supported by the investigation, not a completed
implementation specification. Full method-level changes depend on resolving
the actual typed boundary failures after each admission change.

## NOT CONSIDERED & TODO

- [porting:improvement-over-upstream] Cross-run protobuf isolation must be better
  than the reproduced historical combined-seed behavior. Exact root cause remains
  to be localized.
- [porting:deffered] Full TLS/certificate/JWT functionality is not demonstrated
  by the historical plaintext fixture. Preserve explicit rejection where still
  unimplemented; review any API admitted as computation separately.
- [porting:deffered] Full reflect2 unsafe map ABI and full process-signal service
  are not established by the historical success. Prefer real modeled behavior
  or narrow fixture integration over restoring successful stubs.
- [potential-improvement] Stronger lock-contention and crash-durability scenarios
  extend beyond the historical WAL/snapshot smoke tests.
- [postponed] Race/large suites and renewed certification have not been run for
  this study. Future execution follows the repository's explicit authorization
  requirements; historical reproduction is not current certification.
