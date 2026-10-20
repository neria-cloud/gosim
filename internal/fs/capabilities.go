package fs

type Capability struct{ Operation, Status, Detail string }

// Capabilities describes observed storage calls and the additional cwd routes.
// It is semantic evidence, not a claim that the gquorum application is supported.
func Capabilities() []Capability {
	return []Capability{
		{"openat", "limited", "AT_FDCWD only; basic flags and owner/group/other checks; CLOEXEC ignored; no append"},
		{"close", "supported", "kernel-owned descriptor removal and resource release"},
		{"read", "supported", "kernel capability dispatch to virtual files or sockets"},
		{"write", "supported", "kernel capability dispatch; diagnostic fds owned by rt"},
		{"pread64", "supported", "positional read without changing descriptor offset"},
		{"pwrite64", "supported", "positional write without changing descriptor offset"},
		{"lseek", "supported", "nonnegative virtual file offset"},
		{"fdatasync", "limited", "same durability semantics as fsync"},
		{"fsync", "supported", "inode/dependency durability; before/after crash injection"},
		{"fallocate", "limited", "mode=0; reserves bounded virtual blocks in the requested range"},
		{"ftruncate", "supported", "resize writable virtual file"},
		{"flock", "stub", "success without locking, ownership, or contention"},
		{"fstat", "limited", "inode/type/size/mode/owner/group/link count; timestamps remain zero"},
		{"newfstatat", "limited", "AMD64 path stat; AT_FDCWD with flags=0 or AT_SYMLINK_NOFOLLOW; no symlink objects"},
		{"fstatat", "limited", "ARM64 path stat; AT_FDCWD with flags=0 or AT_SYMLINK_NOFOLLOW; no symlink objects"},
		{"getdents64", "limited", "one batch; dummy inode/offset fields; no resumable cursor"},
		{"mkdirat", "limited", "AT_FDCWD only; owner/group/other checks with fixed 0022 umask"},
		{"fchmod", "limited", "mode changes require owner or root; read-only mode rejects mutation"},
		{"fchmodat", "limited", "AT_FDCWD and flags=0 or AT_SYMLINK_NOFOLLOW; owner/root checks"},
		{"fchown", "limited", "root-only ownership changes; -1 preserves a field"},
		{"fchownat", "limited", "AT_FDCWD and flags=0 or AT_SYMLINK_NOFOLLOW; root-only"},
		{"renameat", "supported", "virtual rename/dependency ordering; AT_FDCWD only"},
		{"unlinkat", "supported", "virtual removal; AT_FDCWD and flags=0 or AT_REMOVEDIR"},
		{"fcntl", "limited/stub", "integer form ENOSYS; lock form success without locks"},
		{"readlinkat", "missing", "unsupported route; no simulated procfs"},
		{"statfs", "supported", "one bounded in-memory volume; 1 KiB allocated-block accounting"},
		{"fstatfs", "supported", "filesystem descriptor resolves the same bounded volume"},
		{"mmap", "limited", "read-only MAP_SHARED, zero offset, 16 MiB maximum; POPULATE ignored"},
		{"munmap", "limited", "releases entire owned mapping by start address; length ignored"},
		{"madvise", "stub", "success without applying advice"},
		{"chdir", "supported", "virtual working directory"},
		{"getcwd", "supported", "NUL-terminated virtual path"},
	}
}
