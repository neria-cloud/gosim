//go:build go1.26

package rand

import (
	"errors"
	"testing"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func TestGo126PrimeDefaultIgnoresCustomReader(t *testing.T) {
	t.Setenv("GODEBUG", "cryptocustomrand=1,cryptocustomrand=0")
	previous := Reader
	Reader = failureReader{errors.New("public reader must not be used by default Prime")}
	defer func() { Reader = previous }()
	e, err := gosimruntime.NewEngine(gosimruntime.EngineConfig{Seed: 77, TimeoutNS: 1e9, ReplayRevision: gosimruntime.ReplayRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.Run(func() {
		prime, err := Prime(failureReader{errors.New("supplied reader must be ignored")}, 17)
		if err != nil || prime.BitLen() != 17 || !prime.ProbablyPrime(20) {
			t.Fatalf("Prime=%v,%v", prime, err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}
