# Syscall monitoring

`machine.SyscallsMonitor()` returns controls for that machine's routed syscall
attempts. All handles for the same machine share state. Counting is initially
disabled; creating a handle does not enable it or start the machine.

| Method | Behavior |
| --- | --- |
| `Start()` | Enable or resume counting without clearing |
| `Stop()` | Pause without clearing |
| `Reset()` | Clear statistics; preserve enabled/paused state |
| `Counts()` | Return `map[string]uint64` of observed names and totals |
| `Stats()` | Return totals, fixed syscall groups, and descriptor-owner buckets |
| `List()` | Return all available names with Group and DescriptorBased, sorted by Name |

Inside a simulated test:

```go
monitor := gosim.CurrentMachine().SyscallsMonitor()
if err := monitor.Start(); err != nil {
    t.Fatal(err)
}
_ = syscall.Getpid()
if err := monitor.Stop(); err != nil {
    t.Fatal(err)
}
counts, err := monitor.Counts()
if err != nil {
    t.Fatal(err)
}
stats, err := monitor.Stats()
if err != nil {
    t.Fatal(err)
}
t.Log(counts["getpid"], stats["getpid"]) // 1; rt group, none descriptor bucket
```

Use a workload barrier or Stop for a stable window: monitor calls are scheduling
points, and separate queries can see different totals while counting continues.
Each snapshot is detached, including nested maps. Successful empty queries return
non-nil containers. Counts/Stats omit uncalled names and zero buckets. List
includes uncalled names but has no counters or runtime descriptor buckets.

Each `SyscallStat` has `Count`, `Group`, and `ByDescriptor`.
Group is fixed: `rt`, `fs`, or `net`. Shared calls such as read/write/close/fcntl
belong to `rt`, even when accessing files or sockets. ByDescriptor records the
primary input descriptor's owner (`rt`, `fs`, `net`), `none` when no primary FD
exists, or `unknown` when its lookup fails. Count equals the sum of the buckets.
For example, two file reads and one socket read produce read Count=3, Group=rt,
ByDescriptor={fs:2, net:1}. Individual descriptor numbers are not retained.

Ownership is captured before execution, including before close removes an FD.
Directory-context and output descriptors do not count as primary inputs: openat
and socket use `none`, accept4 uses its listener, and mmap uses its input FD.
List's DescriptorBased describes this static input rule, not successful execution.
Discovery respects the immutable feature set: 8 names for rt, 34 for rt/fs,
17 for rt/net, and 43 with all features.

Monitor operations work before machine Start and after stop/crash; counters and
enabled state survive Restart. Enabling before Start includes package init.
Destroy invalidates the handle, and handles from a previous run are rejected.
Native operations return ErrNotSim; query errors return nil containers.

Only admitted typed routes count. Modeled errors and retries each count once;
completion, polling, controls, teardown, native calls, and hook-local early
returns do not. Limited/stubbed routes remain attempts, not full Linux semantics.
Translated vgetrandom contributes to getrandom; GODEBUG writes contribute to write.
Vectored writes use writev, without an additional write count. Its trace label
is now `call Writev`; source/route build identity separates these traces without
changing ReplayRevision.

Internally, bounded Machine-owned counters aggregate canonical names and owner
buckets. Generated schema helpers identify primary input FDs. Queries copy
nonzero bucket rows, then fold them into public totals/histograms. Those rows
preserve owner-specific counts for future selection before snapshot folding.
Observation and catalogue boundaries also permit future selection before
aggregation and listing; catalogue entries have only static metadata. CEL and
all filter APIs/evaluation are outside the current implementation.

The [machines example](../examples/network/machines/machines_test.go) measures
file activity on a child machine and checks its totals and ownership buckets.
