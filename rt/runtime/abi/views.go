// Package abi defines the untranslated memory and identity boundary used by
// generated code. It imports no domain contracts or providers.
package abi

import "unsafe"

// BoolToUintptr stores a boolean as a uintptr for syscall arguments
// or return values.
func BoolToUintptr(v bool) uintptr {
	if v {
		return 1
	}
	return 0
}

// BoolFromUintptr extracts a boolean stored as a uintptr for syscall
// arguments or return values.
func BoolFromUintptr(v uintptr) bool {
	return v != 0
}

// TODO: for the Views below we could register read/write race detector deps in
// userspace?

// SliceView is a OS-wrapper around slices passed from user space.  It lets
// the OS access user space memory without triggering the race detector.
type SliceView[T any] struct {
	Ptr []T
}

func NewSliceView[T any](ptr *T, len uintptr) SliceView[T] {
	return SliceView[T]{
		Ptr: unsafe.Slice(ptr, len),
	}
}

func (b SliceView[T]) Len() int {
	return len(b.Ptr)
}

//go:norace
func (b SliceView[T]) Read(into []T) int {
	n := len(into)
	if len(b.Ptr) < n {
		n = len(b.Ptr)
	}
	for i := range b.Ptr[:n] {
		into[i] = b.Ptr[i]
	}
	return n
}

//go:norace
func (b SliceView[T]) Write(from []T) int {
	n := len(b.Ptr)
	if len(from) < n {
		n = len(from)
	}
	for i := range from[:n] {
		b.Ptr[i] = from[i]
	}
	return n
}

func (b SliceView[T]) SliceFrom(from int) SliceView[T] {
	return SliceView[T]{Ptr: b.Ptr[from:]}
}

func (b SliceView[T]) Slice(from, to int) SliceView[T] {
	return SliceView[T]{Ptr: b.Ptr[from:to]}
}

type ByteView = SliceView[byte]

// ValueView is a OS-wrapper around pointers passed from user space. It lets the
// OS access user space memory without triggering the race detector.
type ValueView[T any] struct {
	underlying *T
}

func NewValueView[T any](ptr *T) ValueView[T] {
	return ValueView[T]{
		underlying: ptr,
	}
}

func (s ValueView[T]) UnsafePointer() unsafe.Pointer {
	return unsafe.Pointer(s.underlying)
}

//go:norace
func (s ValueView[T]) Get() T {
	return *s.underlying
}

//go:norace
func (s ValueView[T]) Set(v T) {
	*s.underlying = v
}
