// Copyright 2015 The Go Authors. All rights reserved.  Use of this source code
// is governed by a BSD-style license that can be found at
// https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

//go:build go1.25 && !go1.26 && !race

package host

import (
	"unsafe"
)

const RaceEnabled = false

func RaceAcquire(_ unsafe.Pointer) {
}

func RaceRelease(_ unsafe.Pointer) {
}

func RaceReleaseMerge(_ unsafe.Pointer) {
}

func RaceDisable() {
}

func RaceEnable() {
}

func RaceRead(_ unsafe.Pointer) {
}

func RaceReadPC(_ unsafe.Pointer, _, pc uintptr) {
}

func RaceReadObjectPC(_ unsafe.Pointer, _ unsafe.Pointer, _, pc uintptr) {
}

func RaceWrite(_ unsafe.Pointer) {
}

func RaceWritePC(_ unsafe.Pointer, _, pc uintptr) {
}

func RaceWriteObjectPC(_ unsafe.Pointer, _ unsafe.Pointer, _, pc uintptr) {
}

func RaceReadRange(_ unsafe.Pointer, len int) {
}

func RaceWriteRange(_ unsafe.Pointer, len int) {
}

func RaceErrors() int {
	return 0
}
