//go:build go1.25 && !go1.26

package rand

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

func TestDeterministicRandomSurface(t *testing.T) {
	sample := func() []byte {
		e, err := gosimruntime.NewEngine(
			gosimruntime.EngineConfig{Seed: 77, TimeoutNS: 1e9, ReplayRevision: gosimruntime.ReplayRevision},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		var result []byte
		if err := e.Run(func() {
			buf := make([]byte, 19)
			if n, err := Read(buf); n != len(buf) || err != nil {
				t.Fatalf("Read=%d,%v", n, err)
			}
			result = append(result, buf...)
			n, err := Int(Reader, big.NewInt(129))
			if err != nil || n.Sign() < 0 || n.Cmp(big.NewInt(129)) >= 0 {
				t.Fatalf("Int=%v,%v", n, err)
			}
			result = append(result, n.Bytes()...)
			prime, err := Prime(Reader, 17)
			if err != nil || prime.BitLen() != 17 || !prime.ProbablyPrime(20) {
				t.Fatalf("Prime=%v,%v", prime, err)
			}
			result = append(result, prime.Bytes()...)
			text := Text()
			if len(text) != 26 {
				t.Fatalf("Text length=%d", len(text))
			}
			for _, c := range text {
				if c < 'A' || c > 'Z' {
					if c < '2' || c > '7' {
						t.Fatalf("Text character=%c", c)
					}
				}
			}
			result = append(result, text...)
		}); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if !bytes.Equal(sample(), sample()) {
		t.Fatal("random API is not replayable")
	}
}

type failureReader struct{ err error }

func (r failureReader) Read([]byte) (int, error) { return 0, r.err }
func TestReaderErrorsAndBounds(t *testing.T) {
	expected := errors.New("reader failed")
	if _, err := Int(failureReader{expected}, big.NewInt(9)); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	if _, err := Prime(failureReader{expected}, 17); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	if _, err := Prime(failureReader{expected}, 1); err == nil {
		t.Fatal("invalid prime size accepted")
	}
	defer func() {
		if recover() == nil {
			t.Error("invalid Int maximum accepted")
		}
	}()
	Int(failureReader{expected}, big.NewInt(0))
}

func TestReadFailureAbortsWithoutRecovery(t *testing.T) {
	e, err := gosimruntime.NewEngine(
		gosimruntime.EngineConfig{Seed: 1, TimeoutNS: 1e9, ReplayRevision: gosimruntime.ReplayRevision},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	previous := Reader
	defer func() { Reader = previous }()
	Reader = failureReader{errors.New("entropy reader failed")}
	after, deferred := false, false
	err = e.Run(func() {
		token, err := e.PrepareMachine(1, 1, "reader")
		if err != nil {
			t.Fatal(err)
		}
		e.PublishMachine(token, func() {
			defer func() { _ = recover(); deferred = true }()
			Read(make([]byte, 1))
			after = true
		})
		gosimruntime.NewChan[struct{}](0).Recv()
	})
	var aborted *gosimruntime.AbortError
	if !errors.As(err, &aborted) || aborted.Diagnostic.Operation != "crypto/rand.Read" || after || deferred {
		t.Fatalf("fatal read err=%v after=%v defer=%v", err, after, deferred)
	}
}
