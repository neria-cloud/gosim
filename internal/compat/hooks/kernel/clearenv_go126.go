//go:build go1.26

package kernel

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
