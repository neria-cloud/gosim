//go:build sim

package disabledfs

import (
	"os"
	"testing"
)

func disabledOperation() {
	defer func() { recover(); panic("disabled operation unwound") }()
	_ = os.WriteFile(os.Getenv("HOST_PROBE"), []byte("must never reach host"), 0o600)
	panic("disabled operation returned")
}

func init() {
	if os.Getenv("DISABLED_PHASE") == "init" {
		disabledOperation()
	}
}
func TestDisabled(t *testing.T) { disabledOperation() }
