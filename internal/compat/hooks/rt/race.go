//go:build go1.25

package rt

import (
	"unsafe"

	bridge "github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge"
)

const InternalRaceEnabled = bridge.InternalRaceEnabled

func InternalRace_Acquire(addr unsafe.Pointer)      { bridge.InternalRace_Acquire(addr) }
func InternalRace_Release(addr unsafe.Pointer)      { bridge.InternalRace_Release(addr) }
func InternalRace_ReleaseMerge(addr unsafe.Pointer) { bridge.InternalRace_ReleaseMerge(addr) }
func InternalRace_Disable()                         { bridge.InternalRace_Disable() }
func InternalRace_Enable()                          { bridge.InternalRace_Enable() }
func InternalRace_Read(addr unsafe.Pointer)         { bridge.InternalRace_Read(addr) }
func InternalRace_ReadPC(addr unsafe.Pointer, callerpc, pc uintptr) {
	bridge.InternalRace_ReadPC(addr, callerpc, pc)
}

func InternalRace_ReadObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr) {
	bridge.InternalRace_ReadObjectPC(t, addr, callerpc, pc)
}
func InternalRace_Write(addr unsafe.Pointer) { bridge.InternalRace_Write(addr) }
func InternalRace_WritePC(addr unsafe.Pointer, callerpc, pc uintptr) {
	bridge.InternalRace_WritePC(addr, callerpc, pc)
}

func InternalRace_WriteObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr) {
	bridge.InternalRace_WriteObjectPC(t, addr, callerpc, pc)
}
func InternalRace_ReadRange(addr unsafe.Pointer, len int)  { bridge.InternalRace_ReadRange(addr, len) }
func InternalRace_WriteRange(addr unsafe.Pointer, len int) { bridge.InternalRace_WriteRange(addr, len) }
func InternalRace_Errors() int                             { return bridge.InternalRace_Errors() }
