// Copyright 2015 The Go Authors. All rights reserved.  Use of this source code
// is governed by a BSD-style license that can be found at
// https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

//go:build go1.25 && !go1.26 && race

package host

import (
	"runtime"
	"unsafe"
)

const RaceEnabled = true

func RaceAcquire(addr unsafe.Pointer) {
	runtime.RaceAcquire(addr)
}

func RaceRelease(addr unsafe.Pointer) {
	runtime.RaceRelease(addr)
}

func RaceReleaseMerge(addr unsafe.Pointer) {
	runtime.RaceReleaseMerge(addr)
}

func RaceDisable() {
	runtime.RaceDisable()
}

func RaceEnable() {
	runtime.RaceEnable()
}

func RaceRead(addr unsafe.Pointer) {
	runtime.RaceRead(addr)
}

func RaceReadPC(addr unsafe.Pointer, callerpc, pc uintptr) {
	runtime.RaceRead(addr)
}

func RaceReadObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr) {
	runtimeRaceReadObjectPC(t, addr, callerpc, pc)
}

func RaceWrite(addr unsafe.Pointer) {
	runtime.RaceWrite(addr)
}

func RaceWritePC(addr unsafe.Pointer, callerpc, pc uintptr) {
	runtime.RaceWrite(addr)
}

func RaceWriteObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr) {
	runtimeRaceWriteObjectPC(t, addr, callerpc, pc)
}

func RaceReadRange(addr unsafe.Pointer, len int) {
	runtime.RaceReadRange(addr, len)
}

func RaceWriteRange(addr unsafe.Pointer, len int) {
	runtime.RaceWriteRange(addr, len)
}

func RaceErrors() int {
	return runtime.RaceErrors()
}

// The selected runtime pushes these symbols into internal/race. Type metadata
// crosses only as an opaque native pointer; the runtime owns its exact layout.
//
//go:linkname runtimeRaceReadObjectPC internal/race.ReadObjectPC
func runtimeRaceReadObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr)

//go:linkname runtimeRaceWriteObjectPC internal/race.WriteObjectPC
func runtimeRaceWriteObjectPC(t unsafe.Pointer, addr unsafe.Pointer, callerpc, pc uintptr)
