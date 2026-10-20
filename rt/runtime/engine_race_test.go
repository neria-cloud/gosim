//go:build race

package gosimruntime

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Supervisor/event synchronization must not create a happens-before edge
// between unrelated application machines. Run the deliberate race in a child.
func TestEngineApplicationRaceRemainsDetectable(t *testing.T) {
	if os.Getenv("GOSIM_ENGINE_RACE_CHILD") == "1" {
		e, err := NewEngine(EngineConfig{Seed: 1, TimeoutNS: int64(time.Second), ReplayRevision: ReplayRevision})
		if err != nil {
			t.Fatal(err)
		}
		var value int
		err = e.Run(func() {
			for id := uint64(1); id <= 2; id++ {
				token, err := e.PrepareMachine(id, 1, "racing application")
				if err != nil {
					t.Fatal(err)
				}
				e.PublishMachine(token, func() { value++ })
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = e.Close()
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestEngineApplicationRaceRemainsDetectable$")
	cmd.Env = append(os.Environ(), "GOSIM_ENGINE_RACE_CHILD=1", "GORACE=halt_on_error=1")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "WARNING: DATA RACE") {
		t.Fatalf("new engine hid an application race: %v\n%s", err, output)
	}
}
