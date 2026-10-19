//go:build gosim_retired

package go125

import "github.com/neria-cloud/gosim/rt/runtime"

func Os_runtime_args() []string {
	// TODO: make this configurable / fetch this from the machine API?
	return []string{"gosimapp"}
}

func Os_sigpipe() {
	panic("gosim not implemented")
}

func Os_runtime_beforeExit(exitCode int) {
	panic("gosim not implemented")
}

func Os_runtime_rand() uint64 {
	return gosimruntime.Fastrand64()
}

func Os_checkClonePidfd() {
	panic("gosim not implemented")
}

func Os_ignoreSIGSYS() {
	panic("gosim not implemented")
}

func Os_restoreSIGSYS() {
	panic("gosim not implemented")
}
