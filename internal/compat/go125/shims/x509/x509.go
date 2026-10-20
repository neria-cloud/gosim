//go:build go1.25 && !go1.26

// Package x509 declares the optional crypto/x509 API and rejects its execution.
// It contains no native networking, entropy, filesystem or TLS implementation.
package x509

import (
	"crypto/x509/pkix"
	"time"

	"encoding/asn1"

	"net"

	"net/url"

	"io"

	"math/big"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported() {
	gosimruntime.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "net",
			Operation: "crypto/x509",
			Message:   "optional TLS/certificate operations are not modeled",
		},
	)
}

type (
	CertPool    struct{}
	Certificate struct {
		Raw                         []byte
		RawTBSCertificate           []byte
		RawSubjectPublicKeyInfo     []byte
		RawSubject                  []byte
		RawIssuer                   []byte
		Signature                   []byte
		SignatureAlgorithm          SignatureAlgorithm
		PublicKeyAlgorithm          PublicKeyAlgorithm
		PublicKey                   any
		Version                     int
		SerialNumber                *big.Int
		Issuer                      pkix.Name
		Subject                     pkix.Name
		NotBefore                   time.Time
		NotAfter                    time.Time
		KeyUsage                    KeyUsage
		Extensions                  []pkix.Extension
		ExtraExtensions             []pkix.Extension
		UnhandledCriticalExtensions []asn1.ObjectIdentifier
		ExtKeyUsage                 []ExtKeyUsage
		UnknownExtKeyUsage          []asn1.ObjectIdentifier
		BasicConstraintsValid       bool
		IsCA                        bool
		MaxPathLen                  int
		MaxPathLenZero              bool
		SubjectKeyId                []byte
		AuthorityKeyId              []byte
		OCSPServer                  []string
		IssuingCertificateURL       []string
		DNSNames                    []string
		EmailAddresses              []string
		IPAddresses                 []net.IP
		URIs                        []*url.URL
		PermittedDNSDomainsCritical bool
		PermittedDNSDomains         []string
		ExcludedDNSDomains          []string
		PermittedIPRanges           []*net.IPNet
		ExcludedIPRanges            []*net.IPNet
		PermittedEmailAddresses     []string
		ExcludedEmailAddresses      []string
		PermittedURIDomains         []string
		ExcludedURIDomains          []string
		CRLDistributionPoints       []string
		PolicyIdentifiers           []asn1.ObjectIdentifier
		Policies                    []OID
		InhibitAnyPolicy            int
		InhibitAnyPolicyZero        bool
		InhibitPolicyMapping        int
		InhibitPolicyMappingZero    bool
		RequireExplicitPolicy       int
		RequireExplicitPolicyZero   bool
		PolicyMappings              []PolicyMapping
	}
)

type (
	ExtKeyUsage   int
	HostnameError struct {
		Certificate *Certificate
		Host        string
	}
)

type (
	KeyUsage int
	OID      struct{}
)

type PolicyMapping struct {
	IssuerDomainPolicy  OID
	SubjectDomainPolicy OID
}
type (
	PublicKeyAlgorithm int
	SignatureAlgorithm int
	VerifyOptions      struct {
		DNSName                   string
		Intermediates             *CertPool
		Roots                     *CertPool
		CurrentTime               time.Time
		KeyUsages                 []ExtKeyUsage
		MaxConstraintComparisions int
		CertificatePolicies       []OID
	}
)

func (v *CertPool) Clone() *CertPool          { unsupported(); panic("abort returned") }
func (v *CertPool) AddCert(cert *Certificate) { unsupported(); panic("abort returned") }
func (v *CertPool) AppendCertsFromPEM(pemCerts []byte) (ok bool) {
	unsupported()
	panic("abort returned")
}
func (v *CertPool) Subjects() [][]byte         { unsupported(); panic("abort returned") }
func (v *CertPool) Equal(other *CertPool) bool { unsupported(); panic("abort returned") }
func (v *CertPool) AddCertWithConstraint(cert *Certificate, constraint func([]*Certificate) error) {
	unsupported()
	panic("abort returned")
}

func (v *Certificate) Verify(opts VerifyOptions) (chains [][]*Certificate, err error) {
	unsupported()
	panic("abort returned")
}
func (v *Certificate) VerifyHostname(h string) error { unsupported(); panic("abort returned") }
func (v *Certificate) Equal(other *Certificate) bool { unsupported(); panic("abort returned") }
func (v *Certificate) CheckSignatureFrom(parent *Certificate) error {
	unsupported()
	panic("abort returned")
}

func (v *Certificate) CheckSignature(algo SignatureAlgorithm, signed []byte, signature []byte) error {
	unsupported()
	panic("abort returned")
}

func (v *Certificate) CheckCRLSignature(crl *pkix.CertificateList) error {
	unsupported()
	panic("abort returned")
}

