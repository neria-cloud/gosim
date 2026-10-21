//go:build !go1.25 || go1.27

package host

// Direct native linknames require their own support guard.
var _ = GOSIM_UNSUPPORTED_GO_FAMILY_USE_GO125_OR_GO126
