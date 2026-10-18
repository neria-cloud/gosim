//go:build !go1.25 || go1.27

package hostabi

// There is deliberately no channel-based coroutine fallback.
var _ = GOSIM_UNSUPPORTED_GO_FAMILY_USE_GO125_OR_GO126
