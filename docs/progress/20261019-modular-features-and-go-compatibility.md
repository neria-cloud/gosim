# Modular features and Go compatibility execution record

Plan: `docs/plans/20261019-modular-features-and-go-compatibility.md`
Target: `docs/brainstorms/20261018-modular-features-and-go-compatibility-target.md`
Branch: `feat/refactorin-modularization`
Started: 2026-10-19; baseline commit: `06cc296`.

## Initial evidence

- Read the full brainstorm and target before implementation; the target is the design authority.
- The initial worktree was clean; all 33 plan tasks were unchecked.
- Baseline: `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed (ordinary and race suites, including simulation and race-oracle harnesses). Full log: `.gosim/baseline-go1.26.0.log`.
- Active development worker: go1.26.0, darwin/arm64, GOROOT `/Users/speckzzz/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.darwin-arm64`; default experiments/GOFLAGS; CGO_ENABLED=1 for native SQLite/race tooling. This is development evidence, not target certification.
- Exact target distributions were obtained/resolved and their compiler and source contents verified against pinned official archives. Probe log: `.gosim/fingerprint-probe.log`; no tuple promoted.
- Planning notice correction: the plan is tracked in this checkout; only the brainstorm directory is ignored. Plan/progress edits can be committed normally.

## Declaration ownership ledger

Baseline inventory SHA-256: `84e3efb31d1c9fcf58accc984cc9f7002266b0d799f87da707e98d8f02d31cf3`.
Every declaration in each referenced inventory section is assigned below, including private types/helpers and generated wrappers. “All N” is the complete ordered declaration set in that section, not only exported methods. This accounts for 110 files and 1,604 declarations without duplicating their signatures. Target overrides replace corresponding baseline signatures; they do not excuse dropping unrelated declarations. Task 33 must reconcile each row against actual final destinations.

| Inventory source section | Declarations assigned | Destination / responsibility | Owning task(s) |
| --- | --- | --- | --- |
| `gosimruntime/chan.go` | all 56 | `rt/runtime/chan.go` | 2 |
| `gosimruntime/checksum.go` | all 8 | `rt/runtime/checksum.go` | 2 |
| `gosimruntime/checksumkey_string.go` | all 2 | `rt/runtime/checksumkey_string.go` | 2 |
| `gosimruntime/doc.go` | all 0 | `rt/runtime/doc.go` | 2 |
| `gosimruntime/fnv64.go` | all 5 | `rt/runtime/fnv64.go` | 2 |
| `gosimruntime/globals.go` | all 14 | `rt/runtime/globals.go` | 2 |
| `gosimruntime/log.go` | all 12 | `rt/runtime/log.go` | 2 |
| `gosimruntime/map.go` | all 43 | `rt/runtime/map.go` | 2 |
| `gosimruntime/raceutil.go` | all 0 | `rt/runtime/raceutil.go` | 2 |
| `gosimruntime/raceutil_norace.go` | all 8 | `rt/runtime/raceutil_norace.go` | 2 |
| `gosimruntime/raceutil_race.go` | all 8 | `rt/runtime/raceutil_race.go` | 2 |
| `gosimruntime/rand.go` | all 8 | `rt/runtime/rand.go` | 2 (move); 3 (engine/random overrides) |
| `gosimruntime/runtime.go` | all 75 | `rt/runtime/runtime.go` | 2 (move); 3 (engine/random overrides) |
| `gosimruntime/sema.go` | all 17 | `rt/runtime/sema.go` | 2 |
| `gosimruntime/testmain.go` | all 8 | `rt/runtime/testmain.go` | 2; 21/23 (Driver/protocol) |
| `gosimruntime/time.go` | all 13 | `rt/runtime/time.go` | 2 (move); 3 (engine/random overrides) |
| `gosimruntime/timer_heap.go` | all 15 | `rt/runtime/timer_heap.go` | 2 |
| `gosimruntime/traceflag.go` | all 6 | `rt/runtime/traceflag.go` | 2 |
| `internal/coro/coro_linkname.go` | all 9 | `internal/hostabi; internal/compat/go125 and go126 host` | 12/16; 29 (retirement) |
| `internal/coro/coro_nolinkname.go` | all 5 | `internal/hostabi; internal/compat/go125 and go126 host` | 12/16; 29 (retirement) |
| `internal/coro/upcallcoro_norace.go` | all 6 | `internal/hostabi; internal/compat/go125 and go126 host` | 12/16; 29 (retirement) |
| `internal/coro/upcallcoro_race.go` | all 6 | `internal/hostabi; internal/compat/go125 and go126 host` | 12/16; 29 (retirement) |
| `internal/cryptorand/rand.go` | all 3 | `internal/compat/go125 and go126 shims/cryptorand` | 13/17; 29 (retirement) |
| `internal/hooks/go123/doc.go` | all 0 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/entrypoint.go` | all 5 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/golangorg_x_sys_unix.go` | all 7 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go123/golangorg_x_sys_unix_gen_amd64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 14/18; 27 (retirement) |
| `internal/hooks/go123/golangorg_x_sys_unix_gen_arm64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 15/19; 27 (retirement) |
| `internal/hooks/go123/hash_maphash.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/internal_abi.go` | all 2 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/internal_bytealg.go` | all 12 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/internal_chacha8.go` | all 2 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/internal_cpu.go` | all 6 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/internal_godebug.go` | all 4 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/internal_poll.go` | all 12 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go123/internal_syscall_unix.go` | all 4 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go123/maps.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/math_rand.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/math_rand_v2.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/net.go` | all 1 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go123/os.go` | all 7 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go123/os_signal.go` | all 6 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go123/runtime_debug.go` | all 10 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/runtime_trace.go` | all 4 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/sync.go` | all 31 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/sync_atomic.go` | all 42 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/syscall.go` | all 21 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go123/syscall_gen_amd64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 14/18; 27 (retirement) |
| `internal/hooks/go123/syscall_gen_arm64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 15/19; 27 (retirement) |
| `internal/hooks/go123/time.go` | all 11 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go123/weak.go` | all 2 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/doc.go` | all 0 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/entrypoint.go` | all 5 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/golangorg_x_sys_unix.go` | all 7 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go125/golangorg_x_sys_unix_gen_amd64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 14/18; 27 (retirement) |
| `internal/hooks/go125/golangorg_x_sys_unix_gen_arm64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 15/19; 27 (retirement) |
| `internal/hooks/go125/hash_maphash.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/internal_abi.go` | all 2 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/internal_bytealg.go` | all 12 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/internal_chacha8.go` | all 2 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/internal_cpu.go` | all 6 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/internal_godebug.go` | all 4 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/internal_poll.go` | all 12 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go125/internal_syscall_unix.go` | all 4 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go125/maps.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/math_rand.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/math_rand_v2.go` | all 1 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/net.go` | all 1 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go125/os.go` | all 7 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go125/os_signal.go` | all 6 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go125/runtime_debug.go` | all 10 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/runtime_trace.go` | all 4 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/sync.go` | all 31 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/sync_atomic.go` | all 42 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/syscall.go` | all 21 | `internal/compat/go125 and go126 hooks` | 14–19 (routes/policies); 27 (retirement) |
| `internal/hooks/go125/syscall_gen_amd64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 14/18; 27 (retirement) |
| `internal/hooks/go125/syscall_gen_arm64.go` | all 35 | `internal/compat/go125 and go126 hooks` | 15/19; 27 (retirement) |
| `internal/hooks/go125/time.go` | all 11 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/hooks/go125/weak.go` | all 2 | `internal/compat/go125 and go126 hooks` | 12/16 (runtime); 13/17 (policies); 27 (retirement) |
| `internal/maps_shim/shim.go` | all 0 | `internal/compat/go125 and go126 shims/maps_shim` | 13/17; 29 (retirement) |
| `internal/race/doc.go` | all 0 | `internal/hostabi; internal/compat/go125 and go126 host` | 12/16; 29 (retirement) |
| `internal/race/norace.go` | all 14 | `internal/hostabi; internal/compat/go125 and go126 host` | 12/16; 29 (retirement) |
| `internal/race/race.go` | all 14 | `internal/hostabi; internal/compat/go125 and go126 host` | 12/16; 29 (retirement) |
| `internal/reflect/deepequal.go` | all 3 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/reflect/doc.go` | all 0 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/reflect/makefunc.go` | all 1 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/reflect/no.go` | all 12 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/reflect/swapper.go` | all 1 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/reflect/type.go` | all 62 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/reflect/value.go` | all 111 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/reflect/visiblefields.go` | all 1 | `internal/compat/go125 and go126 shims/reflect` | 13/17; 29 (retirement) |
| `internal/simulation/fs/chunkedfile.go` | all 16 | `internal/fs/model/chunkedfile.go` | 7 (including owned crash-model overrides) |
| `internal/simulation/fs/depkind_string.go` | all 4 | `internal/fs/model/depkind_string.go` | 7 (including owned crash-model overrides) |
| `internal/simulation/fs/filesystem.go` | all 80 | `internal/fs/model/filesystem.go` | 7 (including owned crash-model overrides) |
| `internal/simulation/fs/pendingops.go` | all 23 | `internal/fs/model/pendingops.go` | 7 (including owned crash-model overrides) |
| `internal/simulation/network/config.go` | all 19 | `internal/net/model/config.go` | 10 (ports/directed edges/generations) |
| `internal/simulation/network/delayqueue.go` | all 14 | `internal/net/model/delayqueue.go` | 10 (ports/directed edges/generations) |
| `internal/simulation/network/faults.go` | all 11 | `internal/net/model/faults.go` | 10 (ports/directed edges/generations) |
| `internal/simulation/network/net.go` | all 22 | `internal/net/model/net.go` | 10 (ports/directed edges/generations) |
| `internal/simulation/network/stack.go` | all 39 | `internal/net/model/stack.go` | 10 (ports/directed edges/generations) |
| `internal/simulation/network/trace.go` | all 1 | `internal/net/model/trace.go` | 10 (ports/directed edges/generations) |
| `internal/simulation/syscallabi/errno.go` | all 2 | `rt/runtime/abi; internal/contracts; internal/kernel` | 2 (views); 4/6 (transport/poll); 28 (retirement) |
| `internal/simulation/syscallabi/poll.go` | all 14 | `rt/runtime/abi; internal/contracts; internal/kernel` | 2 (views); 4/6 (transport/poll); 28 (retirement) |
| `internal/simulation/syscallabi/syscall.go` | all 26 | `rt/runtime/abi; internal/contracts; internal/kernel` | 2 (views); 4/6 (transport/poll); 28 (retirement) |
| `internal/synctest/synctest.go` | all 11 | `internal/compat/go125 and go126 shims/synctest` | 13/17; 29 (retirement) |
| `internal/testing/match.go` | all 17 | `internal/compat/go125 and go126 shims/testing` | 13/17; 29 (retirement) |
| `internal/testing/missing.go` | all 19 | `internal/compat/go125 and go126 shims/testing` | 13/17; 29 (retirement) |
| `internal/testing/testing.go` | all 57 | `internal/compat/go125 and go126 shims/testing` | 13/17; 29 (retirement) |
| `metatesting/doc.go` | all 0 | `metatesting/doc.go` | 23 |
| `metatesting/metatest.go` | all 17 | `metatesting/metatest.go` | 23 |

### New and overridden target declarations (T§4–9)

Each row assigns the entire named target declaration block, including all listed fields, constructors, methods, sim/native variants, errors, and private helpers. Signatures remain sourced from the target; this ledger assigns implementation responsibility, not alternative contracts.

