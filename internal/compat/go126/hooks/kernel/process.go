//go:build go1.26 && !go1.27

package kernel

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/neria-cloud/gosim/internal/contracts"
	kernel "github.com/neria-cloud/gosim/internal/kernel"
	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

// Translated machine-local state, deliberately without an init expression.
// The generated boot callback configures it before running package initializers.
// Composition configures root startup; each later machine boot uses the same process setup.
var process struct {
	args, env []string
	update    func(string, string)
}

// ConfigureProcess is harness-only. Build policy currently permits empty debug
// defaults only; host argv, environment and runtime debug defaults are never read.
func ConfigureProcess(args, env []string) error {
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || strings.ContainsRune(entry, 0) {
			return contracts.ErrInvalidConfig
		}
	}
	process.args = append([]string(nil), args...)
	process.env = append([]string(nil), env...)
	process.update = nil
	return nil
}

func Os_runtime_args() []string {
	if len(process.args) == 0 {
		return []string{"gosimapp"}
	}
	return append([]string(nil), process.args...)
}
func Syscall_runtime_envs() []string { return append([]string(nil), process.env...) }
func Syscall_runtimeSetenv(k, v string) {
	// syscall validates names/values before this runtime hook.
	removeEnv(k)
	process.env = append(process.env, k+"="+v)
	if k == "GODEBUG" {
		updateDebug()
	}
}

func Syscall_runtimeUnsetenv(k string) {
	removeEnv(k)
	if k == "GODEBUG" {
		updateDebug()
	}
}

// The incoming map belongs to the translated syscall package. Do not pass it
// into a native runtime function or mutate it; syscall clears its own state.
func Syscall_runtimeClearenv(env map[string]int) {
	for key := range env {
		removeEnv(key)
	}
	if _, ok := env["GODEBUG"]; ok {
		updateDebug()
	}
}

func removeEnv(k string) {
	for i := 0; i < len(process.env); {
		key, _, _ := strings.Cut(process.env[i], "=")
		if key == k {
			process.env = append(process.env[:i], process.env[i+1:]...)
		} else {
			i++
		}
	}
}

func updateDebug() {
	if process.update == nil {
		return
	}
	value := ""
	for _, entry := range process.env {
		if v, ok := strings.CutPrefix(entry, "GODEBUG="); ok {
			value = v
			break
		}
	}
	process.update("", value)
}
func InternalGodebug_setUpdate(update func(string, string)) { process.update = update; updateDebug() }
func InternalGodebug_setNewIncNonDefault(newIncNonDefault func(string) func()) {
	// Named stub: runtime metrics are not exposed; retain no callback.
}

func InternalGodebug_registerMetric(name string, read func() uint64) {
	// Named stub: no host runtime metric sampler or callback retention.
}

func InternalGodebug_write(fd uintptr, p unsafe.Pointer, n int32) int32 {
	if fd != 2 || n < 0 || (p == nil && n != 0) {
		return -1
	}
	if n > 0 {
		simrt.RaceReadRange(p, int(n))
	}
	result, err := Call(kernel.WriteRequest{FD: 2, From: abi.NewSliceView((*byte)(p), uintptr(n))}, kernel.DecodeWriteResult)
	if err != nil {
		return -1
	}
	return int32(result.Count)
}
func Syscall_Getpagesize() int       { return 4096 }
func Os_runtime_rand() uint64        { return simrt.Fastrand64() }
func Os_checkClonePidfd() error      { Reject("os.checkClonePidfd"); return contracts.ErrInvalidState }
func Os_sigpipe()                    { Reject("os.sigpipe") }
func Os_runtime_beforeExit(code int) { Reject("os.Exit") }

// Os_Executable reports the absence of an executable image in a simulated
// process. In particular, protobuf must not hash the host executable at init.
func Os_Executable() (string, error) { return "", syscall.ENOENT }
