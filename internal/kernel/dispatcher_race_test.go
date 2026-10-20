//go:build race

package kernel

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
)

//go:noinline
func applicationRaceWrite(value *int) { *value = 1 }

//go:noinline
func applicationRaceRead(value *int) int { return *value }

func TestSyscallTransportDoesNotHideApplicationRace(t *testing.T) {
	if os.Getenv("GOSIM_DISPATCH_RACE_CHILD") == "1" {
		e := engineFor(t)
		var shared, observed int
		if err := e.Run(func() {
			k, err := New(configFor(contracts.FeatureSet(contracts.RT)), e, nil)
			if err != nil {
				t.Fatal(err)
			}
			call := func() {
				if err := k.dispatcher.Submit(&Invocation{Context: currentContext(k), Route: RouteGetpid, Args: [5]any{GetpidRequest{}}}); err != nil {
					t.Fatal(err)
				}
			}
			writer := createFor(t, k, "writer", func() { applicationRaceWrite(&shared); call() })
			reader := createFor(t, k, "reader", func() { call(); observed = applicationRaceRead(&shared) })
			if err := k.Start(writer); err != nil {
				t.Fatal(err)
			}
			if err := k.Start(reader); err != nil {
				t.Fatal(err)
			}
			if err := k.Run(func() { simrt.Sleep(1000) }); err != nil {
				t.Fatal(err)
			}
		}); err != nil {
			t.Fatal(err)
		}
		if observed < 0 {
			t.Fatal(observed)
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestSyscallTransportDoesNotHideApplicationRace$")
	cmd.Env = append(os.Environ(), "GOSIM_DISPATCH_RACE_CHILD=1", "GORACE=halt_on_error=1")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "WARNING: DATA RACE") || !strings.Contains(string(output), "applicationRaceRead") || !strings.Contains(string(output), "applicationRaceWrite") {
		t.Fatalf("syscall transport hid or replaced application race: %v\n%s", err, output)
	}
}
