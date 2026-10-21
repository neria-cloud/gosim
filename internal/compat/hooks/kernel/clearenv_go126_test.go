//go:build go1.26

package kernel

import (
	"os"
	"reflect"
	"testing"

	"github.com/neria-cloud/gosim/internal/contracts"
)

func TestClearenvPreservesCallerMapAndHost(t *testing.T) {
	t.Setenv("GOSIM_PROCESS_HOOK_TEST", "host")
	if err := runAdapters(t, contracts.FeatureSet(contracts.RT), func() {
		if err := ConfigureProcess(nil, []string{"GODEBUG=test=1", "GOSIM_PROCESS_HOOK_TEST=sim"}); err != nil {
			t.Fatal(err)
		}
		var updates []string
		InternalGodebug_setUpdate(func(_, env string) { updates = append(updates, env) })
		env := map[string]int{"GODEBUG": 0, "GOSIM_PROCESS_HOOK_TEST": 1}
		Syscall_runtimeClearenv(env)
		if len(Syscall_runtime_envs()) != 0 || len(env) != 2 || !reflect.DeepEqual(updates, []string{"test=1", ""}) {
			t.Fatalf("env=%v caller=%v updates=%v", Syscall_runtime_envs(), env, updates)
		}
		Syscall_runtimeClearenv(nil)
		if len(updates) != 2 {
			t.Fatal("empty clearenv sent spurious debug update")
		}
		if os.Getenv("GOSIM_PROCESS_HOOK_TEST") != "host" {
			t.Fatal("host environment cleared")
		}
	}); err != nil {
		t.Fatal(err)
	}
}
