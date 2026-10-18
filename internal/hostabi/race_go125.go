// Copyright 2015 The Go Authors. All rights reserved.  Use of this source code
// is governed by a BSD-style license that can be found at
// https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

//go:build go1.25 && !go1.26 && race

package hostabi

import (
	"unsafe"

	"github.com/neria-cloud/gosim/internal/compat/go125/host"
)

const RaceEnabled = host.RaceEnabled

func RaceAcquire(addr unsafe.Pointer) { host.RaceAcquire(addr) }

func RaceRelease(addr unsafe.Pointer) { host.RaceRelease(addr) }

func RaceReleaseMerge(addr unsafe.Pointer) { host.RaceReleaseMerge(addr) }

func RaceDisable() { host.RaceDisable() }

func RaceEnable() { host.RaceEnable() }

func RaceRead(addr unsafe.Pointer) { host.RaceRead(addr) }

func RaceReadPC(addr unsafe.Pointer, callerpc, pc uintptr) { host.RaceReadPC(addr, callerpc, pc) }

func RaceReadObjectPC(t any, addr unsafe.Pointer, callerpc, pc uintptr) {
	host.RaceReadObjectPC(t, addr, callerpc, pc)
}

func RaceWrite(addr unsafe.Pointer) { host.RaceWrite(addr) }

func RaceWritePC(addr unsafe.Pointer, callerpc, pc uintptr) { host.RaceWritePC(addr, callerpc, pc) }

func RaceWriteObjectPC(t any, addr unsafe.Pointer, callerpc, pc uintptr) {
	host.RaceWriteObjectPC(t, addr, callerpc, pc)
}

func RaceReadRange(addr unsafe.Pointer, len int) { host.RaceReadRange(addr, len) }

func RaceWriteRange(addr unsafe.Pointer, len int) { host.RaceWriteRange(addr, len) }

func RaceErrors() int { return host.RaceErrors() }
