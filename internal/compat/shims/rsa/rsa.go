//go:build go1.25

// Package rsa preserves optional key declarations for etcd authentication and
// JWT code. No RSA algorithm or key-generation operation is modeled.
package rsa

import (
	"crypto"
	"io"
	"math/big"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported(operation string) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "crypto/rsa." + operation, Message: "optional RSA operations are not modeled"})
}

type PublicKey struct {
	N *big.Int
	E int
}

func (pub *PublicKey) Size() int { return (pub.N.BitLen() + 7) / 8 }

type CRTValue struct {
	Exp, Coeff, R *big.Int
}

type PrecomputedValues struct {
	Dp, Dq, Qinv *big.Int
	CRTValues    []CRTValue
}

type PrivateKey struct {
	PublicKey
	D           *big.Int
	Primes      []*big.Int
	Precomputed PrecomputedValues
}

func (priv *PrivateKey) Public() crypto.PublicKey { return &priv.PublicKey }

const (
	PSSSaltLengthAuto       = 0
	PSSSaltLengthEqualsHash = -1
)

type PSSOptions struct {
	SaltLength int
	Hash       crypto.Hash
}

func (opts *PSSOptions) HashFunc() crypto.Hash { return opts.Hash }

// TODO(not planned): model RSA sign/verify and entropy only after their
// translated key ownership and replay semantics are specified.
func GenerateKey(random io.Reader, bits int) (*PrivateKey, error) {
	unsupported("GenerateKey")
	panic("abort returned")
}

func SignPKCS1v15(random io.Reader, priv *PrivateKey, hash crypto.Hash, digest []byte) ([]byte, error) {
	unsupported("SignPKCS1v15")
	panic("abort returned")
}

func VerifyPKCS1v15(pub *PublicKey, hash crypto.Hash, digest, signature []byte) error {
	unsupported("VerifyPKCS1v15")
	panic("abort returned")
}

func SignPSS(random io.Reader, priv *PrivateKey, hash crypto.Hash, digest []byte, opts *PSSOptions) ([]byte, error) {
	unsupported("SignPSS")
	panic("abort returned")
}

func VerifyPSS(pub *PublicKey, hash crypto.Hash, digest, signature []byte, opts *PSSOptions) error {
	unsupported("VerifyPSS")
	panic("abort returned")
}

func (priv *PrivateKey) Sign(random io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	unsupported("PrivateKey.Sign")
	panic("abort returned")
}

var _ crypto.Signer = (*PrivateKey)(nil)
