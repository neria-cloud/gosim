# Features

Gosim simulates the Go runtime, network, and filesystem. Builds select `rt`, `rt,fs`, `rt,net`, or `rt,fs,net`; optional services require their corresponding feature to be enabled.

The table covers the inventory in the [Gosim–turmoil feature report](reports/turmoil-featres-diff.md), with API names and statuses updated for this checkout. Package names below refer to Gosim packages, not standard-library packages; unsupported features are grouped under the related package. “Supported” describes the stated simulated behavior, not complete operating-system compatibility.

| Package | Feature name | Status | Comment |
| --- | --- | --- | --- |
| `cmd/gosim` | Go test runner integration | ✅ | `gosim test` translates and runs ordinary Go tests, with seed selection, race instrumentation, and simulation tracing. |
| `cmd/gosim` | Seed selection and replay | ✅ | The default seed is `1`; `-seeds` accepts individual seeds and ranges. Replay requires the same complete build identity and replay revision. |
| `cmd/gosim`, `rt` | Race detection | ✅ | `gosim test -race` integrates Go's race detector with simulated execution on supported toolchain/platform tuples. |
| `cmd/gosim`, `rt` | Logs and trace diagnostics | ✅ | Logs identify machines, goroutines, and steps. `-simtrace=syscall` traces syscalls; `-simtrace=network` records payload-free policy, fault, and packet scheduling metadata. |
| `cmd/gosim` | Step-based debugging | ✅ | `gosim debug` uses Delve to stop at a selected test, seed, and simulation step; headless debugging is available. |
| `gosim` | Multiple isolated machines | ✅ | `NewMachine` creates an unstarted machine; `Start` runs its program with isolated globals and the selected filesystem/network services. |
| `gosim` | Graceful stop, crash, and restart | ✅ | `Stop` flushes modeled writes; `Crash` retains a crash outcome. `Restart` creates a fresh epoch with new globals, and `SetMain` selects the next boot program. |
| `gosim`, `rt` | Manual simulation stepping | not supported | There is no public API to advance the simulation by one tick or scheduler step. Debugger step selection is a separate facility. |
| `gosim`, `rt` | Syscalls couter or statistics | not supported | TBD |
| `rt` | Virtual time and timeouts | ✅ | Simulated timers and sleeps use virtual time; the scheduler advances to the next timed event when blocked. `rt.SetTimeout` bounds simulation time. |
| `rt` | Deterministic scheduling and randomness | ✅ | A seeded cooperative scheduler controls goroutine selection. Translated map iteration and application randomness are deterministic; scheduler and provider random streams are separate. |
| `rt` | `sync.Map` | ✅ | Deterministic storage and scheduling, including pointer keys. `Range` uses a snapshot and allows callbacks to mutate the map. |
| `rt` | Source-level pause points and barriers | partially | Tests can coordinate simulated goroutines with Go channels. There is no dedicated public typed barrier API for suspend/observe reactions. |
| `rt` | Native hashing and cryptographic computation | partially | Reviewed hashes, checksums and ciphers use native Go computation. Ed25519 and CRC32 use native Go directly. CRC32 standard tables must remain read-only; Ed25519 key generation needs an explicit deterministic reader for replay (`nil` uses host randomness). Host-global hash registration remains rejected. |
| `metatesting` | Determinism and multi-seed checks | ✅ | Native tests run prebuilt simulation workers. `CheckDeterministic` compares repeated runs, and `CheckSeeds` exercises multiple seeds using recorded results and checksums. |
| `gosim`, `net` | Machine naming and address assignment | ✅ | Machines have labels; `net.ConfigureMachine` accepts an IPv4 address or requests deterministic allocation. `net.Address` exposes the assigned address after startup. |
| `net` | Hostname lookup and DNS | not supported | Machine labels do not provide a simulated DNS service or hostname/reverse-lookup API. |
| `net` | IPv4 | ✅ | Simulated machines communicate through IPv4 addresses and the translated socket path. |
| `net` | IPv6 | not supported | The simulated socket provider does not implement IPv6. |
| `net` | TCP streams and listeners | ✅ | Translated Go networking uses simulated stream sockets for listening, connecting, reading, writing, and closing connections. |
| `net` | UDP, broadcast, and multicast | not supported | Datagram sockets and UDP broadcast/multicast operations are outside the implemented socket model. |
| `net` | TCP packet-loss recovery | partially | Streams have acknowledgement and keepalive timers but no TCP retransmission. Only ACK progress renews an active five-second acknowledgement deadline; new writes cannot postpone failure detection. Dropped packets can break streams; repairing a link may require application reconnection. |
| `net` | Two-way and one-way partition/repair | ✅ | `Partition` and `Repair` accept a `Link` with `Both` or `Forward` direction. Explicit partitions remain in effect until repaired, independently of sampled loss. |
| `net` | Packet hold and release | not supported | There is no public control to hold in-flight packets and release them later. |
| `net` | Fixed and random latency | ✅ | Global and per-link latency policies support fixed delays or bounded exponential sampling with configurable minimum, maximum, and curve. |
| `net` | Failure and repair probabilities | ✅ | Global and per-link loss policies configure traffic-triggered failure and repair probabilities. These model link state transitions, not TCP retransmission. |
| `net` | Policy overrides and resets | ✅ | `SetLinkPolicy` copies each supplied latency/loss group; omitted groups retain their settings. `ResetLinkPolicy` restores global inheritance, and invalid values are rejected. |
| `net` | Ordered delivery and stale-packet rejection | ✅ | Delivery remains ordered per directed link. Epoch and connection-generation checks prevent old packets from reaching restarted endpoints or reused connections. |
| `net` | Arbitrary packet reordering | not supported | There is no public reordering policy; latency variation preserves the model's delivery order. |
| `net` | Packet inspection and custom rules | not supported | Network tracing is available, but there is no public in-flight packet iterator or packet rule/verdict installation API. |
| `net` | Network capacity controls | partially | TCPBufferSize configures per-direction capacity for new connections (zero selects 1 KiB; maximum 16 MiB). MTU, listen backlog, and ephemeral port ranges are not configurable. |
| `net` | HTTP dependencies | partially | Existing Go HTTP source can be translated for optional dependencies; this does not establish full HTTP or HTTPS compatibility. |
| `net` | Plaintext gRPC | ✅ | Unary and bidirectional binary streams with metadata authentication over simulated IPv4/TCP. |
| `net` | TLS and certificate operations | not supported | Optional TLS/X509 declarations can be imported, but executing their operations aborts without host I/O. |
| `fs` | Files, directories, and metadata | ✅ | Translated `os` calls support modeled open/read/write, directory operations, rename, and metadata. This does not imply complete POSIX syscall coverage. |
| `fs` | Sync and crash durability | ✅ | Sync makes modeled operations durable. `SetCrashMode` selects `DurableOnly` or `SamplePending` behavior for non-fsynced operations. |
| `fs` | Crash-state enumeration and restore | ✅ | `CrashStates` enumerates immutable recovery snapshots with an explicit state budget. `Restore` applies a retained snapshot before starting or restarting a target machine. |
| `fs` | Crash injection during sync | ✅ | `SetSyncFault` configures bounded crash probabilities around sync, including the probability of crashing before the sync completes. |
| `fs` | Directory sync | ✅ | The filesystem models directory operations and their synchronization for persistence testing. |
| `fs` | Direct-I/O and alignment controls | not supported | There is no public direct-I/O mode or alignment configuration. |
| `fs` | Storage capacity limits | ✅ | `fs.Config.CapacityBytes` sets a pre-start limit (default 128 MiB); `fs.Stats` reports total, used, and free space in allocated 1 KiB file-data blocks. Sparse holes use no blocks. Writes and reservations that need unavailable blocks fail atomically with `ENOSPC`, leaving file contents, size, and write offset unchanged. Capacity cannot be resized after start. |
| `fs` | Read-only storage mode | ✅ | `fs.SetReadOnly` can turn writes off and back on while a machine runs, including for already-open writable files. A write attempted while read-only returns `EROFS` without advancing its offset or changing data. The mode persists across restarts. |
| `fs` | Random storage fault injection | not supported | General random I/O errors, silent corruption, and injected short reads are not exposed as controls. Sync-specific crash injection is supported separately. |
| `fs` | Storage latency and page-cache tuning | partially | `fs.Config.Latency` adds a fixed simulated delay to data I/O and filesystem mutations; metadata queries remain immediate. Variable delays and page-cache tuning are not supported. |
| `fs` | Read-only memory mapping | partially | The implemented path supports shared, read-only file mappings from offset zero, up to 16 MiB. Writable, private, anonymous, and larger mappings are unsupported. |
| `fs` | File locking | not supported | Existing `flock` and record-lock stubs do not model ownership, contention, or exclusion. |
| `fs` | POSIX permissions | partially | Mode, UID, and GID metadata and owner/group/other access checks are modeled. The default identity is root; `fs.Config` can set a machine's effective UID/GID. Chmod requires the owner or root; chown requires root. Supplemental groups, ACLs, and special permission-bit behavior are not modeled. |
| `fs` | `io_uring` | not supported | There is no simulated ring API for asynchronous read, write, fsync, or cancellation. |
