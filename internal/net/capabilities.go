package net

type Capability struct{ Operation, Status, Detail string }

const (
	statusLimited   = "limited"
	statusSupported = "supported"
)

func Capabilities() []Capability {
	return []Capability{
		{
			"socket", statusLimited,
			"IPv4 TCP protocol=0; STREAM|NONBLOCK|CLOEXEC required; no IPv6/UDP/raw/blocking sockets",
		},
		{"bind", statusLimited, "own IPv4 or wildcard; explicit nonzero listening port; client binds unsupported"},
		{"listen", statusLimited, "five-entry backlog; requested backlog otherwise ignored"},
		{"accept4", statusSupported, "nonblocking EAGAIN; shared descriptor allocation and rollback"},
		{
			"connect", statusLimited,
			"EINPROGRESS with poll readiness and SO_ERROR; recycled connections guarded by generation",
		},
		{"getsockname", statusSupported, "actual stream local endpoint or bound listener address"},
		{"getpeername", statusSupported, "modeled stream peer endpoint"},
		{
			"setsockopt", "stub/limited",
			"SO_REUSEADDR/SO_SNDBUF/SO_RCVBUF/SO_KEEPALIVE/SO_LINGER and " +
				"TCP_NODELAY/KEEPIDLE/KEEPINTVL/KEEPCNT accept correctly sized payloads " +
				"without applying options; others ENOPROTOOPT",
		},
		{
			"getsockopt", statusLimited,
			"SO_ERROR only; pending EINPROGRESS, failed opening ETIMEDOUT; error not cleared",
		},
		{"fcntl", "missing", "integer commands return ENOSYS"},
		{"read/write", statusLimited, "owned stream buffers and backpressure; legacy closed-read EPIPE semantics"},
		{"close", statusSupported, "explicit/graceful close notifies peer; crash/abort silent; release exactly once"},
		{"poll", statusSupported, "epoch/connection/subscription generations; kernel deadlines and cancellation"},
	}
}
