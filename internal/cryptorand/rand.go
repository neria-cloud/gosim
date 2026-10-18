// Package rand provides a deterministic replacement for crypto/rand
// when running under gosim simulation.
//
// The real crypto/rand is skipped (not translated) because it imports
// crypto/internal/fips140/drbg which has untranslatable runtime linknames.
// This replacement uses gosim's deterministic PRNG so that simulated code
// gets reproducible "random" bytes.
package rand

import (
	"io" //gosim:notranslate

	"github.com/neria-cloud/gosim/gosimruntime"
)

type deterministicReader struct{}

func (r *deterministicReader) Read(b []byte) (int, error) {
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
	return io.ReadFull(Reader, b)
}