| Target declaration block | Owner/task | Wiring and verification owner |
| --- | --- | --- |
| §4 machine.go: Machine/MachineState/ExitKind/Exit/MachineConfig and all 19 machine/root entry points | Public root; 22 | 22, 25, 33 |
| §4 errors.go: seven sentinel errors, FeatureDisabledError/Error | Public root; 22 | 22, 25 |
| §4 fs config/control/recovery: Config/CrashMode/SyncFault/InodeInfo/Snapshot/CrashIterator/EnumerateOptions/ErrEnumerationLimit and every listed function/method | fs; 22, backed by 8/9 | 22, 25 |
| §4 net config/control: LatencyConfig/LossConfig/Config/MachineConfig/Direction/Link/LinkPolicy and all 9 functions | net; 22, backed by 10/11 | 22, 25 |
| §4 rt control/info: SetTimeout, Info, RunInfo | rt; 22 | 22, 25 |
| §5 identity/errors: Feature/FeatureSet/Has/ParseFeatures/Names, MachineRef/EpochRef/CallContext, StopKind/CloseReason/EventID/WaitID/Readiness/Diagnostic | contracts; 4 | 5/6/22 |
| §5 ports: Clock/Random/Events/ReadinessSink/Trace/Field/Ports and every interface method | contracts; 4; adapters in 5 | 5, 7–11 |
| §5 lifecycle: StartSpec/Provider/ProviderSession/PreparedMachine/MachineInstance/Bindings/ControlBindings and every interface method | contracts; 4; implementations 8/11 | 5, 8/11 |
| §5 machine/transaction: State/Machine/Run/StartTxn/TxnState/ErrTxnState/RuntimePrepared/ExitRecord; Create/BeginStart/Commit/Abort/Start/Restart/Stop/Destroy/SetMain/Wait/Lookup/Accepts | kernel; 5 | 5/6/25 |
| §5 kernel/services: Config/Kernel/New/Run/Close/Require/AbortRun/Ports/mergeBindings | kernel; 5 | 6/21/25 |
| §5 dispatcher: Invocation/RouteID/Result/Dispatcher/NewDispatcher/Submit/DispatchOne/Complete/CancelEpoch/Drain | kernel; 6 | 14/21/25 |
| §5 descriptors: Resource/Readable/Writable/Pollable/DescriptorTable and every interface method | contracts; 4; providers 9/11 | 6/9/11 |
| §5 fdtable/poll: NewFDTable/Allocate/Lookup/Remove/Finalize, Read/Write/CloseFD, PollOpen/PollClose/PollWait/PollReset/PollDeadline/PollUnblock/Notify | kernel; 6 | 6/14/25 |
| §7 FileOps/Stat/Lock: Openat/Renameat/Getdents64/Fallocate/Seek/Pwrite/Pread/Sync/Fstat/Statat/Unlinkat/Truncate/Chdir/Getcwd/Mkdirat/Flock/FcntlInt/FcntlLock/Map/Unmap/Advise | contracts 4; fs Instance 9 | 14/15/18/19 |
| §7 SocketOps/Address: Socket/Bind/Listen/Accept/Connect/LocalAddress/PeerAddress/SetOption/GetOption/FcntlInt | contracts 4; net Instance 11 | 14/15/18/19 |
| §7 FSConfig/FSFaultConfig/InodeInfo/FSControl: ConfigureMachine/SetCrashMode/SetSyncFault/InspectInode/Enumerate/NextSnapshot/CloseIterator/Restore/CloseSnapshot | contracts 4; fs Session 8 | 14/22/25 |
| §7 NetConfig/NetLink/NetLinkPolicy/NetControl: Configure/ConfigureMachine/Address/SetLinkPolicy/ResetLinkPolicy/SetConnectivity | contracts 4; net Session 11 | 14/22/25 |
| §7 fs provider/session/lifecycle/control table: NewProvider/Feature/Open/Controls/Prepare/Destroy/Close/Bindings/Publish/Abort/Quiesce/Settle/Detach/validateConfig/validateFault | fs; 8 | 8/21/25 |
| §7 fs File: Owner/Release/Read/Write | fs; 9 | 6/9/25 |
| §7 net provider/session/lifecycle/control table: all named Provider/Session/Prepared/Instance methods and validators | net; 11 | 11/21/25 |
| §7 net Socket: Owner/Release/Read/Write/Subscribe/Unsubscribe | net; 11 | 6/11/25 |
| §7 network model overrides: NewNetwork/ReserveAddress/AbortReservation/Attach/Detach/Reservation, NewStack/ProcessIncoming/Shutdown, packet epoch/generation | net/model; 10 | 11/25 |
| §7 fs model overrides: FreezeCrash/CrashImage/Clone/IterCrashes/Release, CrashIterator.Next/Close; plannedOp/crashPlan/crashFrame/crashPlanIterator/snapshot/randomCrashSubset/iterator/next/close | fs/model; 7 | 8/25 |
| §7 process: Getrandom/Getpid/Uname/ProcessInfo | kernel; 6 | 14–19/25 |
| §7 control: CurrentRef/Submit/Bind and generated route payloads | control; 14 | 21/22/25 |
| §8 Engine/EngineConfig/MachineToken: NewEngine/PrepareMachine/PublishMachine/AbortMachine/CancelMachine/DisposeMachine/Schedule/CancelEvent/Abort/Run/Close | rt/runtime; 3 | 5/21/25 |
| §8 Random: NewDomainRandom/Uint64/Uint64N/Float64/ExpFloat64 | rt/runtime; 3 | 5/7/10/25 |
| §8 ABI views and primitive IDs/Diagnostic/HostIdentity | rt/runtime/abi; 2 | 3/6/12/25 |
| §8 hostabi: Coro.Start/Next/Yield/Finish, RaceAcquire/Release/Read/Write/Disable/Enable, Identity and retained inventory calls | hostabi + family host; 12/16 | 12/16/29 |
| §8 family Descriptor/profile/tuples/hooks manifest/policies and all native/translated family declarations | compat/go125 12–15; compat/go126 16–19 | 19/24/32 |
| §8 Profile/Hook/PackagePolicy/Validate/Select, selected.Profile | spec 1; selected 12 | 21/24 |
| §9 Config/Identity/Revision/Tuple/Selection/Toolchain/BuildOptions, Overrides/Distribution | buildspec/buildcfg; 1 | 20/21/24 |
| §9 Load/Normalize/Resolve/ValidateWorker/Fingerprint/Command/Identity/Canonical/Digest/ValidateTuple/VerifyHooks | buildcfg/spec; 1 | 21/24 |
| §9 Generate/ValidateImports/GeneratedComposition | compose; 21 | 21/25 |
| §9 Translate/translatePackages/translatePackage/renameFile/Policies/ValidateBoundary, TranslateInput/translatePackageArgs | translate; 21 | 24/25 |
| §9 BuildConfig/TranslateOutput/AsDirname/GetPathForPrecompiledTestBinary | gosimtool; 20/21/23 | 23/24 |
| §9 BuildFlags/ResolvedBuild/resolveBuild, shared CLI building flags/commands, selftest | cmd/gosim; 21/24 | 24/32 |
| §9 certify/MatrixOptions/CertificationReport/SuiteResult | cmd/gosim; 24 | 30/32 |
| §9 GenerateProfile | cmd/gosim-gen; 14 | 15/18/19 |
| §9 metatesting Options/ForPackage/ForOtherPackage/newRunner/Close and retained ListTests/RunAllTests/Run | metatesting; 23 | 23/25 |
| §9 Driver.Run/TestEntrypoint/TestMain, copied RunConfig/RunResult and protocol metadata | runtime/compose; 21/23 | 23/25 |
| §9 computeTranslateToolHash/computePackageHash/CacheEntryV2/TranslatePackageResult/PackageGlobalInfo, encode/decode/cacheGet/cachePut | translate; 20/21 | 20/25 |
| §9 DB/cache NewDB/Get/Put/Touch/Clean/Close plus Delete; unchanged SQLite table/TTL | translate/cache; 20 | 20/33 |
| §9 package manifests, staging/locking/content verification and run metadata | gosimtool; 20/21/23 | 23/25/32 |

### Route ownership ledger

Every route in T§7 is assigned; generated declarations in both architecture inventories are covered by the file ledger above. The route generator tasks must expand this into checked per-symbol family/architecture manifests, preserving limited/stub/rejected capability classifications.

| Routes / controls | Target handler | Implementation / adapter tasks |
| --- | --- | --- |
| openat, renameat, getdents64, fallocate, lseek, pread64, pwrite64 | FileOps | 9; 14/15/18/19 |
| fsync, fdatasync | FileOps.Sync(dataOnly) | 9; 14/15/18/19 |
| fstat, fstatat/newfstatat, unlinkat, ftruncate, chdir, getcwd, mkdirat | FileOps | 9; 14/15/18/19 |
| flock, fcntl pointer locks, mmap, munmap, madvise | FileOps | 9; 14/15/18/19 |
| socket, bind, listen, accept4, connect, getsockname, getpeername, setsockopt, getsockopt | SocketOps | 11; 14/15/18/19 |
| fcntl integer form | kernel owner dispatch to FileOps/SocketOps | 6/9/11; 14/15/18/19 |
| read, write, close | kernel descriptor capabilities | 6; 14/15/18/19 |
| internal/poll operations | kernel poll generations/deadlines | 6; 14/15/18/19 |
| getrandom, getpid, uname | kernel process services | 6; 14/15/18/19 |
| machine controls, self exits, waits | kernel lifecycle/control | 5/14/22 |
| fs diagnostics, configuration, snapshots/recovery | FSControl guarded by FS | 8/14/22 |
| network configuration, addressing, link policy, partition/repair | NetControl guarded by Net | 10/11/14/22 |
| runtime timeout/info and primitive hooks | engine/control | 3/12/16/22 |
| unknown syscall/private symbol and named init probes | explicit profile policy/rejection | 13/14/17/18/25 |

## Task 1 — Build specification, toolchain and profile contracts
Date: 2026-10-19. Commit: the Task 1 implementation commit containing this entry.

Implemented
- Exact persisted Config/Identity/Revision/Tuple and transient Selection/Toolchain/BuildOptions/Overrides/Distribution records, strict configuration/feature validation and owned canonical identities.
- Exact executable resolution, bounded child environments/flags, worker mismatch diagnostics, and pinned official archive verification of executable/compiler/stdlib contents. Both direct SDK and Go toolchain-module installations were verified; byte-identical `_go.mod` copies are the only accepted source-layout transformation.
- Profile validation/selection, deterministic hook manifest digests, feature-specific provider imports, source/evidence-bound tuple acceptance, and declaration/signature checks against the selected toolchain's compiled Linux files.
- Source/module/debug-policy fingerprints exclude evidence/status metadata to prevent circular promotion. Build and evidence identities remain distinct. No executable adapter or compatibility status was fabricated.
- Ownership ledger covers all 110 baseline files / 1,604 declarations plus T§4–9 declarations and route families. Correction to the descriptive ledger counts above: the machine block has 16 listed functions/methods and the net public block has 8; both rows assign the entire block regardless of those counts.

Verified
- `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1`: passed before changes and after implementation, including formatting, CLI build, translated artifacts, all ordinary tests and all race tests. Logs: `.gosim/baseline-go1.26.0.log`, `.gosim/task1-full-suite.log`.
- Focused buildcfg/spec tests: missing/invalid/duplicate features, config replacement/unknown fields, canonical identity dimensions and ownership, source/promotion separation, wrong worker/environment/flags, modified compiler/source/archive, profile isolation and pending/certified evidence, actual hook ABI drift/missing symbol detection.
- Real exact-patch resolution/fingerprinting: go1.25.14 and go1.26.9 on darwin/arm64 both passed; archive hashes match pinned official metadata. The probe downloaded no unpinned executable and did not certify compatibility.
- Source digests: go1.25.14 `08f07471a153a1300478b22176240df8396932c299d756f5d4b6a843829b9e08`; go1.26.9 `2503d7ed10c3ddb78010692d8e835762232379121f3fdd3a8b6bf3b38168b9fa`.

Notes
- Go's own child environment includes GOROOT, and macOS temporary paths have `/var` and `/private/var` aliases. Resolution handles the inherited worker root while pinning the selected child root; tests compare the canonical working directory.
- Repeated init declarations are valid and not addressable hook symbols; the ABI scanner excludes them and anonymous `_` declarations.
- Additional helper files remain within the planned three packages; no legacy execution behavior changed. Build identities intentionally cannot select unfinished adapters. All later tasks remain required.
- Final rename-aware Task 1 size: 2,244 changed lines (2,233 additions + 11 deletions), below the 5,000-line cap; `git diff --check` clean.

## Task 2 — Native runtime relocation and memory ABI
Date: 2026-10-19. Previous task commit: `5757fc9`. Commit: the Task 2 implementation commit containing this entry.

Implemented
- Moved all runtime production/tests from `gosimruntime/` to `rt/runtime/` as renames. A content comparison against the parent commit proved no computational changes in moved files; only one moved test import changed.
- Extracted SliceView, ValueView, Bool helpers and ByteView into native `rt/runtime/abi`, preserving bounds and go:norace methods. Added primitive IDs, Diagnostic and HostIdentity. Removed old view declarations instead of forwarding them.
- Updated all consumers, translation exclusions/imports/goldens, generated wrappers and their generator. Both engine and ABI remain native in generated external modules; no legacy engine import survives in source.
- Added external-module execution under ordinary and race builds, plus focused bounds/borrowing/boolean tests.

