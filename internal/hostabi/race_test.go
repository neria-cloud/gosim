//go:build race && go1.25

package hostabi

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

// Explicit annotations must remain visible through both layers of the facade.
// This child intentionally races; a clean parent asserts that it was detected.
func TestRaceOracle(t *testing.T) {
	if os.Getenv("GOSIM_HOSTABI_RACE_CHILD") == "1" {
		var value uint64
		var done sync.WaitGroup
		done.Add(2)
		for i := 0; i < 2; i++ {
			go func() { defer done.Done(); RaceWrite(unsafe.Pointer(&value)) }()
		}
		done.Wait()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRaceOracle$")
	cmd.Env = append(os.Environ(), "GOSIM_HOSTABI_RACE_CHILD=1")
	data, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(data), "WARNING: DATA RACE") {
		t.Fatalf("race annotation was lost: %v\n%s", err, data)
	}
	if !RaceEnabled || RaceErrors() != 0 {
		t.Fatal("parent race state invalid")
	}
}
