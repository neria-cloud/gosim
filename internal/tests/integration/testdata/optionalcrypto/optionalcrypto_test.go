//go:build sim

package optionalcrypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"math/big"
	"testing"
)

// These assignments type-check the public selected package identities after
// composition, including X.509 signatures consumed by the pinned etcd graph.
var (
	_ crypto.Signer                                             = (*ecdsa.PrivateKey)(nil)
	_ crypto.Signer                                             = (*rsa.PrivateKey)(nil)
	_ func([]byte) (*ecdsa.PrivateKey, error)                   = x509.ParseECPrivateKey
	_ func([]byte) (*rsa.PrivateKey, error)                     = x509.ParsePKCS1PrivateKey
	_ func(*ecdsa.PrivateKey) ([]byte, error)                   = x509.MarshalECPrivateKey
	_ func(elliptic.Curve, *big.Int, *big.Int) *ecdsa.PublicKey = makeECDSAPublicKey
)

func makeECDSAPublicKey(curve elliptic.Curve, x, y *big.Int) *ecdsa.PublicKey {
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
}

func TestCryptoDeclarations(t *testing.T) {
	key := &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: big.NewInt(65537), E: 3}}
	if key.Public() != &key.PublicKey || key.Size() != 3 {
		t.Fatal("RSA key metadata changed")
	}
	if (&rsa.PSSOptions{Hash: crypto.SHA256}).HashFunc() != crypto.SHA256 {
		t.Fatal("PSS metadata changed")
	}
}

// Run this case explicitly and require a fatal UnsupportedOperation result.
// It must never be added to an ordinary positive suite.
func TestCryptoGuard(t *testing.T) {
	func() {
		defer func() { _ = recover() }()
		_, _ = ecdsa.GenerateKey(nil, nil)
	}()
	t.Fatal("application recovered from an unsupported crypto operation")
}
