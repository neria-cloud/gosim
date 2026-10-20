package net

type Capability struct{ Operation, Status, Detail string }

func Capabilities() []Capability {
	return []Capability{
		{
			"socket", "limited",
			"IPv4 TCP protocol=0; STREAM|NONBLOCK|CLOEXEC required; no IPv6/UDP/raw/blocking sockets",
		},
		{"bind", "limited", "own IPv4 or wildcard; explicit nonzero listening port; client binds unsupported"},
		{"listen", "limited", "five-entry backlog; requested backlog otherwise ignored"},
		{"accept4", "supported", "nonblocking EAGAIN; shared descriptor allocation and rollback"},
		{
			"connect", "limited",
			"EINPROGRESS with poll readiness and SO_ERROR; recycled connections guarded by generation",
		},
		{"getsockname", "supported", "actual stream local endpoint or bound listener address"},
		{"getpeername", "supported", "modeled stream peer endpoint"},
		{
			"setsockopt", "stub/limited",
			"SO_REUSEADDR/SO_SNDBUF/SO_RCVBUF/SO_KEEPALIVE/SO_LINGER and " +
				"TCP_NODELAY/KEEPIDLE/KEEPINTVL/KEEPCNT accept correctly sized payloads " +
				"without applying options; others ENOPROTOOPT",
		},
		{"getsockopt", "limited", "SO_ERROR only; pending EINPROGRESS, failed opening ETIMEDOUT; error not cleared"},
		{"fcntl", "missing", "integer commands return ENOSYS"},
		{"read/write", "limited", "owned stream buffers and backpressure; legacy closed-read EPIPE semantics"},
		{"close", "supported", "explicit/graceful close notifies peer; crash/abort silent; release exactly once"},
		{"poll", "supported", "epoch/connection/subscription generations; kernel deadlines and cancellation"},
	}
}
