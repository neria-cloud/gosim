//go:build sim && go1.26

package behavior_test

import (
	"crypto/rand"
	"errors"
	"testing"
)

func TestCryptoPrimeInternalDefault(t *testing.T) {
	t.Setenv("GODEBUG", "cryptocustomrand=1,cryptocustomrand=0")
	previous := rand.Reader
	t.Cleanup(func() { rand.Reader = previous })
	rand.Reader = primeFailureReader{errors.New("public reader must be ignored")}
	prime, err := rand.Prime(primeFailureReader{errors.New("supplied reader must be ignored")}, 17)
	if err != nil || prime.BitLen() != 17 || !prime.ProbablyPrime(20) {
		t.Fatal("internal deterministic reader", prime, err)
	}
}
