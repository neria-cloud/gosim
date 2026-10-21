//go:build sim

package behavior_test

import (
	"crypto/rand"
	"errors"
	"testing"
)

type primeFailureReader struct{ err error }

func (r primeFailureReader) Read([]byte) (int, error) { return 0, r.err }

func TestCryptoPrimeCustomReader(t *testing.T) {
	// The last setting wins in Go 1.26; Go 1.25 always uses the caller's reader.
	t.Setenv("GODEBUG", "cryptocustomrand=0,cryptocustomrand=1")
	expected := errors.New("supplied reader failure")
	if _, err := rand.Prime(primeFailureReader{expected}, 17); !errors.Is(err, expected) {
		t.Fatal("custom reader not used", err)
	}
	prime, err := rand.Prime(rand.Reader, 17)
	if err != nil || prime.BitLen() != 17 || !prime.ProbablyPrime(20) {
		t.Fatal("deterministic prime", prime, err)
	}
}
