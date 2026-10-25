# Filesystem: directory handles and os.Root

The in-memory filesystem is rooted at `/`; processes start in `/app`. Configure each machine before its first start with `fs.ConfigureMachine`. Capacity defaults to 128 MiB and must be a multiple of 1 KiB. `fs.Stats` reports total, used and free bytes in allocated 1 KiB file-data blocks, plus block size and read-only status. Sparse holes consume no blocks; `/etc/hosts` initially consumes one. A write or `fallocate` reservation that would exceed capacity fails atomically with `ENOSPC`. Capacity cannot be resized after configuration. A restored image above the target capacity remains readable, reports zero free space and rejects new allocation until usage falls.

`fs.Config` also sets the machine's effective UID/GID (root/root by default) and a fixed per-operation `Latency`. Files and directories store mode and ownership; creation applies a fixed `0022` umask. Owner/group/other checks cover file opens, directory traversal and mutations; chmod requires the owner or root, and chown requires root. `fs.SetReadOnly(machine, true)` immediately rejects data and metadata mutation with `EROFS`, including writes through existing descriptors. Set it to `false` to resume writes. The policy survives stop, crash, restart and snapshot restore. Configured latency delays completion of data I/O and mutating filesystem calls, including `ENOSPC` and `EROFS`; metadata reads such as stat and statfs remain immediate.

`fs.SetCrashMode` selects `DurableOnly` or `SamplePending`; `fs.SetSyncFault` configures bounded sync-failure probabilities.

After a crash, `fs.CrashStates(machine, fs.EnumerateOptions{MaxStates: n})` owns a synchronous iterator over immutable crash outcomes. A positive budget is required. `Next` returns a snapshot and availability flag; exceeding the budget while more outcomes exist returns `fs.ErrEnumerationLimit`. Close the iterator when finished, and close each returned snapshot separately. Closing the iterator does not invalidate snapshots already returned. `fs.Restore` accepts a retained snapshot from the same simulation for an unstarted or stopped/crashed target; configure its next main function and start/restart it explicitly. Destroying a machine does not invalidate separately owned snapshots.

Graceful stop flushes modeled writes and metadata. Crash behavior preserves uncertainty in non-fsynced operations, including block reservations and mode/ownership changes. Supplemental groups, ACLs, sticky bits, hard links, special files and real POSIX locking remain outside the model.

## Directory handles and symbolic links

An open directory handle continues to address the same directory after it is
renamed or replaced. Closing the handle invalidates its use. Handles to removed
directories can still be inspected and synchronized, but cannot create entries
in the removed directory.

Relative `openat`, `fstatat`, `mkdirat`, `renameat` and `unlinkat` paths use the
supplied directory handle. Absolute paths ignore that handle. `O_DIRECTORY`,
`O_NOFOLLOW`, `O_EXCL` and `AT_SYMLINK_NOFOLLOW` control directory requirements,
exclusive creation and symbolic-link handling. Symlink loops and excessive link
chains fail. `os.NewFile` supports wrapping regular-file and directory handles,
including their `Sync` operations.

## os.Root and directory descriptors

Use `os.OpenRoot` and `os.Root` when operations must remain within a directory.
Valid relative symlinks inside the root are supported; traversal that escapes
the root is rejected. Ordinary `openat` calls can follow intermediate symlinks
outside their supplied directory and do not provide Root confinement.

Both supported Go families provide the following simulated behavior:

| Root methods | Support and limits |
| --- | --- |
| `Name`, `Close` | Supported; operations on a closed root fail |
| `Open`, `Create`, `OpenFile` | Regular files and directories, subject to supported flags |
| `OpenRoot` | Nested roots, including after directory rename |
| `Chmod`, `Chown`, `Lchown` | Chmod requires the owner or root; chown requires root |
| `Mkdir`, `MkdirAll` | Supported; durable creation still requires parent-directory synchronization |
| `Remove`, `RemoveAll` | File/directory and emptiness checks; recursive removal inherits the directory-listing limit |
| `Stat`, `Lstat` | Distinguish links from their targets; timestamps remain zero and directory link counts are incomplete |
| `Readlink`, `Symlink` | Supported within Root's traversal rules |
| `Rename` | Ordinary replacement rules; the destination may be replaced |
| `Link` | Unsupported; attempting hard-link creation aborts the simulation |
| `Chtimes` | Unsupported; attempting to change timestamps aborts the simulation |
| `ReadFile`, `WriteFile` | Subject to file-access and open-flag limits |
| `FS` | Supports `Open`, `ReadFile`, `ReadDir`, `ReadLink`, `Stat` and `Lstat`, with the same confinement and limits |

## Directory listing and synchronization

Directory reads return a complete listing within the supported read-buffer
limit. A directory too large for that limit aborts the simulation with
`UnsupportedOperation`. Resumable enumeration is unavailable.
`Seek(0, io.SeekStart)` rewinds a directory; other directory seeks, including
`SEEK_CUR`, are unsupported.

Use file `Sync` for file data and synchronize affected parent directories when
testing durable creation, removal or rename. Synchronizing a newly created
directory does not make its name durable in its parent. Synchronizing a directory
does not make arbitrary child file data durable. These simulated persistence
outcomes do not establish hardware power-loss behavior for a native filesystem.

## Limits

| Capability | Current boundary |
| --- | --- |
| Open flags | Append, `O_NONBLOCK`, `O_PATH` and `O_TMPFILE` are unsupported; `O_CLOEXEC` is accepted but has no effect |
| Descriptor flags | Integer fcntl operations, including `F_GETFL`, return `ENOSYS`; duplicate descriptors are unsupported |
| Read-only errors | Mutation attempts may report `EROFS` before other errors, so precedence can differ from a native Linux read-only filesystem |
| Hard links and mounts | Hard links, distinct mounts, bind mounts, mount replacement and `statx` are unsupported |
| Special files | No special-file or procfs behavior |
| File locking | Lock calls do not establish ownership, contention or exclusion |

Use native tests for behavior the simulation cannot establish, including mount
identity, hard-link safety and file-lock exclusion.
