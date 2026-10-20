// Code generated from pinned Linux source by gosim-gen. DO NOT EDIT.
//go:build go1.26 && !go1.27 && arm64

package kernel

type Timespec struct {
	Sec  int64
	Nsec int64
}
type Stat_t struct {
	Dev               uint64
	Ino               uint64
	Mode              uint32
	Nlink             uint32
	Uid               uint32
	Gid               uint32
	Rdev              uint64
	X__pad1           uint64
	Size              int64
	Blksize           int32
	X__pad2           int32
	Blocks            int64
	Atim              Timespec
	Mtim              Timespec
	Ctim              Timespec
	X__glibc_reserved [2]int32
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
	Mode    uint32
	Nlink   uint32
	Uid     uint32
	Gid     uint32
	Rdev    uint64
	_       uint64
	Size    int64
	Blksize int32
	_       int32
	Blocks  int64
	Atim    XTimespec
	Mtim    XTimespec
	Ctim    XTimespec
	_       [2]int32
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

const LinuxSYS_ACCEPT4 = 242
const LinuxSYS_BIND = 200
const LinuxSYS_CHDIR = 49
const LinuxSYS_CLOSE = 57
const LinuxSYS_CONNECT = 203
const LinuxSYS_FALLOCATE = 47
const LinuxSYS_FCNTL = 25
const LinuxSYS_FDATASYNC = 83
const LinuxSYS_FLOCK = 32
const LinuxSYS_FSTAT = 80
const LinuxSYS_FSTATAT = 79
const LinuxSYS_FSYNC = 82
const LinuxSYS_FTRUNCATE = 46
const LinuxSYS_GETCWD = 17
const LinuxSYS_GETDENTS64 = 61
const LinuxSYS_GETPEERNAME = 205
const LinuxSYS_GETPID = 172
const LinuxSYS_GETRANDOM = 278
const LinuxSYS_GETSOCKNAME = 204
const LinuxSYS_GETSOCKOPT = 209
const LinuxSYS_LISTEN = 201
const LinuxSYS_LSEEK = 62
const LinuxSYS_MADVISE = 233
const LinuxSYS_MKDIRAT = 34
const LinuxSYS_MMAP = 222
const LinuxSYS_MUNMAP = 215
const LinuxSYS_OPENAT = 56
const LinuxSYS_PREAD64 = 67
const LinuxSYS_PWRITE64 = 68
const LinuxSYS_READ = 63
const LinuxSYS_RENAMEAT = 38
const LinuxSYS_SETSOCKOPT = 208
const LinuxSYS_SOCKET = 198
const LinuxSYS_UNAME = 160
const LinuxSYS_UNLINKAT = 35
const LinuxSYS_WRITE = 64