func (v *Certificate) CreateCRL(
	rand io.Reader,
	priv any,
	revokedCerts []pkix.RevokedCertificate,
	now time.Time,
	expiry time.Time,
) (crlBytes []byte, err error) {
	unsupported()
	panic("abort returned")
}
func (v HostnameError) Error() string                       { unsupported(); panic("abort returned") }
func (v OID) AppendText(b []byte) ([]byte, error)           { unsupported(); panic("abort returned") }
func (v OID) MarshalText() ([]byte, error)                  { unsupported(); panic("abort returned") }
func (v *OID) UnmarshalText(text []byte) error              { unsupported(); panic("abort returned") }
func (v OID) AppendBinary(b []byte) ([]byte, error)         { unsupported(); panic("abort returned") }
func (v OID) MarshalBinary() ([]byte, error)                { unsupported(); panic("abort returned") }
func (v *OID) UnmarshalBinary(b []byte) error               { unsupported(); panic("abort returned") }
func (v OID) Equal(other OID) bool                          { unsupported(); panic("abort returned") }
func (v OID) EqualASN1OID(other asn1.ObjectIdentifier) bool { unsupported(); panic("abort returned") }
func (v OID) String() string                                { unsupported(); panic("abort returned") }
func (v PublicKeyAlgorithm) String() string                 { unsupported(); panic("abort returned") }
func (v SignatureAlgorithm) String() string                 { unsupported(); panic("abort returned") }

const (
	DSA                                       PublicKeyAlgorithm = 2
	DSAWithSHA1                               SignatureAlgorithm = 7
	DSAWithSHA256                             SignatureAlgorithm = 8
	ECDSA                                     PublicKeyAlgorithm = 3
	ECDSAWithSHA1                             SignatureAlgorithm = 9
	ECDSAWithSHA256                           SignatureAlgorithm = 10
	ECDSAWithSHA384                           SignatureAlgorithm = 11
	ECDSAWithSHA512                           SignatureAlgorithm = 12
	Ed25519                                   PublicKeyAlgorithm = 4
	ExtKeyUsageAny                            ExtKeyUsage        = 0
	ExtKeyUsageClientAuth                     ExtKeyUsage        = 2
	ExtKeyUsageCodeSigning                    ExtKeyUsage        = 3
	ExtKeyUsageEmailProtection                ExtKeyUsage        = 4
	ExtKeyUsageIPSECEndSystem                 ExtKeyUsage        = 5
	ExtKeyUsageIPSECTunnel                    ExtKeyUsage        = 6
	ExtKeyUsageIPSECUser                      ExtKeyUsage        = 7
	ExtKeyUsageMicrosoftCommercialCodeSigning ExtKeyUsage        = 12
	ExtKeyUsageMicrosoftKernelCodeSigning     ExtKeyUsage        = 13
	ExtKeyUsageMicrosoftServerGatedCrypto     ExtKeyUsage        = 10
	ExtKeyUsageNetscapeServerGatedCrypto      ExtKeyUsage        = 11
	ExtKeyUsageOCSPSigning                    ExtKeyUsage        = 9
	ExtKeyUsageServerAuth                     ExtKeyUsage        = 1
	ExtKeyUsageTimeStamping                   ExtKeyUsage        = 8
	KeyUsageCRLSign                           KeyUsage           = 64
	KeyUsageCertSign                          KeyUsage           = 32
	KeyUsageContentCommitment                 KeyUsage           = 2
	KeyUsageDataEncipherment                  KeyUsage           = 8
	KeyUsageDecipherOnly                      KeyUsage           = 256
	KeyUsageDigitalSignature                  KeyUsage           = 1
	KeyUsageEncipherOnly                      KeyUsage           = 128
	KeyUsageKeyAgreement                      KeyUsage           = 16
	KeyUsageKeyEncipherment                   KeyUsage           = 4
	MD2WithRSA                                SignatureAlgorithm = 1
	MD5WithRSA                                SignatureAlgorithm = 2
	PureEd25519                               SignatureAlgorithm = 16
	RSA                                       PublicKeyAlgorithm = 1
	SHA1WithRSA                               SignatureAlgorithm = 3
	SHA256WithRSA                             SignatureAlgorithm = 4
	SHA256WithRSAPSS                          SignatureAlgorithm = 13
	SHA384WithRSA                             SignatureAlgorithm = 5
	SHA384WithRSAPSS                          SignatureAlgorithm = 14
	SHA512WithRSA                             SignatureAlgorithm = 6
	SHA512WithRSAPSS                          SignatureAlgorithm = 15
	UnknownPublicKeyAlgorithm                 PublicKeyAlgorithm = 0
	UnknownSignatureAlgorithm                 SignatureAlgorithm = 0
)

func NewCertPool() *CertPool                               { unsupported(); panic("abort returned") }
func ParseCertificate(der []byte) (*Certificate, error)    { unsupported(); panic("abort returned") }
func ParseCertificates(der []byte) ([]*Certificate, error) { unsupported(); panic("abort returned") }
func ParsePKCS1PrivateKey(der []byte) (any, error)         { unsupported(); panic("abort returned") }
func ParsePKCS8PrivateKey(der []byte) (key any, err error) { unsupported(); panic("abort returned") }
