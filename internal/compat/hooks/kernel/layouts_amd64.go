// Code generated from pinned Linux source by gosim-gen. DO NOT EDIT.
//go:build go1.25 && amd64

package kernel

type Timespec struct {
	Sec  int64
	Nsec int64
}
type Stat_t struct {
	Dev       uint64
	Ino       uint64
	Nlink     uint64
	Mode      uint32
	Uid       uint32
	Gid       uint32
	X__pad0   int32
	Rdev      uint64
	Size      int64
	Blksize   int64
	Blocks    int64
	Atim      Timespec
	Mtim      Timespec
	Ctim      Timespec
	X__unused [3]int64
}
type Statfs_t struct {
	Type    int64
	Bsize   int64
	Blocks  uint64
	Bfree   uint64
	Bavail  uint64
	Files   uint64
	Ffree   uint64
	Fsid    Fsid
	Namelen int64
	Frsize  int64
	Flags   int64
	Spare   [4]int64
}
type Fsid struct{ X__val [2]int32 }
type Dirent struct {
	Ino       uint64
	Off       int64
	Reclen    uint16
	Type      uint8
	Name      [256]int8
	Pad_cgo_0 [5]byte
}
type Flock_t struct {
	Type      int16
	Whence    int16
	Pad_cgo_0 [4]byte
	Start     int64
	Len       int64
	Pid       int32
	Pad_cgo_1 [4]byte
}
type RawSockaddr struct {
	Family uint16
	Data   [14]int8
}
type RawSockaddrInet4 struct {
	Family uint16
	Port   uint16
	Addr   [4]byte
	Zero   [8]uint8
}
type RawSockaddrAny struct {
	Addr RawSockaddr
	Pad  [96]int8
}
type Utsname struct {
	Sysname    [65]int8
	Nodename   [65]int8
	Release    [65]int8
	Version    [65]int8
	Machine    [65]int8
	Domainname [65]int8
}
type Rlimit struct {
	Cur uint64
	Max uint64
}
type XTimespec struct {
	Sec  int64
	Nsec int64
}
type XStat_t struct {
	Dev     uint64
	Ino     uint64
	Nlink   uint64
	Mode    uint32
	Uid     uint32
	Gid     uint32
	_       int32
	Rdev    uint64
	Size    int64
	Blksize int64
	Blocks  int64
	Atim    XTimespec
	Mtim    XTimespec
	Ctim    XTimespec
	_       [3]int64
}
type XStatfs_t struct {
	Type    int64
	Bsize   int64
	Blocks  uint64
	Bfree   uint64
	Bavail  uint64
	Files   uint64
	Ffree   uint64
	XFsid   XFsid
	Namelen int64
	Frsize  int64
	Flags   int64
	Spare   [4]int64
}
type XFsid struct{ Val [2]int32 }
type XDirent struct {
	Ino    uint64
	Off    int64
	Reclen uint16
	Type   uint8
	Name   [256]int8
	_      [5]byte
}
type XFlock_t struct {
	Type   int16
	Whence int16
	Start  int64
	Len    int64
	Pid    int32
	_      [4]byte
}
type XRawSockaddr struct {
	Family uint16
	Data   [14]int8
}
type XRawSockaddrInet4 struct {
	Family uint16
	Port   uint16
	Addr   [4]byte
	Zero   [8]uint8
}
type XRawSockaddrAny struct {
	Addr XRawSockaddr
	Pad  [96]int8
}
type XUtsname struct {
	Sysname    [65]byte
	Nodename   [65]byte
	Release    [65]byte
	Version    [65]byte
	Machine    [65]byte
	Domainname [65]byte
}
type XRlimit struct {
	Cur uint64
	Max uint64
}

const LinuxSYS_ACCEPT4 = 288
const LinuxSYS_BIND = 49
const LinuxSYS_CHDIR = 80
const LinuxSYS_CLOSE = 3
const LinuxSYS_CONNECT = 42
const LinuxSYS_FALLOCATE = 285
const LinuxSYS_FCNTL = 72
const LinuxSYS_FDATASYNC = 75
const LinuxSYS_FLOCK = 73
const LinuxSYS_FSTAT = 5
const LinuxSYS_FSYNC = 74
const LinuxSYS_FTRUNCATE = 77
const LinuxSYS_GETCWD = 79
const LinuxSYS_GETDENTS64 = 217
const LinuxSYS_GETPEERNAME = 52
const LinuxSYS_GETPID = 39
const LinuxSYS_GETRANDOM = 318
const LinuxSYS_GETSOCKNAME = 51
const LinuxSYS_GETSOCKOPT = 55
const LinuxSYS_LISTEN = 50
const LinuxSYS_LSEEK = 8
const LinuxSYS_MADVISE = 28
const LinuxSYS_MKDIRAT = 258
const LinuxSYS_MMAP = 9
const LinuxSYS_MUNMAP = 11
const LinuxSYS_NEWFSTATAT = 262
const LinuxSYS_OPENAT = 257
const LinuxSYS_PREAD64 = 17
const LinuxSYS_PWRITE64 = 18
const LinuxSYS_READ = 0
const LinuxSYS_RENAMEAT = 264
const LinuxSYS_SETSOCKOPT = 54
const LinuxSYS_SOCKET = 41
const LinuxSYS_UNAME = 63
const LinuxSYS_UNLINKAT = 263
const LinuxSYS_WRITE = 1