Verified
- `GOTOOLCHAIN=go1.26.0 go test -ldflags=-checklinkname=0 -tags=linkname ./rt/runtime/... ./internal/simulation/...`: passed.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1`: passed, including new external-module ordinary/race fixture, translation goldens, existing behavior/metatesting/race-oracle suites. Final log `.gosim/task2-gate.log`.
- Regenerated all current syscall outputs with `GOTOOLCHAIN=go1.26.0 go run ./internal/simulation/gensyscall`. No syscall/signature changes; output import grouping is reproducible after removing an empty optional-import line from the generator.
- Source audit found no old engine imports or syscallabi memory-view references. Historical documentation paths remain assigned to Task 31; package identifiers retain their existing names.

Notes
- First full run caught the shortened runtime source path in the pretty-log golden. Updated the fixture and verified it before rerunning the full gate; no test was removed.
- Initial isolated generation probe needed the generator's `internal/translate` output directory; after creating it, the only differences were import grouping. The real generator was then run against the checkout and all suites rerun.
- Core Task 2 implementation size before checklist/log: 796 changed lines with rename detection; unchanged runtime moves were reviewed by content comparison. Final size is recorded below.
- Final Task 2 size: 834 changed lines (495 additions + 339 deletions), including checklist/log, with 22 runtime renames; staged whitespace check passed.

## Task 3 — Engine lifecycle and owned deterministic random streams
Date: 2026-10-19. Previous task commit: `3d1bf1d`. Commit: the Task 3 implementation commit containing this entry.

Implemented
- Engine/EngineConfig/MachineToken and every target lifecycle/event method, using the existing coroutine scheduler with a single-active-run guard. Preparation allocates globals without running boot; publication queues boot; cancellation removes waits/timers without application defers; disposal releases retained coroutine/global state after borrowing ends.
- Native run abort records primitive diagnostics before parking application execution. The trusted supervisor survives long enough to run harness cleanup inside the active scheduler. Cleanup failures preserve original diagnostics and cannot prevent later cleanup. Provider events run with supervisor globals and verify their original machine epoch.
- Versioned SHA-256 length-delimited random derivation (`gosim-rng-v2`), existing 64-bit update algorithm, unbiased rejection sampling, Float64/ExpFloat64, fixed domain/machine ownership, and stable streams across restart. New engine scheduler and application streams are separate; legacy integration retains its existing sequence until cutover.
- Exported RunConfig/RunResult (including build/replay metadata) and the Driver contract. Actual generated Driver and TestMain cutover remain assigned to Tasks 21–22.
- Scheduler cleanup now runs on deadlock/error returns as well as normal completion; previously the deadlock branch could retain parked coroutines.

Verified
- Native engine and ABI tests passed with the race detector. Coverage includes failed/aborted preparation, consumed epochs, no boot inside publication, blocked-call cancellation, retained globals/coroutines until disposal, stale events after restart, canceled timers, event/epoch exhaustion, application and provider panic, timeout/deadlock, repeated/invalid lifecycle operations, and single-active-run rejection.
- Application recovery/defer cannot hide abort; supervisor defers still clean up and original plus cleanup diagnostics survive.
- Independent known RNG vector computed with Python SHA-256 and integer multiplication matches Go; bounded/float draws, independent domain streams, restart ownership, logical-machine application randomness, and repeated-seed runs pass.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed after final executable changes, including format/build/ordinary/race suites. Final log `.gosim/task3-final-gate.log`; focused race log `.gosim/task3-runtime-race.log`.

Notes / migration ledger
- New-engine replay revision is `gosim-replay-v2`; existing integration replay goldens did not change.
- Remove old Runtime/Setup/TestMain wiring, globalRuntime/initializeRuntime/run integration, and exported random-hook legacy fallbacks in Task 22. Their source carries TODO(migration) markers. They remain solely to keep the authorized intermediate full-suite path runnable; no profile selects the unfinished new composition.
- Initial race tests caught missing release edges at run entry and publication; each boundary now explicitly publishes captured state. A later lifecycle audit caught premature coroutine pooling and supervisor cancellation; regression tests now enforce both ownership requirements.
- All changes are within Task 3 runtime scope; no tests were waived or compatibility tuples promoted.
- Final Task 3 size: 1,049 changed lines (1,016 additions + 33 deletions), including checklist/log; staged whitespace check passed.

## Task 4 — Translated provider and control contracts
Date: 2026-10-19. Previous task commit: `b0bc40c`. Commit: the Task 4 implementation commit containing this entry.

Implemented
- Exact feature/identity/epoch/call records, lifecycle and memory-view service interfaces, session controls, ports and descriptor capabilities from T§5/7. Contracts depend only on native ABI among Gosim packages.
- Canonical feature parsing and validation, exact provider set/order/ownership checks, duplicate-binding rejection, and finite/valid filesystem and network configuration validation.
- Test-only recording lifecycle/port/descriptor fakes and complete service fakes that explicitly reject unexercised calls; no product implementation or fallback is selected.

Verified
- Focused contracts tests passed: all target interfaces compile against fakes; canonical sets, dependency graph, provider ordering/ownership, no partial duplicate-binding result, lifecycle ordering/failure injection, copied trace fields, descriptor memory views/readiness and invalid value records.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed, including ordinary and race suites. Log `.gosim/task4-gate.log`.
- Initial gate found three formatting differences after the first formatting pass; a second formatter pass and standalone check resolved them before the full rerun.
- No migration stubs or production wiring were introduced. All remaining tasks and exact-patch certification remain required.
- Final Task 4 size: 1,027 changed lines including checklist/log; staged whitespace check passed.

## Task 5 — Kernel registry and lifecycle transactions
Date: 2026-10-19. Previous task commit: `47e80c6`. Commit: the Task 5 implementation commit containing this entry.

Implemented
- Kernel/Machine/Run/StartTxn/RuntimePrepared/ExitRecord structures and lifecycle methods, immutable feature selection, process-wide run IDs, non-reused machine/epoch/wait IDs, state validation and overflow failure.
- Ordered provider opening/control installation, preparation and ownership validation, atomic publication, reverse rollback after descriptor finalization, retained terminal records, epoch-capturing waits, protected root and returning-self rejection.
- Irreversible stop/crash cleanup, once-only settling, joined cleanup failures, root-last teardown, and primitive internal self-exit messages. Main return requests graceful termination; the trusted crash route skips application defers. Kernel.Run executes in Engine.Run's supervisor and leaves Engine.Close to its runner.
- Owned clock/events/trace/random ports with epoch admission, canceled/fired event removal, copied trace fields, independent random streams and restart continuity. Disabled features abort through the native engine and cannot be recovered by application code.
- Native process identity allocator, current-machine token and non-yielding lifecycle guard. Supervisor/event race synchronization applies only to supervisor state; an isolated deliberate application-race subprocess proves the race oracle remains effective.

Verified
- Real-engine ordinary/race lifecycle fixtures cover provider open/control failures, each provider preparation/binding boundary, runtime preparation failure, aborted/repeated commits, publication panic/nil instance/yield, rollback cleanup failures, consumed failed epochs, restart preservation, stale/wrong-run/destroyed handles, waits across restart, root protection, self crash, application panic, disabled feature abort, and ID exhaustion.
- Cleanup tests verify reverse quiesce/detach, filesystem-first settling, remove-before-release descriptors, graceful failure switching remaining cleanup to crash, all joined error messages, and provider/session cleanup despite failures.
- Ports tests verify no staging events, stale callback rejection after restart, live events, event bookkeeping, trace ownership and random independence. Import test permits only contracts/native runtime/ABI among Gosim kernel dependencies.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed, including formatting, CLI/build artifacts, ordinary suites and race suites. Final log `.gosim/task5-final-gate.log`; final focused race log `.gosim/task5-final-focused.log`.

Findings and migration ledger
- Controlled nondeterminism is not a publication barrier: it suppresses the initial coroutine park and can execute boot synchronously. A dedicated guard now rejects yields without changing startup parking or random draws; Sleep rejects before inserting a timer.
- Provider events run on the scheduler stack while lifecycle code runs on the supervisor coroutine. An explicit supervisor-only synchronization token describes that serialized ownership without hiding application races.
- State constants use StateNew/StateStarting/etc because Go cannot declare both constructor New and a package-level constant New. The required lifecycle states and method names are unchanged.
- Config carries native HostIdentity, the canonical full build digest, features and timeout. Importing the host buildspec package would pull os/exec into the translated kernel; that dependency was removed before commit and a boundary regression check added.
- Task 6 completes FDTable capabilities/polling and dispatcher invocation transport/wait completion. The minimal lifecycle descriptor table performs real ownership cleanup; the lifecycle-only dispatcher cancels observer records and validates drain, with no syscall fallback. Both carry Task 6 migration markers. No normal CLI composition selects the unfinished kernel yet.
- Final Task 5 size: 2,095 changed lines (2,072 additions + 23 deletions), including checklist/log; staged whitespace check passed.

## Task 6 — Invocation, descriptor, polling and process services
Date: 2026-10-19. Previous task commit: `9c8ef5d`. Commit: the Task 6 implementation commit containing this entry.

Implemented
- Exact Invocation/RouteID/Result transport and dispatcher admission/completion/cancellation/drain, non-reused IDs, bounded cancellation metadata, and supervisor processing through Kernel.Run. Envelopes accept explicit typed payloads, never callbacks or provider pointers; result extraction clears reusable envelopes.
- Shared monotonically allocated descriptor table, capability-based read/write/close, owner-based fcntl dispatch, sorted remove-before-release finalization, deterministic stdin EOF, and copied stdout/stderr output through the native log sink without filesystem support.
- Poll subscriptions, readiness/reset/unblock, relative deadlines, per-registration and deadline generations, failed-subscription rollback, and cancellation tied to invocation/epoch lifetime. Stale sinks/deadlines cannot target a reopened fd or restarted machine.
- Getrandom uses the logical machine's application stream; flags retain existing validation. Getpid remains the historical fixed 42 and uname models only the bounded nodename. Explicit process capability records label the stub/missing fields instead of inventing process semantics.
- Non-yielding native semaphore wake/retirement and diagnostic output adapters. Cancellation and scheduler teardown prune empty semaphore records so native tables do not retain disposed invocation/application addresses.

Verified
- Ordinary/race tests cover shared fs/socket descriptor ownership, fcntl selection, EBADF and missing capabilities, sorted teardown with multiple errors, fd/poll/invocation exhaustion, readiness/reset/unblock, stale registration/epoch/deadline delivery, failed subscription cleanup, and bounded cancellation metadata.
- Real application calls traverse Submit → supervisor dispatch → native completion, including timer-driven poll wake and machine Wait completion. A machine crashed while blocked in a real poll does not resume or execute defers; borrowed arguments/results are cleared before disposal and late completion does not touch the envelope.
- Unknown routes, callable payloads, overflowing mode values, duplicate completion and non-modeled/combined errors abort rather than becoming successful syscalls or errno fallbacks.
- Native log-sink test verifies output attribution; semaphore test proves no canceled waiter address remains. Deliberate-race subprocesses prove both engine and syscall transport still report unrelated application races.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed after the final native wait-table fix. Final log `.gosim/task6-final-gate.log`; focused race log `.gosim/task6-final-focused.log`. No ordinary or race suite was waived.

Notes / migration ledger
- Task 5 descriptor/dispatcher migration markers are removed. Task 14 still generates the complete manifest and remaining route decoders; the current explicit core-route decoder carries that marker and rejects every unlisted route. Normal CLI selection still uses the authorized intermediate path.
- Domain results remain in the translated envelope; ABI views and primitive semaphore cells cross native helper boundaries. Native generic channels preserve the translated envelope type rather than mixing original and translated contract types.
- Final Task 6 size: 1,940 changed lines (1,913 additions + 27 deletions), including checklist/log; staged whitespace check passed.

## Task 7 — Filesystem model and owned crash snapshots
Date: 2026-10-19. Previous task commit: `aba18c3`. Commit: the Task 7 implementation commit containing this entry.

Implemented
- Moved filesystem algorithms/tests to internal/fs/model, changed package name to model, updated temporary simulation/public imports and regenerated syscall wrappers from the updated generator.
- Immutable CrashImage with separately retained durable chunks, pending write chunks, copied operations and dependency metadata. Clone/IterCrashes/Release and Next/Close have explicit ownership; releasing the source, image or iterator cannot invalidate another retained owner.
- Exact plannedOp/crashPlan/crashFrame/crashPlanIterator representation and iterative inclusion-first DFS, checked unavailable-count overflow, stable operation order, copied results, empty-plan image and idempotent early/exhaustion cleanup. Removed the parked iterator goroutine and old CrashClone/closure API.
- Filesystem.Release now frees model state, pending payloads and mapping references. FlushEverything releases superseded durable/pending storage. Copy-on-write now drops the old shared chunk reference when installing a writable copy; reference counts reject overflow.
- Removed all model mutexes and global allocation pools. Fresh allocations belong to model/snapshot owners and do not carry mutable storage across runs. Random crash selection uses injected contracts.Random.Uint64N(3), preserving dependency-valid two-thirds selection.

Verified
- Existing pending-operation examples compare every intermediate graph against an independent copy of the former recursive order oracle. Tests cover 100,000 dependent operations, copied yields, empty plans, overflow, early/repeated Close and release, immutable pending payloads/directory metadata, snapshot survival after source flush/write/release, and zero chunk references after final owners release.
- Mapping updates on writes/truncation remain intact; snapshots/clones do not retain source mappings. Independent application/network draws do not change filesystem selection; selected operations respect prerequisites and the expected two-thirds rate.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed with ordinary/race behavior, script, translation, runtime and model suites: `.gosim/task7-gate.log`. Final additional mapping/directory tests passed in focused race execution: `.gosim/task7-focused.log`. Regeneration: `GOTOOLCHAIN=go1.26.0 go generate ./internal/simulation`.

Notes / migration ledger
- Temporary legacy callers freeze immutable images, close iterators on explicit release/machine shutdown, release superseded filesystems on restart, and retain a per-logical-machine native fs random stream across restarts. Marked adapters are removed in Task 22. The current dispatcher already serializes these model calls; the new provider will own them through the supervisor.
- Existing unsupported filesystem semantics, including unreachable inode retention in crash images, remain unchanged. No new compatibility tuple is certified by this development-toolchain gate.
- Final Task 7 size: 1,044 changed lines (708 additions + 336 deletions) with rename detection, including checklist/log; staged whitespace check passed.

## Task 8 — Filesystem session lifecycle and recovery controls
Date: 2026-10-19. Previous task commit: `ea7b5f8`. Commit: the Task 8 implementation commit containing this entry.

Implemented
- Filesystem Provider/Session/Prepared/Instance lifecycle with stable session controls, tentative Linux fixture/cwd creation, independent staged restart state, no publication/flush/random draws during preparation or abort, and persistent disk/configuration owned solely by the session.
- Once-only graceful flush or crash-time outcome selection, retained immutable pre-crash image, independent fs stream, and idempotent quiesce/detach/destroy/close. Prior disks survive failed restart preparation; successful publication drops the machine's obsolete crash image without invalidating existing iterators.
- All FSControl methods with atomic config/fault validation, before-first-start cwd configuration, live crash/sync-fault settings, inode inspection, positive enumeration budget, one retained lookahead, independently owned snapshots/restores, monotonic typed handles, checked exhaustion, closed-handle tombstones, and deterministic cleanup.
- Kernel FilesystemControl boundary validates feature/run/caller epoch and target state before invoking the contract under the non-yielding supervisor guard. Public recovery run identity is checked separately from session-local numeric IDs, preventing reuse of an earlier run's handle. Kernel has no filesystem model/provider dependency.

Verified
- Focused ordinary/race lifecycle and recovery tests cover default /app and Linux fixture, invalid/NaN/infinite configs leaving state unchanged, aborted first/restart preparation, failed restored-cwd resolution, graceful/crash differences, repeated settlement, random selection at crash rather than restart, and stale epoch bindings.
- Recovery tests cover empty/exact budget exhaustion versus ErrEnumerationLimit, bounded lookahead, allocation overflow without consuming a state, early/repeated close, wrong-kind/unknown handles, snapshots surviving iterator close/source destruction/restart, independent restores, restored ownership after snapshot close, and deterministic session cleanup of staged/active/retained resources.
- Real kernel/engine integration covers before-start and after-crash controls, state/run/stale-caller guards, destroyed targets, source-destroy snapshot restore, and second-provider preparation failure rolling back FS without publishing a disk.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed, including formatting, build artifacts, all ordinary suites and all race suites: `.gosim/task8-gate.log`. Final focused race log: `.gosim/task8-focused.log`.

Notes / migration ledger
- Added internal/kernel/control_fs.go to implement this task's required kernel state validation; generated routes/public control transport connect to that boundary in Tasks 14/22. Cross-run checks belong at this envelope boundary because the prescribed FSControl recovery methods carry only session-local uint64 IDs.
- Task 9 explicitly owns every FileOps syscall body. ops_pending.go supplies the complete interface with explicit invariant failures for unfinished calls, never success or host fallback; Task 9 removes it. Session fault settings are implemented here; their Sync syscall behavior is Task 9. Normal CLI composition still uses the authorized intermediate integration path.
- The user briefly paused implementation to discuss directory moves, then requested continuation of the original source plan/architecture. No target-architecture/layout revision was made.
- Final Task 8 size: 1,541 changed lines (1,533 additions + 8 deletions), including checklist/log; staged whitespace check passed.

## Task 9 — Filesystem syscall services and descriptor capabilities
Date: 2026-10-19. Previous task commit: `7ac3390`. Commit: the Task 9 implementation commit containing this entry.

Implemented
- Replaced every Task 8 FileOps placeholder with path/open/rename/directory entries, allocation/offset/positional I/O, synchronization, stat, remove/truncate/cwd/mkdir, lock/fcntl and mapping/advice methods. File implements Resource/Readable/Writable against the shared epoch descriptor table; failed allocation closes the inode reference, and Release transfers ownership only once with the original CloseReason.
- Linux flag values are explicit at the provider boundary, then converted to the host-compiled model's os flags. Stat/Lock use version-neutral contracts. Directory bytes retain the common little-endian Linux dirent64 layout and the existing one-batch/dummy-metadata limitation; no architecture-specific Go struct enters the model.
- Kernel typed route decoding for every filesystem service, explicit payload/boolean/narrowed-field validation, feature checks and current-epoch service selection. Existing shared read/write/close/fcntl continue routing by descriptor ownership. Generated family-specific marshalling/manifest replacement remains Task 14.
- Typed internal contracts.CrashRequest lets Sync report a before/after fault without importing kernel or widening Ports. Dispatcher consumes it before completion, cancels the real blocked caller and performs ordinary crash cleanup. Root sync faults end the run through root-last Close. Default zero fault probability consumes no fault stream.
- Mapping references keep unlinked inodes alive through descriptor close, unmap drops that reference, and Detach releases remaining mappings in creation order. Duplicate mkdir/exclusive-open now produce modeled EEXIST instead of generic errors. Unsupported arguments and negative/overflowing offsets fail explicitly.
- Capabilities inventory covers all 26 audited storage syscall names, plus fstatat/chdir/getcwd. Locks and advice remain labeled stubs; permissions/procfs routes remain missing; mappings retain the read-only/shared/zero-offset/16 MiB limit. Directory iteration and full-mapping unmap limitations are stated explicitly.

Verified
- Provider tests cover offsets and positional independence, copied write buffers, file-versus-directory durability, version-neutral stat fields, directory records, rename/unlink/open-file lifetime, permissions/flag limitations, wrong descriptor ownership, failed-open cleanup, repeated close reasons, mapping updates/unlinked lifetime/epoch cleanup, and all audited capability names.
- Real application invocations traverse the kernel dispatcher for file/path/directory/stat/allocation/locking/mapping/advice operations. Real Sync faults before and after durability preserve the expected recovered bytes, never resume the canceled caller, and never execute application defers. Root faults terminate the run and drain all invocation ownership.
- Malformed callable/extra payloads, invalid booleans and disabled filesystem calls abort explicitly. The race detector identified a test's unsynchronized fd handoff; it was fixed with an ordinary simulated channel, preserving the syscall transport's intentional absence of application happens-before edges.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task9-gate.log`. Focused race tests passed: `.gosim/task9-focused.log`. Production import audit confirms fs imports only contracts, its model and native ABI among Gosim packages; no network/kernel implementation import.

