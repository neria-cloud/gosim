// Race annotations used by translated version hooks cross the public runtime
// boundary as pointers and integers only; private hostabi stays inside this module.
package gosimruntime

import (
	"unsafe"

	"github.com/neria-cloud/gosim/internal/hostabi"
)

const RaceEnabled = hostabi.RaceEnabled

func RaceAcquire(addr unsafe.Pointer) { hostabi.RaceAcquire(addr) }

func RaceRelease(addr unsafe.Pointer) { hostabi.RaceRelease(addr) }

func RaceReleaseMerge(addr unsafe.Pointer) { hostabi.RaceReleaseMerge(addr) }

func RaceDisable() { hostabi.RaceDisable() }

func RaceEnable() { hostabi.RaceEnable() }

func RaceRead(addr unsafe.Pointer) { hostabi.RaceRead(addr) }

func RaceReadPC(addr unsafe.Pointer, callerpc, pc uintptr) { hostabi.RaceReadPC(addr, callerpc, pc) }

func RaceWrite(addr unsafe.Pointer) { hostabi.RaceWrite(addr) }

func RaceWritePC(addr unsafe.Pointer, callerpc, pc uintptr) { hostabi.RaceWritePC(addr, callerpc, pc) }

func RaceReadRange(addr unsafe.Pointer, len int) { hostabi.RaceReadRange(addr, len) }

func RaceWriteRange(addr unsafe.Pointer, len int) { hostabi.RaceWriteRange(addr, len) }

func RaceErrors() int { return hostabi.RaceErrors() }
