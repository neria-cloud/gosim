// Copyright 2023 The Go Authors. All rights reserved.  Use of this source code
// is governed by a BSD-style license that can be found at
// https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

package gosimruntime

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/bits"
)

//go:norace
func Fastrand64() uint64 {
	if inControlledNondeterminism() {
		return 0
	}
	return gs.get().engine.RandomFor("application", CurrentMachineToken().ID).Uint64()
}

//go:norace
func Fastrandn(n uint32) uint32 {
	if inControlledNondeterminism() {
		return 0
	}
	return uint32(gs.get().engine.RandomFor("application", CurrentMachineToken().ID).Uint64N(uint64(n)))
}

// Random is an owned domain stream. Its initial state and update algorithm are
// versioned independently from the host Go standard library's random package.
type Random struct{ state uint64 }

const RandomRevision = "gosim-rng-v2"

const (
	randomStateIncrement = 0xa0761d6478bd642f
	randomFloatBits      = 53
)

func NewDomainRandom(seed int64, domain string, machineID uint64) *Random {
	if domain != "scheduler" && domain != "application" && domain != "fs" && domain != "net" {
		panic("unknown random domain: " + domain)
	}
	var seedBytes, machineBytes [8]byte
	binary.LittleEndian.PutUint64(seedBytes[:], uint64(seed))
	binary.LittleEndian.PutUint64(machineBytes[:], machineID)
	h := sha256.New()
	for _, part := range [][]byte{[]byte(RandomRevision), seedBytes[:], []byte(domain), machineBytes[:]} {
		var length [8]byte
		binary.LittleEndian.PutUint64(length[:], uint64(len(part)))
		h.Write(length[:])
		h.Write(part)
	}
	return &Random{state: binary.LittleEndian.Uint64(h.Sum(nil)[:8])}
}

//go:norace
func (r *Random) Uint64() uint64 {
	r.state += randomStateIncrement
	hi, lo := bits.Mul64(r.state, r.state^0xe7037ed1a0b428db)
	return hi ^ lo
}

func (r *Random) Uint64N(n uint64) uint64 {
	if n == 0 {
		panic("random bound must be positive")
	}
	threshold := -n % n
	for {
		x := r.Uint64()
		if x >= threshold {
			return x % n
		}
	}
}

func (r *Random) Float64() float64    { return float64(r.Uint64()>>11) / (1 << randomFloatBits) }
func (r *Random) ExpFloat64() float64 { return -math.Log1p(-r.Float64()) }

type fastrander struct {
	state uint64
}

func (f *fastrander) fastrandn(n uint32) uint32 {
	// This is similar to fastrand() % n, but faster.
	// See https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
	return uint32(uint64(f.fastrand32()) * uint64(n) >> 32)
}

func (f *fastrander) fastrand32() uint32 {
	return uint32(f.fastrand64())
}

// norace to allow modifying state
//
//go:norace
func (f *fastrander) fastrand64() uint64 {
	if inControlledNondeterminism() {
		// TODO: return an actually random value?
		return 0
	}
	f.state += randomStateIncrement
	hi, lo := bits.Mul64(f.state, f.state^0xe7037ed1a0b428db)
	return hi ^ lo
}

func fastrandn(n uint32) uint32 {
	return gs.get().fastrander.fastrandn(n)
}

func fastrand64() uint64 {
	return gs.get().fastrander.fastrand64()
}
