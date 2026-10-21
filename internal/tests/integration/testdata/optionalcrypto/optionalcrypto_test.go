//go:build sim

package optionalcrypto

import (
	"crypto/ecdsa"
	"testing"
)

// Run this case explicitly and require a fatal UnsupportedOperation result.
// It must never be added to an ordinary positive suite.
func TestCryptoGuard(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		_, _ = ecdsa.GenerateKey(nil, nil)
	}()
	t.Fatal("application recovered from an unsupported crypto operation")
}