Notes / migration ledger
- Removed ops_pending.go and its Task 9 unfinished-method marker. The remaining manual decoder marker is owned by Task 14. Existing LinuxOS source remains only for the authorized intermediate full-suite entry path until Tasks 22/28; normal optional composition is not yet selected.
- CrashRequest is an implementation-local typed outcome through the already prescribed error return; it is never serialized as errno or exposed as a public control API. No target architecture or public signature was changed.
- Final Task 9 size: 1,481 changed lines (1,369 additions + 112 deletions), including checklist/log; staged whitespace check passed.

## Task 10 — Network model ownership, epochs and directed links
Date: 2026-10-19. Previous task commit: `26cf5b8`. Commit: the Task 10 implementation commit containing this entry.

Implemented
- Moved internal/simulation/network to internal/net/model, renamed its package to model, updated callers and syscall generator, and regenerated wrappers. Exact injected Network/Stack constructors and ReserveAddress/AbortReservation/Attach/Detach/ProcessIncoming/Shutdown APIs replace direct runtime dependencies.
- Deterministic staged IPv4 allocation, checked monotonic reservation/connection IDs, duplicate-owner and address collision guards, abort rollback, persistent restart identity, explicit address destruction, and stale-detach guards. Staging emits no events and consumes no random values.
- Independent ordered link states and queue invalidation; Both updates both edges, Forward only its selected edge. Explicit connectivity remains separate from traffic-triggered random loss. Preserved failure/repair draw order, exponential sampling/quantization, arrival floors and sequence ordering. Complete policies copy groups; legacy field updates validate both directions before committing or tracing.
- Owner-serialized heap delivery and stream timers through injected Events/Clock, generation-checked cancellation, destination epoch admission, connection generations and handshake tombstones. Removed model mutexes, pools, timer goroutines, package randomness and native trace calls. Readiness uses contracts.ReadinessSink.
- Quiesce retires timers/readiness and incoming packets, rearms unrelated queued destinations, and forbids application work. Graceful cleanup emits immutable close packets against surviving destination epochs; crash emits none. Cleanup sorts final close notifications by connection generation, drains the entire listener backlog, and releases buffers/maps.
- Temporary current-simulation ports forward timer callbacks through the existing syscall dispatcher; legacy polling adapts to ReadinessSink. Both carry Task 22 removal markers. Trace ports carry copied strings with named fractional policy fields. Original plan and target layout remain unchanged.

Verified
- Focused race tests passed: `.gosim/task10-focused.log`. Tests cover reservations/rollback/restart/destroy, no staging events/randomness, duplicate reservations, connection/port reuse, stale source/destination epochs, stale handshakes/callbacks/detach, graceful close after source cancellation, crash silence, queue rearming, backlog cleanup, circular-buffer wrap/window backpressure, poll notification/retirement, directed/symmetric policies, atomic invalid updates, and random-domain independence.
- Preserved fault probability/latency vectors and queue ordering/reference-retention coverage with injected deterministic events. Replaced old one-way rejection tests with directed mutation/isolation assertions and updated existing simulation callers.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed, including formatting, CLI/translation artifacts, all ordinary suites and all race suites: `.gosim/task10-gate.log`. Network tracing still preserves fractional fields, excludes payloads, and reproduces checksums/logs on replay.
- Import audit: model's only Gosim production imports are internal/contracts and rt/runtime/abi; no fs/provider/kernel implementation or native runtime dependency. Generator output is current; staged whitespace check passed.

Notes / migration ledger
- The first full gate found exact logging IDs shifted by removal of the network goroutine and the old trace JSON structure no longer matched the string-field port. Updated those assertions without dropping coverage; documented the log diff and backlog fix in NOT PLANNED CHANGES.
- Final review added duplicate-owner reservation rejection, quiescence on detach, and validation-before-tracing for symmetric updates. A transient syntax error in that edit was fixed before rerunning the final passing focused/full gates.
- New provider/session/socket/control ownership is Task 11; current-simulation adapters remain only until Task 22. No compatibility tuple is certified by this development-toolchain gate.
- Final Task 10 size: 2,792 changed lines (1,858 additions + 934 deletions) with rename detection, including checklist/log.

## Task 11 — Network provider, socket capabilities and kernel controls
Date: 2026-10-19. Previous task commit: `a329b8b`. Commit: the Task 11 implementation commit containing this entry.

Implemented
- Network Provider/Session/Prepared/Instance lifecycle with staged address/stack publication, stable session configuration, abort rollback, restart identity, deterministic cleanup and active-epoch-only SocketOps. Kernel remains unaware of the concrete network/model.
- Socket/Bind/Listen/Accept/Connect/address/options/fcntl services on the shared descriptor table. Socket implements owned Resource/Readable/Writable/Pollable; failed allocation/accept disposes untransferred resources, release is once-only and retains its reason, graceful/explicit close notifies peers, crash/abort are silent.
- Readiness bridges carry socket binding generation and epoch, map notifications to the actual fd, retire subscriptions on unsubscribe/close/quiesce, and preserve polling registered before connect/listen. Model listener finalization permits bounded cleanup after quiescence without duplicating its backlog algorithm.
- Exact NetControl values/methods with complete validation/copy semantics, before-first-start address configuration, retained addresses across stop/restart, directional policy/reset/connectivity and kernel run/state/caller guards. Added explicit manual typed network syscall routes until Task 14's generator.
- Capability evidence explicitly states IPv4-only protocol-0 TCP/nonblocking flags, fixed listen backlog, unsupported ephemeral listeners/client binds, bounded known option stubs, SO_ERROR-only reads, ENOSYS fcntl, and existing closed-read EPIPE semantics. Unknown options/families/protocols fail; no host fallback. Listener addresses report their real bound port, and SO_ERROR payloads encode Linux constants.

Verified
- Real kernel/engine provider tests cover reservations/preparation/abort/collision/retry, before-start and cross-run/stale-epoch controls, duplicate cleanup, descriptor allocation/accept failure, nonblocking data flow, peer addresses/options/unsupported arguments, readiness retirement, deadline replacement, graceful versus crash peer outcomes, and endpoint tuple reuse after restart without connection reuse.
- Real application calls traverse the dispatcher for socket creation/bind/listen/connect/accept/options/address/read/write/fcntl/close and polling. Pending connection/data waits and deadlines wake real blocked callers; malformed payloads and disabled net calls fail the run. Closed kernel retains no epochs, events, poll registrations or invocation leases.
- Real FS-then-Net address collision aborts filesystem preparation without publication or dirty peer-disk flush. Comparing equivalent engine runs with/without failed preparation verifies filesystem, network and scheduler random streams are unchanged. An initial test incorrectly copied native scheduler-owned RNG memory; replaced that inspection with the native stream API, preserving the race oracle.
- Focused race suites passed: `.gosim/task11-focused.log`. Final complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed, including all ordinary/race suites: `.gosim/task11-gate.log`. Production import audit shows net imports only contracts/model/native ABI among Gosim packages; no fs/kernel implementation dependency.
- The gate exposed a pre-existing metatest stderr lifecycle race. Added last-test-owner cleanup, serialized protocol access, bounded worker reaping and normal EOF shutdown. Ownership regression verifies a child owner cannot reap its parent's worker and the final owner waits for process/stderr completion. `TestScript/metatestlogs` passed three consecutive runs: `.gosim/task11-stderr-regression.log`. Read-only process audit found zero orphaned metatest workers in this checkout.

Notes / migration ledger
- Documented the metatest harness fix, actual bound address correction and listener finalization helper in NOT PLANNED CHANGES. Metatesting's build-identity replacement remains Task 23; its package-keyed cache is not final architecture. Existing simulated-use rejection coverage still passes.
- Ordinary CLI integration still uses the authorized intermediate simulation path. Task 14 replaces manual routes; Task 22 connects public APIs and removes temporary adapters. No target document or compatibility certification was changed.
- Final Task 11 size: 1,958 changed lines (1,913 additions + 45 deletions), including checklist/log; staged whitespace check passed.


## Task 12 — Native host ABI selection and Go 1.25 runtime hooks
Date: 2026-10-19. Previous task commit: `14bc00c`. Commit: the Task 12 implementation commit containing this entry.

Implemented
- Added bounded go125/go126 native host packages and a stable hostabi facade for opaque coroutine handles, Start/Next/Yield/Finish, upcalls, all existing race operations and primitive HostIdentity. Engine imports now use the facade. Native coroutine cancellation still skips application defers; no blocking-channel fallback is reachable through the new facade.
- Compile-time family selectors, race/norace implementations and named unsupported-family failure. Metadata remains inspectable across families; no executable opposite-family import. runtime.Version reports identity only and never chooses an implementation.
- Added 96 Go 1.25 hook manifest records covering synchronization, atomics, time, maps, random and translated stdlib race hooks, plus real native Go 1.26 coroutine metadata. Actual private declarations and reviewed source-file hashes come from installed official go1.25.14/go1.26.9 distributions. Native race-only declarations have separate origin verification. No private runtime layout was copied; coroutine pointers stay opaque.
- Profile descriptors return independent metadata, with 24 pending tuples per family and no result/source certification digests. Source fingerprints include executable adapters, shared facade/selection/spec code, ABI manifests, policy and origin files; tests and tuple evidence are excluded to avoid circular evidence identity. genprofile updates/checks only source revisions, never ABI expectations or certification.
- New atomic AND/OR hooks return the previous value in both modes. Removed four obsolete sync symbols in favor of pinned-source internal/sync declarations. Added translated notify-list size validation. Old translator hook selection remains until Task 21; full Go 1.26 translated hooks remain Task 16.

