//go:build sim

package disablednet

import (
	"net"
	"os"
	"testing"
)

func disabledOperation() {
	defer func() { recover(); panic("disabled operation unwound") }()
	_, _ = net.Dial("tcp", os.Getenv("HOST_PROBE"))
	panic("disabled operation returned")
}

func init() {
	if os.Getenv("DISABLED_PHASE") == "init" {
		disabledOperation()
	}
}
func TestDisabled(t *testing.T) { disabledOperation() }
