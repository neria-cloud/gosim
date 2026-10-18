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
