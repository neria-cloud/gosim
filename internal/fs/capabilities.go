package fs

type Capability struct{ Operation, Status, Detail string }

const (
	statusSupported = "supported"
	statusLimited   = "limited"
)

// Capabilities describes observed storage calls and the additional cwd routes.
// It is semantic evidence, not a claim that the gquorum application is supported.
func Capabilities() []Capability {
	return []Capability{
		{
			"openat", statusLimited,
			"AT_FDCWD only; basic flags and owner/group/other checks; CLOEXEC ignored; no append",
		},
		{"close", statusSupported, "kernel-owned descriptor removal and resource release"},
		{"read", statusSupported, "kernel capability dispatch to virtual files or sockets"},
		{"write", statusSupported, "kernel capability dispatch; diagnostic fds owned by rt"},
		{"pread64", statusSupported, "positional read without changing descriptor offset"},
		{"pwrite64", statusSupported, "positional write without changing descriptor offset"},
		{"lseek", statusSupported, "nonnegative virtual file offset"},
		{"fdatasync", statusLimited, "same durability semantics as fsync"},
		{"fsync", statusSupported, "inode/dependency durability; before/after crash injection"},
		{"fallocate", statusLimited, "mode=0; reserves bounded virtual blocks in the requested range"},
		{"ftruncate", statusSupported, "resize writable virtual file"},
		{"flock", "stub", "success without locking, ownership, or contention"},
		{"fstat", statusLimited, "inode/type/size/mode/owner/group/link count; timestamps remain zero"},
		{
			"newfstatat", statusLimited,
			"AMD64 path stat; AT_FDCWD with flags=0 or AT_SYMLINK_NOFOLLOW; no symlink objects",
		},
		{"fstatat", statusLimited, "ARM64 path stat; AT_FDCWD with flags=0 or AT_SYMLINK_NOFOLLOW; no symlink objects"},
		{"getdents64", statusLimited, "one batch; dummy inode/offset fields; no resumable cursor"},
		{"mkdirat", statusLimited, "AT_FDCWD only; owner/group/other checks with fixed 0022 umask"},
		{"fchmod", statusLimited, "mode changes require owner or root; read-only mode rejects mutation"},
		{
			"fchmodat", statusLimited,
			"AT_FDCWD and flags=0 or AT_SYMLINK_NOFOLLOW; owner/root checks",
		},
		{"fchown", statusLimited, "root-only ownership changes; -1 preserves a field"},
		{"fchownat", statusLimited, "AT_FDCWD and flags=0 or AT_SYMLINK_NOFOLLOW; root-only"},
		{"renameat", statusSupported, "virtual rename/dependency ordering; AT_FDCWD only"},
		{"unlinkat", statusSupported, "virtual removal; AT_FDCWD and flags=0 or AT_REMOVEDIR"},
		{"fcntl", "limited/stub", "integer form ENOSYS; lock form success without locks"},
		{"readlinkat", "missing", "unsupported route; no simulated procfs"},
		{"statfs", statusSupported, "one bounded in-memory volume; 1 KiB allocated-block accounting"},
		{"fstatfs", statusSupported, "filesystem descriptor resolves the same bounded volume"},
		{"mmap", statusLimited, "read-only MAP_SHARED, zero offset, 16 MiB maximum; POPULATE ignored"},
		{"munmap", statusLimited, "releases entire owned mapping by start address; length ignored"},
		{"madvise", "stub", "success without applying advice"},
		{"chdir", statusSupported, "virtual working directory"},
		{"getcwd", statusSupported, "NUL-terminated virtual path"},
	}
}
