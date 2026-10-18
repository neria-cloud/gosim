# Gosim and turmoil: testing architecture

## Scope

This compares the checked-out Gosim tree (`2e3c047`) with the checked-out turmoil tree (`4899894`). It describes implemented architecture, not a claim that every application or package works in simulation. Turmoil is a workspace containing the `turmoil` harness plus `turmoil-net`, `turmoil-fs`, and `turmoil-io-uring`; the harness still has its own network implementation, so the newer crates should not be assumed to be fully wired into its API.

## How each test runs

| Concern | Gosim | Turmoil |
| --- | --- | --- |
| Entry point | `gosim test` translates the test package and dependencies, compiles them with the ordinary Go compiler, and runs them under Gosim's runtime and simulated OS. | A Rust test constructs `Sim`, registers host future factories and client futures, then calls `run` or `step`. Code imports simulated socket and optional filesystem types explicitly. |
| Execution unit | Simulated goroutines are coroutines scheduled cooperatively by one process-wide Gosim scheduler. A simulated machine owns goroutines, copied package globals, a network stack, and a disk. | Each host/client has a paused, single-thread Tokio runtime and `LocalSet`; the outer `Sim` drives those runtimes sequentially. A host owns network state and optional filesystem state. |
| Time and order | When no goroutine can run, Gosim jumps to the next scheduled event. A seeded scheduler chooses runnable goroutines at yield points. | `Sim::step` advances the topology and each host runtime by a configured tick. Host iteration can be shuffled. Tokio task randomization requires `tokio_unstable`; the default seed is obtained from the OS unless configured. |
| I/O boundary | AST translation, Go standard-library hooks, and substituted Linux syscalls route ordinary Go APIs into the simulator. | Simulated `tokio::net`/`std::fs` style shims route selected operations into in-memory subsystems. The application or a test build must select those imports. |
| Failure boundary | `Crash`, `Stop`, and restart stop all simulated goroutines on a machine, rebuild its globals and network stack, and retain or partly flush its disk according to the operation. | `crash`/`bounce` cancel and rebuild a host runtime from its future factory; network and optional filesystem crash behavior are coordinated by `Sim`. |

Sources: Gosim [design](../design.md), [runtime](../../gosimruntime/runtime.go), [machine API](../../machine.go), [simulation](../../internal/simulation/simulation.go), and [translator configuration](../../internal/translate/main.go); turmoil [harness](https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/sim.rs), [host runtime](https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/rt.rs), [builder](https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/src/builder.rs), [network crate](https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil-net/README.md), and [Cargo features](https://github.com/tokio-rs/turmoil/blob/489989469b83d1fbe96c49847a30e57f098ecf22/crates/turmoil/Cargo.toml).

## The architectural trade-offs

### Integration with application code

Gosim aims for ordinary Go tests and application calls to keep using `go`, channels, `sync`, `time`, `net`, and `os`. The translator rewrites language constructs and package globals; low-level version-specific hooks connect the translated standard library to its scheduler; syscall substitution reaches the network and disk model. This gives a large potential integration surface with few test-only imports. It also makes the compatibility boundary hard: a package skipped by translation can retain real runtime behavior or produce incompatible types where translated and untranslated code meet. The current skip list explicitly says `net/http` simulation does not work in this tree, despite the design document's broader ecosystem goal. The Go 1.25/1.26 FIPS package changes are a concrete example of maintenance work caused by runtime and standard-library internals.

Turmoil's boundary is explicit. Tests use the `turmoil` harness and select its networking and optional filesystem APIs, or the separate crates' shims, in application code. This makes the simulated dependency visible and lets the network and filesystem components be used in smaller fixtures or a custom scheduler. It also means code that calls unshimmed OS or Tokio facilities can leave the simulated world. The main harness and `turmoil-net` currently provide distinct network stacks; the latter's composable packet rules should not be attributed automatically to `Sim`.

### Scheduling, virtual time, and reproducibility

Gosim controls Go-level scheduling and several language/runtime nondeterminism sources, including map iteration and `math/rand`, then advances time to the next event when work is blocked. This can explore concurrency interleavings efficiently and skip long idle periods. Its cooperative model observes transitions at instrumented yield points; native or untranslated activity is outside that guarantee. The runtime allows only one active Gosim run per process.

Turmoil reuses Tokio's paused time and task machinery instead of implementing an async runtime. One outer loop advances host runtimes by ticks, making host execution order and link delivery easy to inspect. Fixed ticks make elapsed work proportional to simulated time divided by tick size, even when little happens. Reproducibility requires supplying `rng_seed` when random features are used; Tokio's own seeded scheduler integration is conditional on `tokio_unstable`. Neither project should be described as controlling arbitrary real-world I/O merely because its simulated paths are deterministic.

### Host isolation and crash fidelity

Both projects represent multiple hosts inside one physical process. Gosim's translated package globals and per-machine OS resources approximate a Go process boundary, including reinitialization after restart. This broad boundary is useful for testing code with package-level state, but increases translation complexity and cannot isolate state in unconverted native code. Turmoil isolates async tasks with a Tokio runtime per host and recreates the host future on bounce. Its explicit factories simplify lifecycle control, but state stored outside the host runtime or simulated subsystems remains shared by the real test process. Neither model is equivalent to restarting a real OS process.

### Fault models and extensibility

Gosim's syscall-backed Linux model is suited to examining application behavior around TCP, filesystem persistence, and crash recovery through familiar Go APIs. Its fault surface is coupled to the supported syscall and translated-library paths. Turmoil's harness exposes link partitions, holds, latency, and failure rates; `turmoil-net` additionally exposes packet-level rule composition, while optional `turmoil-fs` models storage faults. These are useful injection seams, but their split across crates means users must choose which simulator and harness they are using. The detailed operation-by-operation comparison is in [the feature matrix](turmoil-featres-diff.md).

## Practical choice

Gosim is the closer fit when the target is Go code that relies on ordinary goroutines, package globals, and OS-facing APIs, provided its exact dependency graph translates and its required syscalls are modeled. Turmoil is the closer fit for Tokio applications willing to select simulated I/O types, especially when direct control over host stepping, links, or reusable network/filesystem components matters. Before adopting either, run a representative application test that exercises its actual dependency path: the architecture alone does not prove that a specific library stays inside the simulation.