Verified
- Exact go1.25.14 and go1.26.9 native runtime/facade/metadata suites passed in ordinary and race modes on darwin/arm64: `.gosim/task12/{native125,native126,race125,race126}.log`. Coroutine tests check yield/resume, repeated disposal, skipped defers and upcalls; an isolated intentional race verifies facade annotations remain effective.
- Pinned-source verification passes for both Linux amd64/arm64 declaration selections. Wrong patch/family, absent private symbol, changed hook signature, modified GOROOT source and unsupported-release selectors fail explicitly. Fingerprint tests cover executable/policy/path changes and exclude tests, evidence and inactive-family source.
- Go 1.25 hook tests cover semaphore/notification ordering, wrong notify layout, virtual timer stop/reset/periodicity/Sleep, independent map clone, random replay and atomic results for all integer widths. Both exact toolchains' `go list ./...` skip inactive executable packages and retain metadata packages. Native import audit excludes old coro and opposite-family host code.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed, including CLI build, artifact preparation, all ordinary/race suites and script fixtures: `.gosim/task12-gate.log`. Source revision generator check and whitespace checks pass.

Notes / migration ledger
- Initial full gates exposed omitted embedded profile JSON in the script-test module copy and unflagged native metatest commands that previously selected the channel fallback. Fixed embedded-file copying/error handling and added required linkname flags to those fixtures; documented the diff in NOT PLANNED CHANGES. The final gate passes without a fallback.
- No target architecture edits or new compatibility certification. Exact toolchain enforcement remains Task 24; this development gate is not evidence for adding go1.26.0 to the supported matrix. Old coro/race packages remain for their other legacy consumers until Task 29.
- Final Task 12 size: 3,604 changed lines (3,528 additions + 76 deletions), including checklist/log; staged whitespace check passed.
- Final boundary review corrected new translated hooks to call primitive race bridges in the public rt/runtime package. Only original runtime code imports internal/hostabi; the new hook import test enforces this external-module boundary. No translated reflect/type value passes into the native race bridge. Re-ran both exact families' ordinary/race suites and the full development gate successfully after the correction.
- Regenerating unchanged revision files during an intermediate gate invalidated its mtime-based prebuilts. genprofile now preserves unchanged file mtimes; regenerated artifacts and the subsequent complete gate pass. The corrected Task 12 commit includes this bridge and generator fix.
- Corrected final Task 12 size: 3,673 changed lines (3,597 additions + 76 deletions); staged whitespace check passed.


## Task 13.1 — Versioned Go 1.25 shim surfaces and boundary checks
Date: 2026-10-19. Previous task commit: `ce3c4cc`. Commit: the Task 13.1 implementation commit containing this entry.

Implemented
- Recorded numbered Task 13.1/13.2 children before implementation: 2,855 existing shim lines plus fixes/tests/source evidence and the policy audit would exceed the 5,000-line commit cap. Parent Task 13 remains incomplete until the policy child passes.
- Copied all inventoried reflect/testing/synctest/maps/cryptorand shim files under bounded go125 tags, with no executable legacy/opposite-family imports. Native reflection/cache operations remain private inside the reflection bridge; context/time/os/testing values remain in translated packages. Original translator selection is unchanged.
- Corrected reflected map missing-key invalid Values, invalid-value deletion, map Clear, iterator reset/current-entry checks, Seq/Seq2 and early termination, comparability, negative map size, and rejection of native map/channel crossings. Map struct-field access cannot expose the native implementation pointer. Added SliceAt and retained explicit unsupported channel/dynamic-map/private-reflect2 operations; no host channel scheduling fallback.
- Updated testing.TB to Go 1.25's Context/Output/Attr/Chdir surface and corrected CoverMode's signature. Context cancellation precedes cleanup; output buffers incomplete lines; attributes validate keys/values. TempDir/Chdir/Setenv use translated os calls and restore state through cleanup, with parallel-test guards. Bench/fuzz/coverage execution remains explicitly unsupported; initialization/query responses match the simulation runner.
- Completed deterministic crypto/rand Int/Prime/Text without native entropy or upstream optional nondeterministic byte consumption. Read errors invoke a new primitive native Abort diagnostic bridge and cannot be recovered by application code; sync throw/fatal uses the same fatal path. Internal synctest unbubbled association probes remain modeled; creating/waiting for bubbles rejects explicitly.
- Added 200 exact upstream shim declarations, paired implementation-side signature validation, public reflect.Type/testing.TB interface checks, and upstream source-file hashes. Shim records remain separate from individual callable hooks because translation substitutes whole packages/type universes. Source fingerprints refreshed without certification changes.

Verified
- Exact go1.25.14 ordinary and race suites passed for all adapters/shims/native runtime: `.gosim/task13/ordinary125.log`, `.gosim/task13/focused125.log`. Source signatures, both public interface method sets and modified-signature rejection pass.
- Reflection tests exercise modeled maps inside the real engine, iteration/early exit/reset, missing/deleted entries, nil maps, independent construction, clear/deep equality, wrapped ordinary values/functions/slices, and explicit native-container rejection. Random tests cover repeatable bytes/Int/Prime/Text, supplied-reader errors and bounds; an application-machine fatal Read cannot run recovery, deferred code or its continuation.
- Testing helper tests cover context identity/cancellation-before-LIFO-cleanup, buffered output/attributes, environment/cwd restoration, temporary-directory cleanup and parallel guards. These helper tests invoke ordinary Go directly and do not claim translated fs routing. Internal synctest probes and explicit bubble rejection pass.
- Initial fixture failures were test setup issues: missing positive engine timeout, Darwin temporary-path symlink spelling, and incorrectly using the supervisor (whose cleanup defers must run) for the application-fatal oracle. Corrected each fixture; final exact-patch runs pass without weakening the assertions.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed after the final changes: `.gosim/task13.1-gate.log`. Native engine/old translator semantics and all ordinary/race/script suites remain green. Source fingerprint check, import search and whitespace check pass.

Notes / migration ledger
- Documented native reflected deletion and primitive fatal-adapter bridge under NOT PLANNED CHANGES. Old shims remain only for the authorized intermediate translator path until cutover/removal tasks. Task 13.2 owns reviewed package modes, named initialization probes, module pins, rejection fixtures and completion of the parent task; Task 21 connects new composition.
- Final Task 13.1 size: 4,188 changed lines (4,178 additions + 10 deletions), including child-task split/checklist/log; staged whitespace check passed.


## Task 13.2 — Audited Go 1.25 package and initialization policies
Date: 2026-10-19. Previous task commit: `53ed9ba`. Commit: the Task 13.2 implementation commit containing this entry.

Implemented
- Replaced new-profile skip assumptions with explicit translate/native-pure/bridge/reject policies, exact bridge symbol rewrites and trusted adapter-only native imports. Unknown standard-library packages fail review; unknown application packages stay translated. Context/time/net/io/errors/math/big retain translated identities. HTTP/gRPC/TLS/private reflect2/host entropy/native coroutine and native embed.FS boundaries reject explicitly. Corrected legacy comments that falsely claimed native networking remained intercepted; legacy selection itself remains only until Task 21.
- Classified 26 source-backed init/first-use/operation probes. Narrow standard-fd F_GETFL and RLIMIT_NOFILE discovery contracts do not enable fs/net or grant general syscall success. DNS configuration/hosts access requires fs; IP capability probing is lazy and requires net. Process clone, native bubbles and unknown operations reject. Source hashes cover every probe's callsite/declaration evidence.
- Added modeled single-processor runtime hooks and explicit finalizer/metric/pool-cleanup stub capability records. GOMAXPROCS above one aborts the application machine without recovery/deferred execution. Runtime hook source signatures come from pinned Go 1.25.14. Finalizer registration does not retain a host-GC callback.
- Pinned x/sys v0.27.0 and reflect2 v1.0.1 with module and go.mod sums; verify selected graph, reject replacements, and hash extracted source contents. No dependency upgrade was necessary. Profile verification validates policy metadata; fingerprints include the new source/metadata and preserve pending certification status.

Verified
- Exact go1.25.14 ordinary/race adapter, shim, host facade and runtime suites pass: `.gosim/task13/policy-ordinary125.log`, `.gosim/task13/policy-race125.log`. Both architecture selections' upstream signatures remain verified by existing source checks.
- Policy fixtures exercise minimal os/net/testing imports, exact symbol rewrites, source-comment native escapes, HTTP/gRPC/entropy rejection and all four feature sets' named probe contracts. These are policy/AST fixtures, not claims of completed generated-program execution; Task 21 activates the new path. Raw AST comments are required because CommentGroup.Text removes directive comments.
- Module tests verify actual pinned repository dependencies and reject wrong versions, replacement modules, modified source, substituted go.mod, duplicate pins and missing source. Temporary tampering fixtures never modify the shared module cache.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed with ordinary/race/script suites: `.gosim/task13.2-gate.log`. Source revision generator check and whitespace check pass.

Notes / migration ledger
- Task 14 must implement the classified os initialization responses, machine-local argv/environment/debug callbacks and bounded diagnostic writes. net's IPv4 capability probe passes IPPROTO_TCP=6, whereas the new provider currently accepts only protocol 0; its adapter/provider boundary must reconcile this explicitly. IPv6 remains EAFNOSUPPORT. These are adapter work already assigned to Task 14, not successful runtime behavior claimed by policy metadata.
- Later translation wiring must validate assembly/private hook requirements for translated math/big, hash and third-party packages rather than inherit broad legacy assembly exemptions. Native engine/ABI are explicit bridge islands; direct generated imports of internal/hostabi are rejected. No target edits or newly certified tuple.
- Final Task 13.2 size: 1,417 changed lines (1,311 additions + 106 deletions), including checklist/log; staged whitespace check passed.


## Task 14.1 — Stable route inventory and reproducible generator
Date: 2026-10-19. Previous task commit: `6dff8c1`. Commit: the Task 14.1 implementation commit containing this entry.

Implemented
- Recorded children 14.1/14.2/14.3 before implementation to keep generated transport/control/architecture changes within the 5,000-line cap. Original 892-line generator remains solely for the intermediate legacy ABI until Task 26, rather than rewriting live legacy output during the new-path construction.
- Added a 69-route schema covering current syscalls, shared fd ownership, polling, lifecycle, every FSControl/NetControl method, runtime timeout and run information. Explicit IDs preserve all 43 existing temporary route numbers. Legacy names have migration dispositions, not accepted aliases: partial-disk restart and auto-boot recovery become the target's separate controls/snapshots.
- Added cmd/gosim-gen and host routegen GenerateProfile with required exact toolchain/family/architecture/output inputs. Before writing, it verifies actual pinned Go executable/source declarations and selected module contents, renders all outputs, then writes atomically per file. Unchanged files keep their mtimes. Only requested architecture records and common architecture-neutral transport are written; other families/architectures are untouched.
- Generated stable route constants and typed request/result codecs in kernel. Exact payload types, empty legacy scalar slots, empty extra argument slots and typed responses are checked. Generic callbacks/pointers/any are rejected by the schema; only Create/SetMain accept the prescribed Main func(). Runtime contracts remain translated; generated source imports no concrete providers or host metadata.
- Generated amd64 source/provenance record includes transport and manifest digests, exact source-origin digest and external module pins; embedded for script-module copying and included in adapter fingerprints. The record explicitly says transport-schema, not completed adapters or certification. Child 14.2 wires typed payloads; child 14.3 generates selected Linux layouts/wrappers.

Verified
- Inventory audit checks both existing generated syscall interfaces, manual Sys* methods and every fs/net control interface method. Stable original IDs, malformed/duplicate schema/field/callback rejection, trailing/unknown JSON rejection, current-output comparison and scoped/repeated generation checks pass.
- Typed payload/result tests reject wrong route types, pointer instead of value payload, arbitrary callback, unused scalar/argument slots, missing payload, unknown route and mixed errno/result forms. Request value copies and representative fd/poll/fs/net/lifecycle/runtime payloads pass. Existing live kernel dispatcher semantics remain covered until the next child replaces the decoders.
- Exact go1.25.14 ordinary/race generator, kernel and compatibility suites passed: `.gosim/task14/schema-ordinary125.log`, `.gosim/task14/schema-race125.log`.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task14.1-gate.log`. Source revision check and whitespace check pass. No architecture-native adapter execution is claimed by this transport-schema step.
- Final Task 14.1 size: 2,530 changed lines (2,479 additions + 51 deletions), including checklist/log; staged whitespace check passed.


## Task 14.2 — Generated dispatch and per-run control transport
Date: 2026-10-19. Previous task commit: `fd98a6b`. Commit: the Task 14.2 implementation commit containing this entry.

Implemented
- Replaced temporary core/fs/net decoders with generated dispatch for all 69 typed routes. Inputs/results use the same manifest as codecs; scalar/extra slots are rejected. Filesystem and socket methods resolve guarded typed services; lock views copy neutral Lock values in/out, and unknown/non-errno syscall errors remain fatal invariants.
- Added all lifecycle/fs/net/runtime control routes, including epoch-capturing machine waits, non-returning root/child self-exit, retained recovery handles and separate run checks for restored snapshots. Pending poll and machine completions now return their generated result types. Crash-on-sync still cancels the invocation without waking the caller.
- Added internal/control CurrentRef/Submit/Bind. Supervisor-only installation occurs before root startup; transport replaces caller-supplied context with the current run/epoch and relies on dispatcher admission. Teardown releases the binding, subsequent runs get fresh identities, and application rebinding cannot mutate it. No provider imports or supplied trampoline.
- Added reviewed SharedGlobals metadata for the single translated control binding only. Task 21 will apply this selected-profile record during actual translation; the binding is not stored in native engine state. Existing translator selection remains unchanged until the planned cutover.
- Added copied run information, kernel label/state lookups, and real virtual timeout replacement. Engine owns/resets/releases its timeout timer; invalid/overflow deadlines fail without changing the active deadline. Composition will supply runtime revision and certification metadata with the existing identity.

Verified
- Existing kernel filesystem/network/race/wait/cancellation tests now construct generated typed requests and decode typed results; previous assertions remain. No test-only production compatibility decoder was retained. The intentionally racing child fixture was also moved to the new Getpid payload.
- Control tests exercise create/configure/start/stop/crash/restart/set-main/wait/destroy, every fs/net control route, retained snapshots, wrong-run/stale handles, context stamping, run-info copy isolation, failed application rebind, cleanup and repeated runs. Root and child self-exit skip caller defers/continuations; child exit preserves its controller. Ignored errors/recovery cannot evade disabled-feature, forged-payload or unknown-route aborts.
- Focused development race suite passes, including the application-race oracle. Initial races identified instrumented envelope validation, compiler-generated array comparison and copied run-info backing arrays, plus a native upcall result variable. Used bounded metadata-only norace handling, scalar iteration and kernel-owned copy construction; application memory views and the oracle remain instrumented. The virtual timeout test verifies extension beyond the original deadline, rejection of invalid resets and a shorter subsequent deadline.
- Exact go1.25.14 ordinary/race transport, kernel, runtime, generator and compatibility suites passed: `.gosim/task14/transport-ordinary125.log`, `.gosim/task14/transport-race125.log`. Repeated generation/source checks pass.
- Complete `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed with ordinary/race/script suites: `.gosim/task14.2-gate.log`. No target architecture edits or new certification. Child 14.3 still owns Linux adapter layouts and initialization responses.
- Final Task 14.2 size: 2,111 changed lines (1,592 additions + 519 deletions), including checklist/log; staged whitespace check passed.


