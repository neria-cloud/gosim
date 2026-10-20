// Copyright 2015 The Go Authors. All rights reserved.  Use of this source code
// is governed by a BSD-style license that can be found at
// https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

//go:build go1.25 && !go1.26

package runtimebridge

import (
	"unsafe"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
)

// InternalRaceEnabled is the translated stdlib race constant.
const InternalRaceEnabled = gosimruntime.RaceEnabled

func InternalRace_Acquire(addr unsafe.Pointer) { gosimruntime.RaceAcquire(addr) }

func InternalRace_Release(addr unsafe.Pointer) { gosimruntime.RaceRelease(addr) }

func InternalRace_ReleaseMerge(addr unsafe.Pointer) { gosimruntime.RaceReleaseMerge(addr) }

func InternalRace_Disable() { gosimruntime.RaceDisable() }

func InternalRace_Enable() { gosimruntime.RaceEnable() }

func InternalRace_Read(addr unsafe.Pointer) { gosimruntime.RaceRead(addr) }

func InternalRace_ReadPC(addr unsafe.Pointer, callerpc, pc uintptr) {
	gosimruntime.RaceReadPC(addr, callerpc, pc)
}

func InternalRace_ReadObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr) {
	gosimruntime.RaceReadObjectPC(t, addr, callerpc, pc)
}

func InternalRace_Write(addr unsafe.Pointer) { gosimruntime.RaceWrite(addr) }

func InternalRace_WritePC(addr unsafe.Pointer, callerpc, pc uintptr) {
	gosimruntime.RaceWritePC(addr, callerpc, pc)
}

func InternalRace_WriteObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr) {
	gosimruntime.RaceWriteObjectPC(t, addr, callerpc, pc)
}

func InternalRace_ReadRange(addr unsafe.Pointer, len int) { gosimruntime.RaceReadRange(addr, len) }

func InternalRace_WriteRange(addr unsafe.Pointer, len int) { gosimruntime.RaceWriteRange(addr, len) }

func InternalRace_Errors() int { return gosimruntime.RaceErrors() }
