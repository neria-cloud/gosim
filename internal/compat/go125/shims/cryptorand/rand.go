//go:build go1.25 && !go1.26

// Package rand provides a deterministic replacement for crypto/rand
// when running under gosim simulation.
//
// This version-owned shim uses the application's deterministic random stream.
// Its io and math/big values stay translated with the caller; it never reads
// host entropy and must not be used to generate real secrets.
package rand

import (
	"io"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type deterministicReader struct{}

func (_ *deterministicReader) Read(b []byte) (int, error) {
	for i := 0; i < len(b); {
		v := gosimruntime.Fastrand64()
		for j := 0; j < 8 && i < len(b); j++ {
			b[i] = byte(v)
			v >>= 8
			i++
		}
	}
	return len(b), nil
}

// Reader is a deterministic random reader for gosim simulation.
var Reader io.Reader = &deterministicReader{}

// Read fills b with deterministic random bytes from gosim's PRNG.
func Read(b []byte) (int, error) {
	if _, err := io.ReadFull(Reader, b); err != nil {
		gosimruntime.Abort(abi.Diagnostic{
			Kind: "RuntimeFatal", Feature: "rt", Operation: "crypto/rand.Read",
			Message: "failed to read random data: " + err.Error(),
		})
	}
	return len(b), nil
}
