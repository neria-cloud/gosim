# Gosim syscall support for a future gquorum integration

**Audit date:** 2026-10-18  
**Gosim revision:** `e6122b1`  
**Comparison:** gquorum's Linux/ARM64, Go 1.26.5 syscall capture (`docs/reports/gquorum-syscalls.md` in the supplied gquorum worktree)

## Executive assessment

Gosim has a useful simulated **file, crash, and IPv4 TCP** syscall layer, but the traced gquorum cache-server workload cannot yet be treated as a fully simulated integration test. The most consequential gaps are:

1. **gRPC and HTTP are outside the translated type universe.** Gosim deliberately skips `net/http`, `crypto/tls`, and `google.golang.org/grpc`; its HTTP/gRPC behavior tests are excluded with `//go:build ignore`. A gquorum test using its current gRPC transport will need translation support or a transport adapter.
2. **File locking is not modeled.** `flock` and the pointer-form `fcntl` lock wrapper return success without acquiring a lock. The integer-form `fcntl` returns `ENOSYS`. gquorum and its etcd WAL/bbolt dependencies use both lock families. Tests could otherwise claim that two writers safely share a WAL or database when they do not.
3. **File permissions are not modeled.** There is no `fchmod`/`fchmodat` implementation. gquorum explicitly calls `File.Chmod` for identity files and `os.Chmod` for snapshots. These paths may fail during initialization or snapshot publication.
4. **bbolt mappings have a 16 MiB ceiling.** Gosim supports read-only shared file mappings only at offset zero and rejects larger lengths. Small bbolt databases may work; larger ones fail when remapped.
5. **Socket support is narrow.** Only nonblocking IPv4 TCP streams are created. UDP DNS, IPv6, MPTCP probes, real socket options, and real epoll descriptors are not simulated. Current `getsockname` reports a placeholder address for listeners.

The gquorum capture observed **57 distinct syscall names** across three server processes. That is a measured workload inventory, not a universal list of gquorum requirements. Many names belong to the **native Go runtime**, which Gosim does not translate. They must not be scored as missing virtual OS syscalls merely because no `LinuxOS.Sys*` method exists.

Of the 57 observed names, this audit classifies **14 as substantively simulated, 16 as limited, 3 as success-returning stubs, 4 as missing, 18 as host-runtime calls, 1 (`fcntl`) as mixed limited/stub, and 1 as the launch boundary**. These counts describe syscall names, not call volume or readiness of the complete gquorum stack.

## How to read the inventory

Status describes the **simulated application path**, not whether a syscall can appear in a host `strace` of a Gosim test binary.

| Status | Meaning |
| --- | --- |
| **Simulated** | A translated call routes to Gosim's `LinuxOS` and has substantive behavior for the observed use. It may still have ordinary implementation limits noted below. |
| **Limited** | A route exists, but accepted arguments, returned values, or semantics differ materially from Linux or the gquorum use. |
| **Stub** | A route returns success without the effect that callers rely on. This is more dangerous for testing than an explicit error. |
| **Missing** | No simulated implementation for the observed call, or the routed implementation returns `ENOSYS`. A translated raw call typically reaches `RawSyscall6` and returns `ENOSYS`; it may panic if it bypasses the expected wrapper for a syscall Gosim considers handled. |
| **Host runtime** | The trace call is attributable to native Go runtime/process mechanics. Gosim's cooperative scheduler or host Go runtime handles that concern; there is no virtual Linux syscall contract for it. |
| **Launch boundary** | The tracer's `execve` of the server, outside the running library. |

