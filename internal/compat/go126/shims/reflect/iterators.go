//go:build go1.26 && !go1.27

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package reflect

import "iter" //gosim:notranslate

func (t *typeImpl) Fields() iter.Seq[StructField] {
	if t.Kind() != Struct {
		panic("reflect: Fields of non-struct type " + t.String())
	}
	return func(yield func(StructField) bool) {
		for i := range t.NumField() {
			if !yield(t.Field(i)) {
				return
			}
		}
	}
}

func (t *typeImpl) Methods() iter.Seq[Method] {
	return func(yield func(Method) bool) {
		for i := range t.NumMethod() {
			if !yield(t.Method(i)) {
				return
			}
		}
	}
}

func (t *typeImpl) Ins() iter.Seq[Type] {
	if t.Kind() != Func {
		panic("reflect: Ins of non-func type " + t.String())
	}
	return func(yield func(Type) bool) {
		for i := range t.NumIn() {
			if !yield(t.In(i)) {
				return
			}
		}
	}
}

func (t *typeImpl) Outs() iter.Seq[Type] {
	if t.Kind() != Func {
		panic("reflect: Outs of non-func type " + t.String())
	}
	return func(yield func(Type) bool) {
		for i := range t.NumOut() {
			if !yield(t.Out(i)) {
				return
			}
		}
	}
}

// Fields returns an iterator over each [StructField] of v along with its [Value].
//
// The sequence is equivalent to calling [Value.Field] successively
// for each index i in the range [0, NumField()).
//
// It panics if v's Kind is not Struct.
func (v Value) Fields() iter.Seq2[StructField, Value] {
	t := v.Type()
	if t.Kind() != Struct {
		panic("reflect: Fields of non-struct type " + t.String())
	}
	return func(yield func(StructField, Value) bool) {
		for i := range v.NumField() {
			if !yield(t.Field(i), v.Field(i)) {
				return
			}
		}
	}
}

// Methods returns an iterator over each [Method] of v along with the corresponding
// method [Value]; this is a function with v bound as the receiver. As such, the
// receiver shouldn't be included in the arguments to [Value.Call].
//
// The sequence is equivalent to calling [Value.Method] successively
// for each index i in the range [0, NumMethod()).
//
// Methods panics if v is a nil interface value.
//
// Calling this method will force the linker to retain all exported methods in all packages.
// This may make the executable binary larger but will not affect execution time.
func (v Value) Methods() iter.Seq2[Method, Value] {
	return func(yield func(Method, Value) bool) {
		rtype := v.Type()
		for i := range v.NumMethod() {
			if !yield(rtype.Method(i), v.Method(i)) {
				return
			}
		}
	}
}
