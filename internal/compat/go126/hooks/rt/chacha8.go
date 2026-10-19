//go:build go1.26 && !go1.27

// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt

// InternalChacha8rand_block follows the selected Go source's generic ChaCha8
// algorithm. Four blocks are interlaced; only key words receive feed-forward.
// Explicit packing avoids native assembly and unsafe host endian assumptions.
func InternalChacha8rand_block(seed *[4]uint64, buf *[32]uint64, counter uint32) {
	var words [16][4]uint32
	for lane := 0; lane < 4; lane++ {
		x := [16]uint32{0x61707865, 0x3320646e, 0x79622d32, 0x6b206574}
		for i := 0; i < 4; i++ {
			x[4+2*i] = uint32(seed[i])
			x[5+2*i] = uint32(seed[i] >> 32)
		}
		x[12] = counter + uint32(lane)
		original := x
		for round := 0; round < 4; round++ {
			chachaQuarter(&x, 0, 4, 8, 12)
			chachaQuarter(&x, 1, 5, 9, 13)
			chachaQuarter(&x, 2, 6, 10, 14)
			chachaQuarter(&x, 3, 7, 11, 15)
			chachaQuarter(&x, 0, 5, 10, 15)
			chachaQuarter(&x, 1, 6, 11, 12)
			chachaQuarter(&x, 2, 7, 8, 13)
			chachaQuarter(&x, 3, 4, 9, 14)
		}
		for i := 4; i < 12; i++ {
			x[i] += original[i]
		}
		for i := range x {
			words[i][lane] = x[i]
		}
	}
	for i := range words {
		buf[i*2] = uint64(words[i][0]) | uint64(words[i][1])<<32
		buf[i*2+1] = uint64(words[i][2]) | uint64(words[i][3])<<32
	}
}

func chachaQuarter(x *[16]uint32, a, b, c, d int) {
	x[a] += x[b]
	x[d] ^= x[a]
	x[d] = x[d]<<16 | x[d]>>16
	x[c] += x[d]
	x[b] ^= x[c]
	x[b] = x[b]<<12 | x[b]>>20
	x[a] += x[b]
	x[d] ^= x[a]
	x[d] = x[d]<<8 | x[d]>>24
	x[c] += x[d]
	x[b] ^= x[c]
	x[b] = x[b]<<7 | x[b]>>25
}
