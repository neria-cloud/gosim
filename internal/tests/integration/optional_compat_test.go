//go:build sim

package behavior_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"math/big"
	"reflect"
	"runtime"
	"runtime/trace"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
	"github.com/prometheus/client_golang/prometheus"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/neria-cloud/gosim"
)

// These assignments type-check the selected public key and X.509 identities.
var (
	_ crypto.Signer                                             = (*ecdsa.PrivateKey)(nil)
	_ crypto.Signer                                             = (*rsa.PrivateKey)(nil)
	_ func([]byte) (*ecdsa.PrivateKey, error)                   = x509.ParseECPrivateKey
	_ func([]byte) (*rsa.PrivateKey, error)                     = x509.ParsePKCS1PrivateKey
	_ func(*ecdsa.PrivateKey) ([]byte, error)                   = x509.MarshalECPrivateKey
	_ func(elliptic.Curve, *big.Int, *big.Int) *ecdsa.PublicKey = makeOptionalECDSAPublicKey
)

func makeOptionalECDSAPublicKey(curve elliptic.Curve, x, y *big.Int) *ecdsa.PublicKey {
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

func TestReflect2Initialization(t *testing.T) {
	if jsoniter.ConfigDefault == nil || jsoniter.ConfigCompatibleWithStandardLibrary == nil {
		t.Fatal("jsoniter configuration did not initialize")
	}
	ptr := reflect2.TypeOfPtr((*string)(nil))
	if ptr.Kind() != reflect.Ptr || ptr.Elem().Kind() != reflect.String {
		t.Fatal("pointer element metadata is incorrect")
	}
	if ptr.Elem() != reflect2.TypeOf("") || ptr.Elem() != reflect2.Type2(reflect.TypeOf("")) {
		t.Fatal("type wrappers are not canonical")
	}
	if reflect2.TypeOf(nil) != nil || reflect2.Type2(nil) != nil || !reflect2.IsNil((*string)(nil)) {
		t.Fatal("nil type or value metadata is incorrect")
	}
	if _, ok := reflect2.TypeOf("").(reflect2.MapType); ok {
		t.Fatal("scalar wrapper incorrectly implements MapType")
	}
	if _, ok := reflect2.TypeOf("").(reflect2.PtrType); ok {
		t.Fatal("scalar wrapper incorrectly implements PtrType")
	}
	if (reflect2.Config{UseSafeImplementation: true}).Froze().TypeOf("") == reflect2.ConfigUnsafe.TypeOf("") {
		t.Fatal("independent configurations share mutable wrapper state")
	}
}

func TestReflect2PerMachineState(t *testing.T) {
	results := make(chan reflect2.Type, 2)
	for i := 0; i < 2; i++ {
		m, err := gosim.NewMachine(gosim.MachineConfig{Label: "reflect2", Main: func() {
			results <- reflect2.TypeOfPtr((*string)(nil)).Elem()
		}})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Start(); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	if left, right := <-results, <-results; left == right {
		t.Fatal("machine-local reflect2 wrappers were shared")
	}
}

func TestDisabledObservabilityInitialization(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("runtime tracing became active")
	}
	if runtime.SetMutexProfileFraction(-1) != 0 {
		t.Fatal("host mutex profiling became active")
	}
	provider := sdktrace.NewTracerProvider()
	ctx, span := provider.Tracer("etcd-probe").Start(context.Background(), "disabled-runtime-trace")
	span.End()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if prometheus.DefaultGatherer == nil || prometheus.NewGoCollector() == nil {
		t.Fatal("Prometheus collector registration did not initialize")
	}
}
