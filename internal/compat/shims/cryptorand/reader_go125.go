//go:build go1.25 && !go1.26

package rand

import "io"

func primeReader(rand io.Reader) io.Reader { return rand }
