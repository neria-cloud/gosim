// Copyright 2015 The Go Authors. All rights reserved.  Use of this source code
// is governed by a BSD-style license that can be found at
// https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

//go:build go1.25 && !go1.26 && !race

package host

import (
	"unsafe"
)

const RaceEnabled = false

func RaceAcquire(addr unsafe.Pointer) {
}

func RaceRelease(addr unsafe.Pointer) {
}

func RaceReleaseMerge(addr unsafe.Pointer) {
}

func RaceDisable() {
}

func RaceEnable() {
}

func RaceRead(addr unsafe.Pointer) {
}

func RaceReadPC(addr unsafe.Pointer, callerpc, pc uintptr) {
}

func RaceReadObjectPC(t any, addr unsafe.Pointer, callerpc, pc uintptr) {
}

func RaceWrite(addr unsafe.Pointer) {
}

func RaceWritePC(addr unsafe.Pointer, callerpc, pc uintptr) {
}

func RaceWriteObjectPC(t any, addr unsafe.Pointer, callerpc, pc uintptr) {
}

func RaceReadRange(addr unsafe.Pointer, len int) {
}

func RaceWriteRange(addr unsafe.Pointer, len int) {
}

func RaceErrors() int {
	return 0
}