The generated Linux syscall interface lists **36 `Sys*` methods plus `PollOpen`/`PollClose`** on ARM64; `SysFcntlFlock` is manually wrapped. A route alone does **not** establish correct semantics. See [the generated interface](../../internal/simulation/linux_gen_arm64.go), [the implementation](../../internal/simulation/os_linux.go), [the raw fallback](../../internal/simulation/os_linux.go#L165), and [the stdlib hook](../../internal/hooks/go123/syscall.go). The generated AMD64 route set is analogous but includes `newfstatat` rather than ARM64's `fstatat` name.

## Every syscall observed in the gquorum capture

Names follow the Linux/ARM64 trace. Go APIs can use different kernel names on AMD64 or avoid an observable syscall through vDSO or runtime code. The original report has arguments, counts, errno, and phase evidence; this table evaluates Gosim behavior for each name.

### Storage and file descriptors (26)

| Observed syscall | Status | Gosim behavior and gquorum implication |
| --- | --- | --- |
| `openat` | **Limited** | Virtual files/directories. Accepts read/write/create/exclusive/truncate/directory, ignores `O_CLOEXEC`; rejects other flags such as `O_APPEND`. Mode and permission checks are TODOs. WAL/database opens with the observed basic flags are plausible. |
| `close` | **Simulated** | Closes virtual file or TCP socket descriptors. |
| `read` | **Simulated** | Reads virtual files and TCP streams. Trace reads from DNS UDP or runtime eventfd are outside this support. |
| `write` | **Simulated** | Writes virtual files and TCP streams; stdout/stderr have special logging behavior. Trace writes to runtime eventfd are host runtime operations. |
| `pread64` | **Simulated** | Positional read on a virtual file. |
| `pwrite64` | **Simulated** | Positional write on a virtual file; relevant to bbolt page writes. |
| `lseek` | **Simulated** | Virtual file offset changes; relevant to etcd WAL segment checks. |
| `fdatasync` | **Limited** | Calls the same `SysFsync` implementation, so data-only versus full-sync distinctions are absent. Both create a simulated durability boundary; crash-on-sync injection also applies. |
| `fsync` | **Simulated** | Calls `filesystem.Sync` on a virtual file inode, including directories. Crash tests cover file and directory sync ordering. |
| `fallocate` | **Limited** | Supports mode 0, offset 0 only; extends the virtual file length with zeros rather than modeling reserved capacity separately. gquorum's traced 64 MiB WAL preallocation has those arguments, but Linux allocation semantics differ. |
| `ftruncate` | **Simulated** | Resizes a writable virtual file. |
| `flock` | **Stub** | Always succeeds and holds no lock. gquorum WAL refill explicitly calls `LOCK_EX|LOCK_NB` and `LOCK_UN`; contention behavior cannot be tested. |
| `fstat` | **Limited** | Reports a virtual file's type and size. `fillStat` does not populate Linux metadata such as permissions; non-file descriptors fail. |
| `newfstatat` | **Limited** | AMD64 name for the virtual path-stat implementation (`fstatat` on ARM64). Only flags 0 are accepted; type and size are populated, permissions are not. |
| `getdents64` | **Simulated** | Enumerates virtual directory entries; useful for WAL/snapshot discovery. |
| `mkdirat` | **Limited** | Creates virtual directories; permission mode is not faithfully enforced. |
| `fchmod` | **Missing** | No generated route or `LinuxOS` method. gquorum's `File.Chmod` on identity files needs this path. |
| `fchmodat` | **Missing** | No generated route or `LinuxOS` method. gquorum's snapshot `os.Chmod` needs this path. |
| `renameat` | **Simulated** | Renames virtual paths; crash tests cover rename plus file/directory sync ordering. |
| `unlinkat` | **Simulated** | Removes virtual files or directories with the supported flags. |
| `fcntl` | **Limited / stub** | Integer-form `SysFcntl` returns `ENOSYS`; `internal/syscall/unix.fcntl` also returns `ENOSYS`. The separate pointer-form `SysFcntlFlock` returns success without a lock. Therefore descriptor flag operations and etcd `F_OFD_*` locks are not faithfully supported. |
| `readlinkat` | **Missing** | No virtual implementation; the observed `/proc/self/exe` lookup has no simulated procfs target. |
| `statfs` | **Missing** | No virtual implementation; the observed `/proc` check from Prometheus procfs cannot inspect a simulated proc filesystem. |
| `mmap` | **Limited** | Supports file-backed `PROT_READ|MAP_SHARED` at offset 0, up to 16 MiB; `MAP_POPULATE` is ignored. bbolt's mapped pages fit only while its requested map length stays within that ceiling. Anonymous runtime mappings are host runtime activity. |
| `munmap` | **Simulated** | Releases Gosim-created file mappings by starting address; host anonymous mapping cleanup remains native. |
| `madvise` | **Stub** | Always succeeds without applying a hint. `MADV_RANDOM` for bbolt access pattern can be ignored for correctness, but behavior/performance claims cannot use this result. |

### Networking (13)

| Observed syscall | Status | Gosim behavior and gquorum implication |
| --- | --- | --- |
| `socket` | **Limited** | Accepts exactly `AF_INET`, `SOCK_STREAM|SOCK_CLOEXEC|SOCK_NONBLOCK`, protocol 0. No UDP DNS, IPv6, or MPTCP socket. The traced MPTCP/IPv6 probes may fall back, but DNS through UDP cannot use Gosim's stack. |
| `bind` | **Limited** | Binds a virtual IPv4 TCP listener; other families and protocols are unavailable. |
| `listen` | **Simulated** | Enables a virtual TCP listener. |
| `accept4` | **Limited** | Accepts a virtual TCP stream and participates in Gosim's virtual poller, but requires exactly `SOCK_CLOEXEC|SOCK_NONBLOCK`. |
| `connect` | **Limited** | Connects a virtual IPv4 TCP stream to a simulated machine/port. Docker DNS addresses and host services are not virtual peers. |
| `getsockname` | **Limited** | Connected streams report their source, but listeners currently report `0.0.0.0:1` as a placeholder, not their actual bound address. |
| `getpeername` | **Simulated** | Reports a connected virtual stream's peer. |
| `setsockopt` | **Stub** | Always returns success, without implementing keepalive, `TCP_NODELAY`, `TCP_USER_TIMEOUT`, etc. Option-dependent behavior is not testable. |
| `getsockopt` | **Limited** | Only `SOL_SOCKET/SO_ERROR` on connected streams is implemented. Traced `SO_LINGER`, timeout options, and `TCP_INFO` return `ENOSYS`. |
| `epoll_create1` | **Host runtime** | Native Go uses epoll; Gosim uses its `internal/poll` hooks and `PollOpen`/`PollClose` rather than a virtual epoll descriptor. A translated direct epoll user is unsupported. |
| `epoll_ctl` | **Host runtime** | Same boundary: Gosim's virtual poller substitutes for Go's internal poll operations, but direct epoll control is unsupported. |
| `epoll_pwait` | **Host runtime** | Same boundary: simulated network wait is cooperative, while the test process's native Go runtime may still call epoll on host descriptors. |
| `eventfd2` | **Host runtime** | The native Go netpoll wakeup fd is a host mechanism; Gosim does not expose a simulated eventfd to application code. |

### Go runtime, threads, signals, and process control (17)

| Observed syscall | Status | Gosim behavior and gquorum implication |
| --- | --- | --- |
| `clone` | **Host runtime** | Native Go creates OS threads for the test binary; simulated goroutines use Gosim coroutines. No simulated fork/thread-creation syscall. |
| `futex` | **Host runtime** | Native runtime parks/wakes host threads; translated Go `sync` primitives are redirected to Gosim scheduling hooks. Direct futex use is unsupported. |
| `nanosleep` | **Host runtime** | Trace's short runtime sleeps are host mechanics; translated `time.Sleep` uses simulated time. Direct nanosleep use is unsupported. |
| `sched_yield` | **Host runtime** | Host runtime scheduling; translated goroutine scheduling is cooperative. |
| `sched_getaffinity` | **Host runtime** | CPU discovery of the native process, not a simulated machine property. |
| `getpid` | **Limited** | Routed application call returns the fixed virtual PID `42`. Native runtime calls still see the host PID. |
| `gettid` | **Host runtime** | Native thread identity only; no simulated thread ID syscall. |
| `tgkill` | **Host runtime** | Native Go's signal/preemption path; no simulated thread signals. |
| `rt_sigaction` | **Host runtime** | Native Go signal-handler installation; Gosim has no virtual signal-handler syscall. |
| `rt_sigprocmask` | **Host runtime** | Native Go thread signal masks; not simulated. |
| `rt_sigreturn` | **Host runtime** | Native kernel signal return; not simulated. |
| `sigaltstack` | **Host runtime** | Native Go alternate signal stack; not simulated. |
| `prctl` | **Host runtime** | Trace's optional Go VMA-name call is native. Direct translated `prctl` is unsupported. |
| `prlimit64` | **Host runtime** | Native process limit discovery; no virtual limits syscall. |
| `getrandom` | **Limited** | Translated call with flags 0 gets deterministic Gosim PRNG bytes, useful for replay but not cryptographic entropy. Other flags return `EINVAL`; skipped crypto packages may still use host entropy. |
| `uname` | **Limited** | Virtual nodename is set from the simulated machine label; it is not a full kernel identity model. |
| `exit_group` | **Host runtime** | Server process shutdown in the trace. A Gosim machine ends through its simulation lifecycle; direct process-exit syscalls are unsupported. |

### Launch boundary (1)

| Observed syscall | Status | Gosim behavior and gquorum implication |
| --- | --- | --- |
| `execve` | **Launch boundary** | This is strace entering `/cache-server`, not evidence that the gquorum library spawns children. Gosim's translated `syscall` before/after-exec hooks currently panic if used; subprocess behavior is outside the modeled machine. |

## What Gosim implements beyond this trace

The trace did not exercise every Gosim route. The generated interface also exposes `chdir`, `getcwd`, and ARM64 `fstatat`, as well as internal virtual poll registration. Absence from one trace does not imply these calls are irrelevant to another gquorum workload. Conversely, a method's existence does not mean every Linux flag or option is implemented. Gosim's own [README](../../README.md) describes its filesystem and network API as limited; [behavior tests](../../internal/tests/behavior) demonstrate selected file, network, and crash cases, not gquorum compatibility.

The supplied `catalogue.json` also defines `exit`, but its `observed.json` contains **no `exit` entry**. It is therefore excluded from the 57-name measured table. A direct translated thread/process exit has no virtual syscall implementation; host Go thread exits remain native runtime behavior.

## Priority for adding gquorum tests

| Priority | Work | Reason and first useful validation |
| --- | --- | --- |
| 1 | Resolve translated gRPC/HTTP integration, or introduce an in-process transport adapter at gquorum's transport boundary. | Current gRPC-based cache-server tests cannot compile as fully translated Gosim programs; see the excluded [HTTP/gRPC test](../../internal/tests/behavior/net_grpc_http_test.go) and [skip list](../../internal/translate/main.go). Start with a two-machine request/response test before Raft. |
| 2 | Implement meaningful `flock` and `fcntl` (`F_OFD_*` plus required descriptor flags), including contention and release on close/crash. | Avoid false positive WAL/database ownership tests. Add two-machine and same-machine lock-contention tests; check exactly which lock API versions of bbolt/etcd use. |
| 3 | Implement `fchmod`/`fchmodat`, store mode bits, and report them through stat. | Identity and snapshot publication currently call chmod. Test those paths on fresh and recovered storage. |
| 4 | Lift or make configurable the 16 MiB file-mapping cap and verify bbolt remap behavior. | A growing applied-state/metadata DB can outgrow a small smoke test. Exercise page growth and crash/reopen with actual gquorum dependency versions. |
| 5 | Tighten virtual TCP behavior and decide how tests will avoid or model DNS. | The observed workload uses UDP DNS, while Gosim accepts IPv4 TCP only; use numeric simulated peer addresses for the first tests. Test listener address reporting and `SO_ERROR`/other options actually used by the transport. |
| 6 | Expand integration coverage to WAL rotation, snapshots, restart, partitions, and both `DisableFSync` settings. | gquorum's report captured ordinary startup/load/shutdown, not all recovery paths. Assert durability ordering and deterministic replay across seeds. |

The first useful slice is likely a **storage-only gquorum test** using a virtual directory and a minimal transport substitute. It can exercise WAL, bbolt, fsync boundaries, machine crashes, and recovery while the gRPC translation gap is addressed. Treat a passing test as evidence only for the exercised calls and flags; the current no-op locks and socket options can otherwise hide real bugs.

## Evidence and audit limits

- The supplied gquorum evidence is at `/Volumes/M4DATA/Users/speckzzz/reps/.paseo/worktrees/1lh543gc/structural-refactorings-and-fuzzing/docs/reports/gquorum-syscalls.md`, with the 57-name catalogue and measured counts in adjacent `syscalls/catalogue.json` and `syscalls/observed.json`. Its capture used Linux/ARM64 and Go 1.26.5 with fsync-on/off variants. It is an application-plus-runtime trace, not a direct list of calls made solely by gquorum.
- Gosim code evidence: [Linux syscall methods](../../internal/simulation/os_linux.go), [ARM64 generated wrappers](../../internal/simulation/linux_gen_arm64.go), [raw fallback](../../internal/simulation/os_linux.go#L165), [syscall hooks](../../internal/hooks/go123/syscall.go), and [translator skip list](../../internal/translate/main.go).
- gquorum source evidence in the supplied worktree: `storage/walstore/state.go` (identity chmod/sync/rename), `snapshot.go` (snapshot chmod), `refill.go` (flock), and `operations.go` (WAL/applied-state sync ordering). The supplied report also identifies dependency calls in bbolt v1.4.3 and etcd v3.6.12.
- This is a **source audit**, not a successful gquorum-on-Gosim integration run. Third-party call paths may differ with another dependency version or with gquorum features absent from the capture. Go runtime-only trace calls were classified separately; direct use of many of those Linux syscalls from translated application code remains unsupported.
