# Gosim and turmoil: feature matrix

## Scope and reading guide

This matrix compares Gosim at `2e3c047` with turmoil at `4899894`, using the checked-out implementation and public APIs. **Main Sim** means the `turmoil` crate's `Builder`/`Sim` harness. **Separate crate** means a capability in `turmoil-net`, `turmoil-fs`, or `turmoil-io-uring`; that capability is not necessarily available through Main Sim. “Not exposed” means no corresponding public operation was found in the inspected API, rather than proof that no workaround exists. Feature flags and shims must be enabled or imported where stated.

## Test execution and control

| Feature | Gosim | Turmoil |
| --- | --- | --- |
| Normal test runner integration | `gosim test` runs translated Go tests, with `-seeds`, `-race`, and `-simtrace` flags. [G-test] | Rust tests construct `Builder`/`Sim` and call `run`; no dedicated test CLI in Main Sim. [T-lib] [T-sim] |
| Multi-host test program | `NewMachine` starts a Go function with its own globals, disk, and network stack. [G-machine] | `host` registers a restartable future factory; `client` registers test futures. [T-sim] |
| Manual progress | `Yield` gives another simulated goroutine a chance to run. No public one-step simulation API found. [G-sim] | `Sim::step` advances one configured tick; `Sim::run` drives until clients finish. [T-sim] |
| Virtual time | Scheduler jumps to the next timed event when blocked; a simulation timeout can be set. [G-runtime] [G-sim] | Paused Tokio time advances by `tick_duration`; `simulation_duration` limits the run, and epoch is configurable. [T-builder] [T-rt] |
| Repeatable seed | Default test seed is `1`; CLI accepts individual seeds and ranges. [G-test] | `rng_seed` makes turmoil's random choices repeatable; default seed comes from the OS. Tokio task seeding requires `tokio_unstable`. [T-builder] [T-rt] |
| Scheduling variation | Seeded cooperative goroutine selection; translated map iteration and `math/rand` use simulator-controlled randomness. [G-runtime] [G-design] | Optional random host order. Tokio runtime scheduler seeding is conditional on `tokio_unstable`; no Go-like map transformation. [T-builder] [T-rt] |
| Determinism checking | Event checksum/trace and metatesting helpers `CheckDeterministic` and `CheckSeeds`. [G-meta] [G-test] | Seeded reruns and `tracing` output; no comparable automatic checksum/metatesting API found in Main Sim. [T-builder] [T-lib] |
| Race detector | `gosim test -race` builds with Go's race detector hooks. [G-test] | No built-in race detector in Main Sim; Rust's normal compiler and ecosystem checks remain available outside this API. [T-cargo] |
| Logs and packet diagnostics | `-simtrace`, simulator logs, and trace checksums. [G-test] [G-runtime] | `tracing` events; `turmoil=trace` emits packet-level trace. Main Sim exposes in-flight messages via `links`. [T-lib] [T-sim] |

## Machines, network, and faults

| Feature | Gosim | Turmoil |
| --- | --- | --- |
| Crash and restart | `Crash`, graceful `Stop`, `Restart`, and `RestartWithPartialDisk`; new package globals and network stack on restart. [G-machine] [G-simulation] | Main Sim `crash`/`bounce` cancels and recreates host futures/runtime. Optional filesystem participates in crash. [T-sim] [T-rt] |
| IPv4 and IPv6 | Machine API says IPv4 only; simulated `SysSocket` accepts `AF_INET` stream sockets only. [G-machine] [G-socket] | Main Sim `Builder::ip_version` supports IP version selection; UDP API includes IPv4 and IPv6 multicast methods. [T-builder] [T-udp] |
| TCP streams/listeners | Simulated `SOCK_STREAM` socket stack behind Go's translated networking path. [G-socket] [G-net] | Main Sim supplies `TcpStream` and `TcpListener` shims. `turmoil-net` has a separate TCP kernel. [T-net] [T-netcrate] |
| UDP datagrams | Not supported by the inspected Linux socket path: `SysSocket` rejects `SOCK_DGRAM`. [G-socket] | Main Sim supplies `UdpSocket`, including send/receive and broadcast/multicast methods. [T-udp] |
| TCP packet loss recovery | Gosim models a stream with ack/keepalive timers; exact TCP fidelity should be checked for the workload. [G-net] | Main Sim's `fail_rate` documentation says dropped TCP messages break connections because it has no retransmission. The separate `turmoil-net` crate models retransmission. [T-builder] [T-netcrate] |
| Host naming / resolution | Machine labels become hostnames; addresses can be assigned or allocated. [G-machine] | Main Sim has hostname lookup/reverse lookup; `turmoil-net` has an in-memory DNS table. [T-sim] [T-netcrate] |
| Partition and repair | `SetConnected` toggles connectivity for a host pair. [G-sim] [G-net] | Main Sim has two-way and one-way partition/repair, plus hold/release of in-flight messages. [T-sim] |
| Latency and loss | `SetDelay` sets a host-pair delay. No public probabilistic loss control found. [G-sim] | Main Sim sets global/per-link latency, latency curve, failure and repair rates; builder supplies initial values. [T-sim] [T-builder] |
| Packet inspection and custom rules | No public packet iterator or rule installation API found; network implementation is internal. [G-net] | Main Sim `links` inspects and can deliver held packets. Separate `turmoil-net` supports packet `Rule`/`Verdict` chains and reusable latency rules. [T-sim] [T-netcrate] |
| Network capacities | Internal stream buffers exist; no public capacity setting found. [G-net] | Main Sim configures TCP/UDP capacity and ephemeral port range. Separate `turmoil-net` configures MTU, send/receive buffers, backlog, retransmission thresholds. [T-builder] [T-netcrate] |