## Task 14.3.1 — Go 1.25 initialization and adapter transport foundation
Date: 2026-10-19. Previous task commit: `3aa84b2`. Commit: the Task 14.3.1 implementation commit containing this entry.

Implemented
- Split 14.3 into buildable children before implementation to keep each commit below 5,000 changed lines. Added version-owned process/environment/debug/poll adapters and exact upstream hook/source/capability records. Process state is translated and machine-local; boot configuration and isolation become reachable through composition in Task 21. Current build debug defaults are empty; no host argv/environment/debug state is inherited.
- Added stable route 70 Require, used before optional-domain pointer marshalling/local validation. It validates the feature value and uses existing checked control transport; no new central entry point or provider import. Standard live descriptors accept only F_GETFL with zero argument, returning modeled access flags. Net sockets accept explicit TCP protocol 6 as well as 0; IPv6 and UDP remain unsupported.
- Poll hooks use actual uintptr handle signatures from pinned Go 1.25.14, with no heap pointer disguised as an integer. Checked integer kernel registrations cover wait/reset/deadline/unblock/close; nonpollable open maps to EPERM and invalid descriptors preserve EBADF. Debug callbacks receive real local updates, diagnostic writes use kernel descriptors, and metric registration retains no callback.

Verified
- Exact go1.25.14 ordinary adapter/source/generator tests pass: `.gosim/task14/init-tests.log`; affected ordinary control/kernel/net suites passed during iteration. Exact-patch race suites pass: `.gosim/task14/init-race.log`, including existing application race oracles.
- Static Linux amd64 Go 1.25.14 adapter test binary executed successfully via Docker Desktop emulation: `.gosim/task14/init-amd64.log`. This proves the process/poll/kernel adapter probes, not the pending architecture-specific syscall wrappers, translated composition, or certification matrix. Colima context remains unavailable; final matrix still belongs to Task 32.
- Ordinary native tests share the process global that translation will isolate. Aborted fixture coroutines skip defers; explicit test-only race acquire/release publishes reset state across sequential runs. Production process state retains race instrumentation. Source-origin updates preserve native private-declaration verification records.
- Complete development `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task14.3.1-gate.log`. Regeneration, source fingerprint check, formatting and whitespace checks pass. Child 14.3.2 retains the full amd64 layout/wrapper/init-resource-limit requirements; no parent completion or certification claimed.
- Final Task 14.3.1 size: 728 changed lines (718 additions + 10 deletions), including checklist/log; staged whitespace check passed.


## Task 14.3.2 — Complete Go 1.25 amd64 syscall adapters
Date: 2026-10-19. Previous task commit: `197eaca`. Commit: the Task 14.3.2 implementation commit containing this entry. Completes Tasks 14.3 and 14.

Implemented
- Generate 93 explicit syscall/x/sys/internal-unix wrappers across kernel/fs/net from selected Go 1.25.14 and pinned x/sys declarations. Expected hook signatures remain reviewed profile inputs; generation does not silently update expectations. Complete legacy generated-wrapper coverage is independently checked until Task 26 removes those files.
- Generate both standard-library and x/sys Linux record layouts and reviewed syscall numbers. Struct spelling, scalar widths, padding, pointer-free layout and required declarations are checked before output. Linux getrandom's number comes from internal/syscall/unix because the frozen syscall package lacks SYS_GETRANDOM. Output records now identify typed-adapters and include the architecture-specific adapter digest.
- Marshal neutral stat/lock/address records, bounded socket lengths, network-byte-order ports, diagnostic uname fields and typed borrowed byte views. Optional-domain gates precede pointer reads/local validation; integer fcntl rejects pointer commands and pointer locks accept only the reviewed POSIX commands. Raw calls reject, even for a known number, with no native syscall fallback. RLIMIT_NOFILE returns ENOSYS without touching output; other resources reject.
- Capability records identify inherited lock/advice/socket-option stubs and limited semantics. GETLK intentionally preserves the existing no-op lock record; it does not invent lock ownership or unlock results. Undersized IPv4 output buffers explicitly return EINVAL before descriptor mutation. New adapter packages contain no concrete provider imports.

Verified
- Exact go1.25.14 ordinary/race host compatibility, source, generator, kernel/control/provider suites pass: `.gosim/task14/adapters-ordinary125.log`, `.gosim/task14/adapters-race125.log`. Missing declarations, mutated pointer layouts, changed syscall numbers/signatures, provider imports and missing first-operation feature gates are rejected. Complete generation is byte-identical, preserves unchanged mtimes and leaves unselected family/architecture outputs alone.
- Linux amd64 ordinary probes pass through Docker Desktop emulation: `.gosim/task14/amd64-execution.log`. Exact `go version go1.25.14 linux/amd64` race probes pass in the requested amd64 golang:1.25.14-bookworm container: `.gosim/task14/amd64-race.log`. Tests use actual syscall/x/sys stat and flock pointer types through ABI linknames, file operations/mapping/dirents, TCP bind/connect/accept/address/options, resource discovery, nil/length errors and disabled/raw fatal boundaries.
- Borrowed-memory race oracle detects the intentional application store versus adapter read. Its initial stack assertion assumed an uninlined wrapper name; the runtime reports racereadrange at the caller after inlining. The assertion now requires that range read and the application writer, preserving the actual race requirement. No production race suppression was added.
- Complete development `GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task14.3.2-gate.log`. Formatting, source revision and whitespace checks pass. These are adapter executions, not yet translated-program wiring or certification; Task 21 wires composition, Task 32 owns the required Colima/host matrix. Target architecture unchanged.
- Final Task 14.3.2 size: 3204 changed lines (3180 additions + 24 deletions), including checklist/log; staged whitespace check passed.


## Task 15 — Go 1.25 arm64 adapters and native Darwin layout verification
Date: 2026-10-19. Previous task commit: `2ced221`. Commit: the Task 15 implementation commit containing this entry.

Implemented
- Added separately source-derived Linux arm64 layouts and kernel/fs/net wrappers, retaining identical logical hook/capability sets across architectures. Verify the complete arm64 source declarations, field widths and syscall numbers; Go's FSTATAT and x/sys NEWFSTATAT both name arm64 syscall 79. Stat.nlink narrowing explicitly aborts on overflow before output writes. Source-origin/fingerprint and architecture adapter records include the new outputs.
- Shared the existing Linux ABI probes across amd64/arm64, preserving actual syscall/x/sys pointer-type linkname tests and the application-memory race oracle. Added arm64 maximum/overflow and negative timestamp marshalling tests, executed on Darwin as well as Linux. No copied Darwin syscall layout or provider import enters the adapters.
- Added a native Darwin probe that reads generated Linux arm64 layout source, applies the existing translator's build-constraint/filename rewrites and compiles/executes it with exact go1.25.14 on Darwin arm64. This verifies that source-selection boundary; complete new-path translation/composition remains Task 21.

Verified
- Exact Go 1.25.14 Linux arm64 ordinary/race adapter executions pass through Docker Desktop: `.gosim/task15-linux-ordinary.log`, `.gosim/task15-linux-race.log`; container reports `go version go1.25.14 linux/arm64`. Darwin ordinary compatibility suites pass in `.gosim/task15-host-ordinary.log`; affected race/ABI/generator/native-compilation probes pass in `.gosim/task15-host-race-retry.log`. Both hook/shim package sets compile with required linkname flags.
- Architecture parity, changed-arm64-layout/number rejection, complete generated-output checking and arm64-only repeatability/scoping pass. `.gosim/task15-scope.log` verifies arm64 generation preserves existing amd64/common bytes and unchanged mtimes. The common gate also runs these generator checks.
- First host race attempt exhausted the internal disk's temporary storage. Its empty temporary build directories had already been removed by Go. Redirected this task's TMPDIR and GOCACHE into `.gosim/host-tmp` and `.gosim/host-go-build` on the workspace volume; no user files/shared cache were deleted and no permanent environment setting changed.
- The subsequent full rebuild exposed a pre-existing legacy translator cold-cache bug: synthetic package.test import paths were requested as real packages. Fixed uncached reload selection to use classifyPackage's base path; recorded the small diff under NOT PLANNED CHANGES. The complete ordinary/race/script/translation rebuild then passed: `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1`, `.gosim/task15-gate-retry.log`. Do not reuse the failed first-attempt logs as evidence.
- Source fingerprint, formatting and whitespace checks pass. No target architecture changes or certification promotion; final Colima/host matrix remains Task 32.
- Final Task 15 size: 1618 changed lines (1551 additions + 67 deletions), with rename detection and checklist/log included; staged whitespace check passed.


## Task 16 — Go 1.26 runtime hooks and source policies
Date: 2026-10-19. Previous task commit: `e3da8a7`. Commit: the Task 16 implementation commit containing this entry.

Implemented
- Added independently owned, bounded Go 1.26 synchronization/atomic/time/map/random/race hooks and runtime policy records. Verified 99 runtime hook declarations against exact go1.26.9 Linux amd64 and arm64 source, recording source hashes and native private declarations. All 24 Go 1.26 tuples remain pending; full shims/init audit and syscall adapters remain Tasks 17–19.
- Recorded the actual runtime-hook signature delta: sync.runtime_StoreReluintptr returns uintptr in Go 1.25.14 and no result in Go 1.26.9. The other reviewed runtime hook signatures match. Timer/Ticker retain their channel/bool prefix; native layout tests check the prefix without copying native timer internals. Notify-list state remains the engine-owned translated replacement.
- Corrected ReadObjectPC/WriteObjectPC in both family adapters/facades: use one opaque native type pointer and forward to the selected runtime's pushed symbols, retaining composite range semantics. Both pinned versions already push those symbols; tests check source directives and runtime execution. Added missing Go 1.25 maps translation policy after comparing both exact source sets; recorded these incidental corrections in the plan.

Verified
- Exact go1.25.14 and go1.26.9 ordinary/race hook, shim (where implemented), native facade, engine and source-policy suites passed: `.gosim/task16/ordinary125.log`, `ordinary126.log`, `race125.log`, `race126.log`. New object read/write oracles detect races on the last array element through a separately declared single-pointer ABI. Store-result mutation, missing-symbol, unknown bridge/stdlib and unsupported-family rejection checks pass; family-owned executable imports remain isolated.
- The first new family-ownership assertion incorrectly excluded the existing Go 1.25 native coroutine target; corrected it to require the owning family for both host and translated hook targets. Final logs above contain successful reruns. Native source origins, typed Go 1.25 generator output and adapter revisions are current.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task16-gate.log`, including ordinary, race, script and translation suites. Formatting and whitespace checks pass. No target architecture changes; this is adapter evidence, not the pending composition wiring or final certification matrix.
- Final Task 16 size: 2,615 changed lines (2,575 additions + 40 deletions), including checklist/log; staged whitespace check passed.


## Task 17.1 — Go 1.26 shim surfaces and semantic deltas
Date: 2026-10-19. Previous task commit: `9b6a991`. Commit: the Task 17.1 implementation commit containing this entry.

Implemented
- Split Task 17 before implementation into shim and policy/init children to retain reviewable commits below 5,000 changed lines. Added independently bounded Go 1.26 reflect/testing/maps/synctest/cryptorand sources, preserving wrapped native metadata boundaries, explicit unsupported operations and deterministic random streams.
- Audited exact Go 1.26.9 shim declarations and source changes. Added Type.Fields/Methods/Ins/Outs and Value.Fields/Methods using wrapped indexed operations, preserving eager kind checks, bound methods and early termination. The native reflect implementation owns its changed internal type/map layouts; these are not copied across the boundary. Internal synctest source is unchanged between the pinned patches; its native bubble execution remains explicitly unsupported.
- Added testing.ArtifactDir's default temporary-directory mode with per-test reuse, independent test directories and cleanup; TempDir honors translated GOTMPDIR. Persistent -test.artifacts output is outside the simulation runner's supported flag surface and will be recorded as a limited capability in 17.2, not silently described as implemented persistence.
- Go 1.26 Prime now ignores the supplied reader by default and uses the deterministic application stream independently of mutable crypto/rand.Reader. Translated GODEBUG=cryptocustomrand=1 selects the supplied reader; the last setting wins. Int retains its supplied-reader behavior, Read retains fatal reader-error semantics, and no host entropy enters any shim. The new crypto/internal/rand and testing/cryptotest boundaries belong to 17.2's policy audit.

Verified
- Exact go1.26.9 and go1.25.14 ordinary/race shim, hook and source-manifest suites pass: `.gosim/task17/shims-ordinary126.log`, `shims-race126.log`, `shims-ordinary125.log`, `shims-race125.log`. Manifest verification checks the implementation and upstream signatures; reflected Type/testing.TB public interfaces match their pinned families. Invalid signature/native container, deterministic read/prime/text, iterator mutation/binding/early stop, artifact reuse/isolation/cleanup and custom-reader tests pass.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task17.1-gate.log`, including ordinary/race/script/translation suites. Go 1.25 sources and fingerprints are unchanged. Source revision, formatting and whitespace checks pass.
- Parent Task 17 remains open for policy/init audit. No target architecture changes, composition activation or certification promotion.
- Final Task 17.1 size: 4,316 changed lines (4,308 additions + 8 deletions), including checklist/log; staged whitespace check passed.


