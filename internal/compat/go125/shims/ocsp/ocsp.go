//go:build go1.25 && !go1.26

// Package ocsp declares the optional golang.org/x/crypto/ocsp API and rejects its execution.
// It contains no native networking, entropy, filesystem or TLS implementation.
package ocsp

import (
	"crypto"
	"crypto/x509/pkix"

	"math/big"

	"time"

	"crypto/x509"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported() {
	gosimruntime.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "net",
			Operation: "golang.org/x/crypto/ocsp",
			Message:   "optional TLS/certificate operations are not modeled",
		},
	)
}

const unsupportedAbortMessage = "abort returned"

type Request struct {
	HashAlgorithm  crypto.Hash
	IssuerNameHash []byte
	IssuerKeyHash  []byte
	SerialNumber   *big.Int
}
type RequestOptions struct {
	Hash crypto.Hash
}
type Response struct {
	Raw                []byte
	Status             int
	SerialNumber       *big.Int
	ProducedAt         time.Time
	ThisUpdate         time.Time
	NextUpdate         time.Time
	RevokedAt          time.Time
	RevocationReason   int
	Certificate        *x509.Certificate
	TBSResponseData    []byte
	Signature          []byte
	SignatureAlgorithm x509.SignatureAlgorithm
	IssuerHash         crypto.Hash
	RawResponderName   []byte
	ResponderKeyHash   []byte
	Extensions         []pkix.Extension
	ExtraExtensions    []pkix.Extension
}

func (_ *Request) Marshal() ([]byte, error) { unsupported(); panic(unsupportedAbortMessage) }
func (_ *Response) CheckSignatureFrom(_ *x509.Certificate) error {
	unsupported()
	panic(unsupportedAbortMessage)
}

const (
	AACompromise         = 10
	AffiliationChanged   = 3
	CACompromise         = 2
	CertificateHold      = 6
	CessationOfOperation = 5
	Good                 = 0
	KeyCompromise        = 1
	PrivilegeWithdrawn   = 9
	RemoveFromCRL        = 8
	Revoked              = 1
	ServerFailed         = 3
	Superseded           = 4
	Unknown              = 2
	Unspecified          = 0
)

func CreateRequest(_ *x509.Certificate, _ *x509.Certificate, opts *RequestOptions) ([]byte, error) {
	unsupported()
	panic(unsupportedAbortMessage)
}

func CreateResponse(
	_ *x509.Certificate,
	_ *x509.Certificate,
	_ Response,
	_ crypto.Signer,
) ([]byte, error) {
	unsupported()
	panic(unsupportedAbortMessage)
}
func ParseRequest(_ []byte) (*Request, error) { unsupported(); panic(unsupportedAbortMessage) }
func ParseResponse(_ []byte, _ *x509.Certificate) (*Response, error) {
	unsupported()
	panic(unsupportedAbortMessage)
}

func ParseResponseForCert(_ []byte, _ *x509.Certificate, _ *x509.Certificate) (*Response, error) {
	unsupported()
	panic(unsupportedAbortMessage)
}
