//go:build go1.26 && !go1.27

// Package tls declares the optional crypto/tls API and rejects its execution.
// It contains no native networking, entropy, filesystem or TLS implementation.
package tls

import (
	crypto "crypto"
	x509 "crypto/x509"

	net "net"

	io "io"

	time "time"

	context "context"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported() {
	gosimruntime.Abort(
		abi.Diagnostic{
			Kind:      "UnsupportedOperation",
			Feature:   "net",
			Operation: "crypto/tls",
			Message:   "optional TLS/certificate operations are not modeled",
		},
	)
}

type Certificate struct {
	Certificate                  [][]byte
	PrivateKey                   crypto.PrivateKey
	SupportedSignatureAlgorithms []SignatureScheme
	OCSPStaple                   []byte
	SignedCertificateTimestamps  [][]byte
	Leaf                         *x509.Certificate
}
type CertificateRequestInfo struct {
	AcceptableCAs    [][]byte
	SignatureSchemes []SignatureScheme
	Version          uint16
}
type (
	ClientAuthType  int
	ClientHelloInfo struct {
		CipherSuites      []uint16
		ServerName        string
		SupportedCurves   []CurveID
		SupportedPoints   []uint8
		SignatureSchemes  []SignatureScheme
		SupportedProtos   []string
		SupportedVersions []uint16
		Extensions        []uint16
		Conn              net.Conn
	}
)

type ClientSessionCache interface {
	Get(sessionKey string) (session *ClientSessionState, ok bool)
	Put(sessionKey string, cs *ClientSessionState)
}
type (
	ClientSessionState struct{}
	Config             struct {
		Rand                                io.Reader
		Time                                func() time.Time
		Certificates                        []Certificate
		NameToCertificate                   map[string]*Certificate
		GetCertificate                      func(*ClientHelloInfo) (*Certificate, error)
		GetClientCertificate                func(*CertificateRequestInfo) (*Certificate, error)
		GetConfigForClient                  func(*ClientHelloInfo) (*Config, error)
		VerifyPeerCertificate               func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
		VerifyConnection                    func(ConnectionState) error
		RootCAs                             *x509.CertPool
		NextProtos                          []string
		ServerName                          string
		ClientAuth                          ClientAuthType
		ClientCAs                           *x509.CertPool
		InsecureSkipVerify                  bool
		CipherSuites                        []uint16
		PreferServerCipherSuites            bool
		SessionTicketsDisabled              bool
		SessionTicketKey                    [32]byte
		ClientSessionCache                  ClientSessionCache
		UnwrapSession                       func(identity []byte, cs ConnectionState) (*SessionState, error)
		WrapSession                         func(ConnectionState, *SessionState) ([]byte, error)
		MinVersion                          uint16
		MaxVersion                          uint16
		CurvePreferences                    []CurveID
		DynamicRecordSizingDisabled         bool
		Renegotiation                       RenegotiationSupport
		KeyLogWriter                        io.Writer
		EncryptedClientHelloConfigList      []byte
		EncryptedClientHelloRejectionVerify func(ConnectionState) error
		GetEncryptedClientHelloKeys         func(*ClientHelloInfo) ([]EncryptedClientHelloKey, error)
		EncryptedClientHelloKeys            []EncryptedClientHelloKey
	}
)

type (
	Conn            struct{}
	ConnectionState struct {
		Version                     uint16
		HandshakeComplete           bool
		DidResume                   bool
		CipherSuite                 uint16
		CurveID                     CurveID
		NegotiatedProtocol          string
		NegotiatedProtocolIsMutual  bool
		ServerName                  string
		PeerCertificates            []*x509.Certificate
		VerifiedChains              [][]*x509.Certificate
		SignedCertificateTimestamps [][]byte
		OCSPResponse                []byte
		TLSUnique                   []byte
		ECHAccepted                 bool
	}
)

type (
	CurveID uint16
	Dialer  struct {
		NetDialer *net.Dialer
		Config    *Config
	}
)

type EncryptedClientHelloKey struct {
	Config      []byte
	PrivateKey  []byte
	SendAsRetry bool
}
type RecordHeaderError struct {
	Msg          string
	RecordHeader [5]byte
	Conn         net.Conn
}
type (
	RenegotiationSupport int
	SessionState         struct {
		Extra     [][]byte
		EarlyData bool
	}
)
type SignatureScheme uint16

func (v *CertificateRequestInfo) Context() context.Context { unsupported(); panic("abort returned") }
func (v *CertificateRequestInfo) SupportsCertificate(c *Certificate) error {
	unsupported()
	panic("abort returned")
}
func (v ClientAuthType) String() string             { unsupported(); panic("abort returned") }
func (v *ClientHelloInfo) Context() context.Context { unsupported(); panic("abort returned") }
func (v *ClientHelloInfo) SupportsCertificate(c *Certificate) error {
	unsupported()
	panic("abort returned")
}

func (v *ClientSessionState) ResumptionState() (ticket []byte, state *SessionState, err error) {
	unsupported()
	panic("abort returned")
}
func (v *Config) Clone() *Config                       { unsupported(); panic("abort returned") }
func (v *Config) SetSessionTicketKeys(keys [][32]byte) { unsupported(); panic("abort returned") }
func (v *Config) BuildNameToCertificate()              { unsupported(); panic("abort returned") }
func (v *Config) EncryptTicket(cs ConnectionState, ss *SessionState) ([]byte, error) {
	unsupported()
	panic("abort returned")
}

func (v *Config) DecryptTicket(identity []byte, cs ConnectionState) (*SessionState, error) {
	unsupported()
	panic("abort returned")
}
func (v *Conn) LocalAddr() net.Addr                        { unsupported(); panic("abort returned") }
func (v *Conn) RemoteAddr() net.Addr                       { unsupported(); panic("abort returned") }
func (v *Conn) SetDeadline(t time.Time) error              { unsupported(); panic("abort returned") }
func (v *Conn) SetReadDeadline(t time.Time) error          { unsupported(); panic("abort returned") }
func (v *Conn) SetWriteDeadline(t time.Time) error         { unsupported(); panic("abort returned") }
func (v *Conn) NetConn() net.Conn                          { unsupported(); panic("abort returned") }
func (v *Conn) Write(b []byte) (int, error)                { unsupported(); panic("abort returned") }
func (v *Conn) Read(b []byte) (int, error)                 { unsupported(); panic("abort returned") }
func (v *Conn) Close() error                               { unsupported(); panic("abort returned") }
func (v *Conn) CloseWrite() error                          { unsupported(); panic("abort returned") }
func (v *Conn) Handshake() error                           { unsupported(); panic("abort returned") }
func (v *Conn) HandshakeContext(ctx context.Context) error { unsupported(); panic("abort returned") }
func (v *Conn) ConnectionState() ConnectionState           { unsupported(); panic("abort returned") }
func (v *Conn) OCSPResponse() []byte                       { unsupported(); panic("abort returned") }
func (v *Conn) VerifyHostname(host string) error           { unsupported(); panic("abort returned") }
func (v *ConnectionState) ExportKeyingMaterial(label string, context []byte, length int) ([]byte, error) {
	unsupported()
	panic("abort returned")
}
func (v CurveID) String() string { unsupported(); panic("abort returned") }
func (v *Dialer) Dial(network string, addr string) (net.Conn, error) {
	unsupported()
	panic("abort returned")
}

func (v *Dialer) DialContext(ctx context.Context, network string, addr string) (net.Conn, error) {
	unsupported()
	panic("abort returned")
}
func (v RecordHeaderError) Error() string      { unsupported(); panic("abort returned") }
func (v *SessionState) Bytes() ([]byte, error) { unsupported(); panic("abort returned") }
func (v SignatureScheme) String() string       { unsupported(); panic("abort returned") }

const (
	CurveP256                                     CurveID              = 23
	CurveP384                                     CurveID              = 24
	CurveP521                                     CurveID              = 25
	ECDSAWithP256AndSHA256                        SignatureScheme      = 1027
	ECDSAWithP384AndSHA384                        SignatureScheme      = 1283
	ECDSAWithP521AndSHA512                        SignatureScheme      = 1539
	ECDSAWithSHA1                                 SignatureScheme      = 515
	Ed25519                                       SignatureScheme      = 2055
	NoClientCert                                  ClientAuthType       = 0
	PKCS1WithSHA1                                 SignatureScheme      = 513
	PKCS1WithSHA256                               SignatureScheme      = 1025
	PKCS1WithSHA384                               SignatureScheme      = 1281
	PKCS1WithSHA512                               SignatureScheme      = 1537
	PSSWithSHA256                                 SignatureScheme      = 2052
	PSSWithSHA384                                 SignatureScheme      = 2053
	PSSWithSHA512                                 SignatureScheme      = 2054
	RenegotiateFreelyAsClient                     RenegotiationSupport = 2
	RenegotiateNever                              RenegotiationSupport = 0
	RenegotiateOnceAsClient                       RenegotiationSupport = 1
	RequestClientCert                             ClientAuthType       = 1
	RequireAndVerifyClientCert                    ClientAuthType       = 4
	RequireAnyClientCert                          ClientAuthType       = 2
	TLS_AES_128_GCM_SHA256                                             = 4865
	TLS_AES_256_GCM_SHA384                                             = 4866
	TLS_CHACHA20_POLY1305_SHA256                                       = 4867
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA                               = 49161
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256                            = 49187
	TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256                            = 49195
	TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA                               = 49162
	TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384                            = 49196
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305                             = 52393
	TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256                      = 52393
	TLS_ECDHE_ECDSA_WITH_RC4_128_SHA                                   = 49159
	TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA                                = 49170
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA                                 = 49171
	TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256                              = 49191
	TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256                              = 49199
	TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA                                 = 49172
	TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384                              = 49200
	TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305                               = 52392
	TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256                        = 52392
	TLS_ECDHE_RSA_WITH_RC4_128_SHA                                     = 49169
	TLS_FALLBACK_SCSV                                                  = 22016
	TLS_RSA_WITH_3DES_EDE_CBC_SHA                                      = 10
	TLS_RSA_WITH_AES_128_CBC_SHA                                       = 47
	TLS_RSA_WITH_AES_128_CBC_SHA256                                    = 60
	TLS_RSA_WITH_AES_128_GCM_SHA256                                    = 156
	TLS_RSA_WITH_AES_256_CBC_SHA                                       = 53
	TLS_RSA_WITH_AES_256_GCM_SHA384                                    = 157
	TLS_RSA_WITH_RC4_128_SHA                                           = 5
	VerifyClientCertIfGiven                       ClientAuthType       = 3
	VersionSSL30                                                       = 768
	VersionTLS10                                                       = 769
	VersionTLS11                                                       = 770
	VersionTLS12                                                       = 771
	VersionTLS13                                                       = 772
	X25519                                        CurveID              = 29
	X25519MLKEM768                                CurveID              = 4588
)

func Client(conn net.Conn, config *Config) *Conn { unsupported(); panic("abort returned") }
func Server(conn net.Conn, config *Config) *Conn { unsupported(); panic("abort returned") }
func Dial(network string, addr string, config *Config) (*Conn, error) {
	unsupported()
	panic("abort returned")
}

func Listen(network string, laddr string, config *Config) (net.Listener, error) {
	unsupported()
	panic("abort returned")
}

func NewListener(inner net.Listener, config *Config) net.Listener {
	unsupported()
	panic("abort returned")
}

func LoadX509KeyPair(certFile string, keyFile string) (Certificate, error) {
	unsupported()
	panic("abort returned")
}

func X509KeyPair(certPEMBlock []byte, keyPEMBlock []byte) (Certificate, error) {
	unsupported()
	panic("abort returned")
}

func NewLRUClientSessionCache(capacity int) ClientSessionCache {
	unsupported()
	panic("abort returned")
}
func VersionName(version uint16) string { unsupported(); panic("abort returned") }

// Cipher suite discovery is part of the optional, unsupported TLS API.
type CipherSuite struct {
	ID                uint16
	Name              string
	SupportedVersions []uint16
	Insecure          bool
}

func CipherSuites() []*CipherSuite         { unsupported(); panic("abort returned") }
func InsecureCipherSuites() []*CipherSuite { unsupported(); panic("abort returned") }
