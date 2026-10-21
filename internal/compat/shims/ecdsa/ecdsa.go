//go:build go1.25

// Package ecdsa declares optional signing and key types for translated callers.
// Key generation and signature algorithms remain unsupported.
package ecdsa

import (
	"crypto"
	"io"
	"math/big"

	"github.com/neria-cloud/gosim/internal/compat/shims/elliptic"
	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported(operation string) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "crypto/ecdsa." + operation, Message: "optional ECDSA operations are not modeled"})
}

type PublicKey struct {
	elliptic.Curve
	X, Y *big.Int
}

type PrivateKey struct {
	PublicKey
	D *big.Int
}

func (priv *PrivateKey) Public() crypto.PublicKey { return &priv.PublicKey }

// TODO(not planned): model ECDSA signing, verification and key generation only
// after their randomness and translated key ownership are specified.
func GenerateKey(c elliptic.Curve, random io.Reader) (*PrivateKey, error) {
	unsupported("GenerateKey")
	panic("abort returned")
}

func Sign(random io.Reader, priv *PrivateKey, digest []byte) (*big.Int, *big.Int, error) {
	unsupported("Sign")
	panic("abort returned")
}

func Verify(pub *PublicKey, digest []byte, r, s *big.Int) bool {
	unsupported("Verify")
	panic("abort returned")
}

func (priv *PrivateKey) Sign(random io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	unsupported("PrivateKey.Sign")
	panic("abort returned")
}

var _ crypto.Signer = (*PrivateKey)(nil)
