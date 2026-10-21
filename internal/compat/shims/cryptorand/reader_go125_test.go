//go:build go1.25 && !go1.26

package rand

import (
	"errors"
	"testing"
)

func TestPrimeBaselineUsesCustomReader(t *testing.T) {
	t.Setenv("GODEBUG", "cryptocustomrand=0")
	expected := errors.New("supplied reader")
	if _, err := Prime(failureReader{expected}, 17); !errors.Is(err, expected) {
		t.Fatal(err)
	}
}