## Task 17.2 — Go 1.26 package and initialization policy audit
Date: 2026-10-19. Previous task commit: `a5f43a1`. Commit: the Task 17.2 implementation commit containing this entry. Completes Task 17.

Implemented
- Added independently owned Go 1.26 package/bridge/native-import/probe/capability/module/shared-global records. Reused classifications only after comparing selected source and dependency changes. Exact compiler/native islands remain separate from application permissions; unknown stdlib and native symbols fail closed, and private native import permissions are exact owner/package pairs.
- Audited Linux-selected dependencies of os/net/fmt/context/sync/time/random/hash/reflect/testing/maps and native pure helpers. No dependency in those roots lacks a recorded policy. Go 1.26 replaces internal/itoa and internal/runtime/strconv with translated internal/strconv; raw runtime syscalls move to rejected internal/runtime/syscall/linux. Native GC scan and pprof-label internals reject. Crypto moves entropy into versioned/FIPS packages and introduces crypto/internal/rand; the whole crypto/rand shim remains the sole supported entropy surface. testing/cryptotest.SetGlobalRandom explicitly rejects its process-wide native DRBG mutation.
- Reviewed native-pure deltas: cmp and utf16 unchanged; math/bits and UTF-8 changes remain primitive computations; netip adds pure Prefix.Compare and replaces integer formatting without exposing handles/I/O. Reviewed unique cache change is diagnostic text only. Added exact-patch source hashes for these boundaries and named initialization probes.
- Confirmed resource-limit, standard-fd, poll, debug registration and lazy network/resolver probe classifications. Added guarded ArtifactDir and lazy copy-file-range probe records, plus the new runtimeClearenv(map[string]int) operation. Task 18 now explicitly owns implementing this new Go 1.26 process hook; it must keep the environment map translated. Persistent artifact export, native crypto overrides, unsupported process handles and existing syscall stubs remain visible capabilities.

Verified
- Extended existing package/bridge/init/negative-fixture/native-import tests to both families. Go 1.26 delta tests verify new package rejection, integer-helper translation, clearenv classification, artifact fs gate, unknown symbols and exact native-import ownership. Probe records have pinned source evidence and modules match the reviewed dependency graph. Initial capability validation caught a duplicate Prime record; replaced the prior record with the reviewed Go 1.26 semantics before final verification.
- Exact go1.25.14/go1.26.9 ordinary and race compatibility suites pass: `.gosim/task17/policy-ordinary125.log`, `policy-ordinary126.log`, `policy-race125.log`, `policy-race126.log`. Source and dependency audit outputs are retained under `.gosim/task17/`; no Go 1.25 source/fingerprint changes.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task17.2-gate.log`. Revision regeneration/check, formatting and whitespace checks pass. Adapter binding remains Task 18, full translated composition Task 21, certification Task 32; no target architecture changes or tuple promotion.
- Final Task 17.2 size: 1,904 changed lines (1,392 additions + 512 deletions), including checklist/log; staged whitespace check passed.


## Task 18 — Go 1.26 amd64 syscall, process and poll adapters
Date: 2026-10-19. Previous task commit: `566acc1`. Commit: the Task 18 implementation commit containing this entry.

Implemented
- Extended the host generator to select each family's metadata, source verification and bounded output independently. Generated Go 1.26 amd64 kernel/fs/net wrappers and Linux layouts from exact go1.26.9 plus pinned x/sys source. All 93 syscall declarations retain their reviewed signatures/layouts; the shared logical routes and neutral layout expectations are validated against the selected source, not assumed from the earlier patch. No executable cross-family import or provider import enters generated adapters.
- Added family-owned process/poll/control bindings and the new syscall.runtimeClearenv(map[string]int) hook. The caller map remains in translated code and is not mutated; process/debug state is machine-local, host environment remains untouched, and GODEBUG notification occurs only when its key was cleared. Pinned Linux runtime source documents this new behavior. Source manifest, module pins and complete generated adapter/transport digests are current.
- Retained first-operation optional-domain gates, neutral pointer marshalling, errno/capability classifications, explicit raw-call rejection, borrowed-memory annotations and actual uintptr poll handles. Added complete Linux amd64 probes with upstream stat/flock types through ABI linknames. Arm64 generation/output execution remains Task 19.

Verified
- Exact go1.26.9 ordinary/race host compatibility and generator suites pass: `.gosim/task18/ordinary126.log`, `race126.log`. Exact go1.25.14 compatibility/generator regression suite passes: `ordinary125.log`. Generation checks cover current outputs, signature mutation rejection, unchanged mtimes, other-family protection, absent unselected architecture, forbidden imports and feature gates before marshalling. Existing Go 1.25 generated bytes remain unchanged.
- Linux amd64 ordinary/race adapter and runtime-hook tests passed with actual `go version go1.26.9 linux/amd64`: `.gosim/task18/linux-amd64.log`. Used Docker Desktop's existing golang:1.25.14-bookworm container OS/tooling with an explicitly mounted official Go 1.26.9 Linux amd64 toolchain and GOTOOLCHAIN=local. Docker server reports aarch64 (`docker-server-arch.log`); this is emulated execution. Full final matrix still requires Colima and its own exact image/evidence records.
- Downloaded the selected Linux toolchain into workspace-local `.gosim/linux-toolchains` and used workspace build caches to avoid the nearly full internal disk. A direct module download stores tools without execute bits; first execution reported permission denied, then execute bits were set on this task-owned toolchain's bin/pkg-tool files, without changing source bytes. The final Linux log records the successful ordinary/race execution, including syscall-layout, filesystem/TCP, invalid/disabled/raw-call, poll/process and application-memory race oracles.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task18-gate.log`. Formatting, revision and whitespace checks pass. No target edits or compatibility promotion; new composition remains Task 21.
- Final Task 18 size: 2,989 changed lines (2,950 additions + 39 deletions), including checklist/log; staged whitespace check passed.


## Task 19.1 — Go 1.26 arm64 generation and native host probes
Date: 2026-10-19. Previous task commit: `d511a39`. Commit: the Task 19.1 implementation commit containing this entry.

Implemented
- Split Task 19 before implementation into arm64 execution and complete hook/shim inventory closure. Generated Go 1.26 arm64 layouts and kernel/fs/net adapters from the selected source; all four family/architecture outputs now have current-output checks. Shared Linux ABI tests across both Go 1.26 architectures; added native Darwin arm64 stat/number, timestamp and nlink-overflow probes.
- Extended the Linux-source-to-native-Darwin compilation probe to both exact families using the real translator filename/build-constraint rewriting. It verifies Linux arm64 layout in each native Darwin executable; complete translated-program composition remains Task 21.
- Fixed the legacy translation golden test's shared prebuilt fixture dependency after the exact Go 1.25 run exposed a missing-family host package in the extracted module. Each test now builds its own matching CLI with the test compiler, pins loader PATH/GOTOOLCHAIN and uses full repository source. Confirmed the shared extraction had go126/host and no go125/host. Recorded this incidental test isolation fix in the plan; no golden expectations or production runtime behavior changed.

Verified
- Exact go1.26.9 Linux arm64 ordinary/race adapter and runtime-hook execution passed: `.gosim/task19/linux-arm64.log`. Mounted the official Linux arm64 toolchain into the existing Debian Go 1.25 image with GOTOOLCHAIN=local, as in Task 18; actual executable reports go1.26.9 linux/arm64 on the aarch64 Docker Desktop server. This is native architecture execution within the VM, not Colima certification.
- Darwin Go 1.26.9 ordinary and race compatibility, source, four-output generation, native ABI and layout checks passed: `.gosim/task19/host-ordinary126.log`, `host-126.log`. Go 1.25 compatibility/generation/host-ABI checks passed in `host-125.log`; that initial log's translator failure is not passing evidence. After fixture repair, exact Go 1.25 ordinary and Go 1.26 race translator suites passed with `-count=1`: `translate-125-retry.log`, `translate-126-retry.log`. Both include the two pinned native Darwin probes.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task19.1-gate.log`, with full ordinary/race/script/translation suites. Formatting, source revision and whitespace checks pass. Parent Task 19 remains open for declaration inventory closure; no target changes or compatibility promotion.
- Final Task 19.1 size: 1,392 changed lines (1,311 additions + 81 deletions), with rename detection and checklist/log; staged whitespace check passed.


## Task 19.2.1 — Complete computational and unsupported private hooks
Date: 2026-10-19. Previous task commit: `31aa140`. Commit: the Task 19.2.1 implementation commit containing this entry.

Implemented
- Split inventory closure before implementation into remaining adapters and a durable declaration/route ledger. Added independently bounded family-owned byte/string helpers, translated ChaCha8 with explicit four-block interlacing/packing, and net's application-stream random hook. No production native bytes/strings or ChaCha assembly bridge is introduced. MakeNoZero's zeroed allocation is a named limited capability.
- Added explicit non-recoverable rejection adapters for private CPU/function ABI, memory-equality ABI, process/fork/exec/thread/cgo and VDSO operations. They abort before inspecting pointers; CPU feature globals remain zero because native runtime CPU initialization is not translated application initialization. These wrappers preserve compilable stdlib bodies without host fallback or invented CPU features.
- Added architecture-specific hook manifests and DescriptorForArch, preserving the target Profile/Hook schemas. Each family's common declarations plus selected amd64/arm64 declarations are verified against exact Linux-selected source. The private gettimeofday entry exists only on amd64 in syscall and pinned x/sys; arm64 continues through its rejected raw syscall. Updated source provenance and regenerated all four route records.
- Removed native coro entries from both callable hook manifests; their signatures remain in native origin verification and hostabi ownership. The new architecture test prohibits any callable hook target outside the selected translated hooks tree.

Verified
- Exact go1.25.14/go1.26.9 ordinary and race compatibility/generator suites pass: `.gosim/task19/completion-125.log`, `completion-126.log`, `completion-race125.log`, `completion-race126.log`. Byte/string tests compare varied binary/UTF-8/empty/overlapping-prefix inputs with stdlib; ChaCha8 compares 64 seeds and six counters (including wraparound) with the selected native runtime. Tests verify application random-stream identity and every new fatal operation cannot return or recover.
- Both exact families pass ordinary/race runtime and kernel hook execution on Linux amd64 and arm64: `completion-linux-amd64.log`, `completion-linux-arm64.log`. Existing Docker Desktop image/toolchain arrangement matches Tasks 18/19.1; amd64 is emulated, arm64 native within the VM, and Go 1.26 uses its explicit official mounted toolchain. These are adapter execution results, not Colima certification or completed translated composition.
- Initial generation tests correctly reported stale route-origin digests after the added source provenance; regenerated both families before successful final tests. Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task19.2.1-gate.log`, including ordinary/race/script/translation suites. Revision check and whitespace check pass. No target edits or compatibility promotion; Task 19.2.2 remains open.
- Final Task 19.2.1 size: 1,692 changed lines (1,627 additions + 65 deletions), including checklist/log; staged whitespace check passed.


## Task 19.2.2 — Complete declaration and route accounting
Date: 2026-10-19. Previous task commit: `e30a842`. Commit: the Task 19.2.2 implementation commit containing this entry. Completes Tasks 19.2 and 19.

Implemented
- Added a durable 964-declaration hook/shim baseline ledger with immutable source paths/signatures and a checked digest. Dispositions: 592 hooks, 308 retained private/shim bindings, 44 package rejections, 12 target replacements and eight moved-hook records. Original go123/go125 and architecture copies remain individually accounted for. Tests expand both selected families and architectures and verify exact package/receiver/type ownership under Linux source constraints; they do not depend on legacy files or ignored brainstorm documents.
- Verified old sync mutex declarations moved to internal/sync in both pinned families, and runtimeClearenv is absent in Go 1.25 but implemented in Go 1.26. Reviewed native ChaCha linkname and runtime-entry replacements, whole-package exclusions and architecture-only CPU/VDSO entries. Exact source inspection corrected the preliminary assumption that os.ignoreSIGSYS/restoreSIGSYS were obsolete: both still exist around pidfd probing, so added fatal kernel adapters and tests for both families.
- Added named existing shim limitations: private reflect2 and StructOf rejection, no-bubble synctest probes, and testing init/coverage/verbosity behavior. Added bounded-family source-selection checks, target service-route ownership checks and dispatcher coverage for every recorded route. Existing generated ABI/layout/source/behavior tests remain in place.
- Added the planned read-only generator check mode and corrected validation commands to actual explicit flag names. Tests cover unchanged mtimes, stale/missing outputs, failure preservation and explicit destination. No target changes or compatibility promotion; composition/public cutover and certification remain later tasks.

Verified
- Exact go1.25.14/go1.26.9 ordinary and race compatibility, four-output generation and hostabi suites pass: `.gosim/task19/ledger-125.log`, `ledger-126.log`, `ledger-race125.log`, `ledger-race126.log`. These include unsupported-family compilation rejection. Both pinned CLI `gosim-gen -toolchain=<patch> -profile=<family> -arches=amd64,arm64 -output=. -check` commands pass (`check125.log`, `check126.log`). Task 19.1 and 19.2.1 retain actual Linux amd64/arm64 and Darwin adapter execution evidence.
- Initial full gate failed when macOS killed prebuilt ordinary metatest workers (`task19.2.2-gate.log`). codesign verified their bytes; launching a fresh byte-identical copy succeeded while the existing inode exited -9. Fixed legacy copy publication to create/chmod/copy/close/rename a fresh file, with regression checks for open-inode preservation, unchanged mtimes and failed-copy cleanup. Refreshed the two known-bad task-owned metatest artifacts without changing bytes.
- First retry passed those tests but exposed the same stale-inode fault in the native race reference fixture (`task19.2.2-gate-retry.log`); its fresh identical copy also ran. Refreshed that fixture, then uncached TestRaceOriginal passed (`native-race-retry.log`). Expected simulated race reports were correctly classified by their outer harness; none were waived.
- Final complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task19.2.2-gate-final.log`, including ordinary/race/script/translation suites. Source revision, formatting and whitespace checks pass. Incidental executable-copy fix and generator-command correction are recorded in the plan's NOT PLANNED CHANGES.
- Final Task 19.2.2 size: 1,753 changed lines (1,733 additions + 20 deletions), including checklist/log; staged whitespace check passed.


