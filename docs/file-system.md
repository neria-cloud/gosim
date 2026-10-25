# Filesystem: directory handles and os.Root

Gosim supports directory handles, symbolic links and Go's `os.Root` API for
filesystem tests. The simulation follows Linux behavior on supported Linux and
macOS hosts, within the limits below.

For filesystem configuration, capacity, permissions and crash controls, see
[Filesystem recovery](../README.md#filesystem-recovery). The full feature list
remains in the [features table](features.md).

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