## Filesystem and fault injection

| Feature | Gosim | Turmoil |
| --- | --- | --- |
| Simulated file API | Linux syscall replacement backs ordinary translated `os` file operations; supports open/read/write, directories, rename, and metadata in its filesystem. [G-design] [G-fs] | With `unstable-fs`, shims mirror parts of `std::fs`, `tokio::fs`, and Unix file traits; application imports must select them. [T-cargo] [T-fs] |
| Durability and crash recovery | Pending operations become durable on sync/flush; crash preserves durable data. `IterDiskCrashStates` enumerates possible partial crash states. [G-fs] [G-machine] | `turmoil-fs` retains synced data, discards or partly persists pending writes on crash, with configurable sync probability and block size. No exhaustive crash-state iterator found. [T-fs] |
| Crash during sync | `SetSometimesCrashOnSync` injects a crash around `fsync`. [G-machine] | No equivalent sync-specific host-crash switch found; `turmoil-fs` can inject sync persistence and I/O faults, while the test can call `Sim::crash`. [T-fs] [T-sim] |
| Directory sync and direct I/O | Filesystem models directory operations and sync; no public direct-I/O configuration found. [G-fs] | `turmoil-fs` exposes `sync_dir`, `direct_io`, and configurable alignment. [T-fs-shim] [T-fs] |
| Capacity and random storage faults | No public capacity, random I/O error, corruption, or short-read controls found. [G-machine] [G-fs] | Optional `turmoil-fs` configures capacity, I/O errors, silent read corruption, short reads, latency, page cache, and direct-I/O alignment. [T-fs] |
| Read-only memory mapping | Internal filesystem has `Mmap`/`Munmap`; scope is limited to its implemented syscall path. [G-fs] | No mmap shim found in the inspected `turmoil-fs` public file API. [T-fs-shim] |
| `io_uring` | No simulated `io_uring` API found. [G-design] | Optional `unstable-io_uring` exposes a separate simulated ring with read, write, fsync, and cancel operations. [T-cargo] [T-uring] |
| Source-level pause points | `Yield` is available, and tests can coordinate through Go channels; no dedicated typed barrier API found. [G-sim] | Optional `unstable-barriers` supplies typed `trigger`, `Barrier`, and suspend/observe reactions. [T-cargo] [T-barriers] |

## Integration limits that affect feature claims

Gosim's design intends broad Go standard-library compatibility, but the current translator skip list explicitly states that `net/http` simulation does not work in this tree. Its crypto/TLS and HTTP skip decisions also make translated/untranslated type boundaries relevant to any application dependency graph. Network claims above describe the implemented simulated socket path, not a guarantee that a particular high-level package reaches it. [G-translate]

Turmoil's Main Sim and `turmoil-net` are distinct network implementations. The Main Sim TCP loss behavior and the separate crate's retransmission model are therefore both recorded in the table. The filesystem and `io_uring` features are opt-in Cargo features; `turmoil-fs` and `turmoil-net` can also be used as standalone components. Calls through real socket/file types are outside these simulated models. [T-cargo] [T-netcrate] [T-fs]

## Source index

[G-design]: ../design.md
[G-machine]: ../../machine.go
[G-sim]: ../../simulation.go
[G-runtime]: ../../gosimruntime/runtime.go
[G-test]: ../../gosimruntime/testmain.go
[G-meta]: ../../metatesting/metatest.go
[G-simulation]: ../../internal/simulation/simulation.go
[G-socket]: ../../internal/simulation/os_linux.go
[G-net]: ../../internal/simulation/network/stack.go
[G-fs]: ../../internal/simulation/fs/filesystem.go
[G-translate]: ../../internal/translate/main.go

[T-lib]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/lib.rs
[T-cargo]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/Cargo.toml
[T-builder]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/builder.rs
[T-sim]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/sim.rs
[T-rt]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/rt.rs
[T-net]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/net/mod.rs
[T-udp]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/net/udp.rs
[T-netcrate]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil-net/README.md
[T-fs]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil-fs/src/lib.rs
[T-fs-shim]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil-fs/src/shim/std/fs/mod.rs
[T-uring]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil-io-uring/src/opcode.rs
[T-barriers]: https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/barriers.rs
