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