## Task 20 — V2 translation cache and package-specific artifacts
Date: 2026-10-19. Previous task commit: `8efa76d`. Commit: the Task 20 implementation commit containing this entry.

Implemented
- Added exact CacheEntryV2 and PackageSelector-based result serialization using gzip/gob. Keys include the gosim-translation-v2 namespace, canonical complete identity, package/test identity, selected CompiledGoFiles and non-Go/embed contents, module/replacement metadata and go.mod contents, and sorted dependency hashes, all length-delimited. Read failures return errors; no new-path log.Fatal. Decoding consumes the gzip checksum/footer and rejects trailing data, wrong format/digests and unsafe output paths. Corrupt entries produce a diagnostic, delete safely and become misses; old-format keys are not consulted.
- Added DB.Delete and locked Cache.Delete, retaining immutable inserts and the sole unchanged STRICT SQLite table. Get/Put/Clean now compute the coarsened current hour per operation instead of retaining the construction hour; seven-day last-use retention remains unchanged.
- Added canonical manifest records, content/identity/package-set verification and `.gosim/v2/<identity>/packages/<package-build-digest>/` directories. Binary/generated hashes, executable permissions and complete recorded file sets are checked. Candidate/certified identities and separate package names cannot collide. Paths reject traversal, file/directory conflicts, reserved manifest collisions and symlink escapes.
- Added private process-unique staging, per-artifact OS file locks and validated atomic directory publication. Complete valid artifacts are immutable/reused; a failed build preserves the previous output. Corrupt outputs are replaced only after a new complete artifact validates, with rollback if publication fails. Different packages under one identity build independently. Filesystem I/O errors are surfaced rather than silently replacing unreadable output.
- Kept the active v1 translator on explicitly named migration helpers until Task 21 connects immutable selection, cache and compiler artifacts. Mechanical PackageSelector changes include its legacy generator template, keeping regeneration coherent. No target edits, public cutover or compatibility promotion.

Verified
- Cache tests cover hits/misses, immutable insert, corrupt gzip/truncation/checksum/trailing/old-envelope/old-result data, wrong digests, unsafe output names, observable deletion/replacement, old-key isolation and SQLite I/O failures. Hash tests cover every identity field, candidate promotion, canonical order, length boundaries, selected source/non-Go/embed/go.mod content changes with unchanged mtimes, ignored unselected source, dependencies, module metadata and test-package identity.
- Artifact tests verify manifests and hashes, unknown/trailing JSON rejection, independent package slots (including equal content), failed builder/invalid manifest preservation, corrupt-output repair, staging cleanup, executable permissions, traversal/parent/root/lock symlink rejection, twelve concurrent callers with one build, concurrent different-package builds and two real processes sharing one build lock. Database tests verify the unchanged sole STRICT table, hourly touches, seven-day cleanup, idempotent deletion and replacement.
- Exact go1.25.14/go1.26.9 host ordinary/race suites for gosimtool, translator/cache and CLI pass: `.gosim/task20-125.log`, `task20-126.log`, `task20-race125.log`, `task20-race126.log`. Both pinned families also pass artifact/SQLite ordinary and race execution on Linux amd64 and arm64: `.gosim/task20/linux-amd64.log`, `linux-arm64.log`, using the existing Docker Desktop arrangement; amd64 is emulated, and this does not replace the final Colima matrix.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task20-gate.log`, with ordinary/race/script/translation suites. Formatting, revision and whitespace checks pass. The executable publication fix from Task 19 remained green.
- Final Task 20 size: 1,856 changed lines (1,707 additions + 149 deletions), including checklist/log; staged whitespace check passed.


## Task 21.1 — Immutable profile policies and source boundaries
Date: 2026-10-19. Previous task commit: `d7513d9`. Commit: the Task 21.1 implementation commit containing this entry.

Implemented
- Split Task 21 before implementation into four buildable children: policy validation, composition/Driver generation, translator/cache wiring, and real candidate execution. Retained every parent requirement and verification; the split keeps each implementation commit below the authorized 5,000-line cap. No target changes.
- Added selected.Resolve against the compile-time family and architecture-specific descriptor. It reselects the declared tuple, checks the complete canonical Selection (including hook digest, providers and runtime packages), validates owned policy metadata and returns independent values. A supplied identity cannot switch executable adapter families.
- Added Policies with owned selected hook targets, package/symbol replacements, explicit native bridges and shared control binding metadata. ValidateBoundary checks typed imports and symbol uses, including dot imports, method receivers and function values; rejects unsupported/native/cgo/retired/other-family boundaries, unauthorized notranslate directives, unreviewed native linknames and bodyless assembly entries. Concrete provider imports are restricted to the identity-specific generated composition path or their own implementation tree.
- Standard-library references to exact reviewed fatal hooks remain compilable; application uses of rejected capabilities fail. Bodies replaced entirely by hooks are not treated as surviving native calls. Existing legacy execution remains active until the remaining Task 21 wiring and Task 22 cutover; this commit does not claim new-path execution or certification.

Verified
- Exact go1.25.14/go1.26.9 ordinary and race translator, selection and compatibility suites pass: `.gosim/task21.1-125.log`, `task21.1-126.log`, `task21.1-race125.log`, `task21.1-race126.log`. Tests cover all feature sets, both simulation architectures, altered selection/profile/patch/revision/digest/provider fields, metadata ownership, typed native boundary rejection, reviewed shim exemptions and provider dependency direction.
- Initial focused tests exposed diagnostic expectations that were less specific than the existing named capability errors; corrected those expectations. Initial full compatibility checks correctly caught stale source fingerprints after changing selected metadata; regenerated both revisions before successful checks. No source ABI expectations or tuple evidence changed.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task21.1-gate.log`, including ordinary/race/script/translation suites. Formatting, source revision and whitespace checks pass. Parent Task 21 remains open.
- Final Task 21.1 size: 732 changed lines (729 additions + 3 deletions), including checklist/log; staged whitespace check passed.


## Task 21.2 — Optional composition and generated Driver
Date: 2026-10-19. Previous task commit: `ba59b31`. Commit: the Task 21.2 implementation commit containing this entry.

Implemented
- Added Generate/ValidateImports and GeneratedComposition. The host generator imports metadata only; rendered source includes selected hooks/shims and only enabled concrete provider constructors. Generation validates canonical selection before I/O, publishes one complete source file atomically/exclusively, reuses identical files without changing inode/mtime, and rejects changed source, extra Go files, symlinks or malformed output records.
- Generated Driver checks the exact native HostIdentity, creates one Engine, opens kernel/providers inside its supervisor, binds control before published root initialization, delegates test entry to the selected shim, and closes kernel/control before Engine.Close. Preserved the existing ten-minute default timeout. Run identity/certification is stamped from selection, never promoted.
- Added native engine ConfigureRun/RunResult capture and a primitive HostIdentity bridge. Runner inputs and outputs are copied; logs/checksums survive cleanup; trace flags are applied only under the single-run guard and restored afterward. Translated cleanup errors are formatted while the supervisor remains active.
- Fixed resolved directory opening in the filesystem model after the generated probe found that `/` returned ENOENT through an empty filename lookup. Tests retain root/multiple/trailing slash directories, empty path rejection (including O_CREATE), and ENOTDIR for a regular file with a trailing slash. Recorded this incidental repair in the plan.

Verified
- Exact go1.25.14/go1.26.9 ordinary and race composition, engine and complete filesystem suites pass: `.gosim/task21.2-125.log`, `task21.2-126.log`, `task21.2-race125.log`, `task21.2-race126.log`. Generated temporary modules compile/run all four feature combinations with each pinned native compiler, including race builds. Probes cover binding before init, typed directory/socket operations, identity/features, repeated-seed checksums, disabled-feature non-return/non-recovery, abort cleanup, subsequent runs, invalid traces and mismatched native host rejection.
- Actual generated native dependency graphs exclude each disabled provider/model. Generation tests cover ten concurrent callers, immutable identical reuse, forged provider selections, source/import tampering and unsafe output records. These native-input composition probes are explicitly not AST-translated execution; Task 21.4 retains that required proof.
- Initial probe fixes included valid Linux nonblocking socket flags and a shared host observer counter changed to atomic after race checks caught accesses across aborted simulated runs. No engine/application race suppression was introduced. Capture tests also verify input/output ownership, state rejection, trace restoration and rejected nested engines leaving the active run unchanged.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task21.2-gate.log`, including ordinary/race/script/translation suites. Formatting, revision and whitespace checks pass; no target edits or certification promotion. Parent Task 21 remains open.
- Final Task 21.2 size: 865 changed lines (862 additions + 3 deletions), including checklist/log; staged whitespace check passed.


## Task 21.3 — Immutable translation inputs and v2 cache wiring
Date: 2026-10-19. Previous task commit: `8803322`. Commit: the Task 21.3 implementation commit containing this entry.

Implemented
- Adopted exact BuildConfig/TranslateInput/TranslateOutput records and independently owned selection data. The new entry validates identity/selection/digest/worker, generates composition first, loads Linux sources with cgo disabled using the pinned executable, selects the reviewed graph, hashes actual selected files and content dependencies, and materializes v2 cached results into private staging. Caller errors and AST failures return errors instead of terminating the host.
- Added explicit Go-command PATH binding because go/packages resolves `go` from the worker process PATH before applying Config.Env. The loader rejects an unbound worker, sets GOTOOLCHAIN=local, and uses read-only module loading. Generated input module files are atomically/exclusively published; framework requirements are merged before loading so Go cannot mutate shared go.mod. Existing regular go.sum additions are retained. Relative replacements become absolute, with version suffix handling for /vN and gopkg.in modules.
- Threaded selected hooks, bridge rewrites, native-import permissions, shared globals, test Driver metadata and filename rewriting into AST transforms. Removed automatic SSA-derived map sharing on the new path; only selected explicit shared globals survive. Hook-only imports are excluded using surviving signatures/declarations, preserving rejection for remaining uses, dot/blank imports and application private ABI. Pushed translated linkname definitions can target packages absent from a reduced graph without importing them.
- Isolated old CLI/viewer/full-suite calls as LegacyTranslate/LegacyBuildConfig/LegacyTranslateOutput until Task 22. Added a temporary native TestMainDriver entry that calls generated Driver directly; Task 22 removes the old Runtime entry, and Task 23 retains responsibility for the protocol-2 handshake. Actual candidate artifact compilation/execution remains Task 21.4.
- New-path evidence exposed profile gaps: reflect's bytes/strings imports formed a cycle through errors, so replaced only the two tiny comparisons locally; reviewed slog/internal and slog/internal/buffer as translated state; selected math/big's supplied pure Go arithmetic using the owned math_big_pure_go source tag. Added deterministic empty simulated GOROOT and translated fatal AddCleanup/Cleanup.Stop adapters, retaining application AddCleanup rejection and never registering host GC callbacks. Regenerated family source revisions; no target edits or promotion.

Verified
- Exact go1.25.14/go1.26.9 ordinary and race suites for translator, gosimtool, buildcfg, compatibility, composition, runtime and CLI pass: `.gosim/task21.3-125.log`, `task21.3-126.log`, `task21.3-race125.log`, `task21.3-race126.log`. Pinned child workers exercise real AST translation, generated test Driver metadata, cache miss/hit byte equivalence and changed source content with unchanged mtime; race workers select race sources. Full translated binary execution remains explicitly required by 21.4.
- Tests cover config ownership/forgery, unbound loader rejection, Linux/cgo-free environment, concurrent input publication/immutable reuse/failed replacement/symlinks, module paths, hook-exclusive versus surviving rejected dependencies, returned input errors, reflection byte equality, deterministic GOROOT and unrecoverable cleanup rejection. Existing typed boundary, artifact corruption, hook/layout/source and legacy translation tests remain green.
- Complete development `TMPDIR="$PWD/.gosim/host-tmp" GOCACHE="$PWD/.gosim/host-go-build" GOTOOLCHAIN=go1.26.0 ./test.sh --concurrency 1` passed: `.gosim/task21.3-gate.log`, including ordinary/race/script/translation suites. Formatting, source revision and whitespace checks pass. Parent Task 21 remains open.
- Final Task 21.3 size: 3,381 changed lines (2,534 additions + 847 deletions), including checklist/log, using conservative `git diff --cached --numstat -M`; staged whitespace check passed.
