// Copyright (c) 2017-2020 Denis Subbotin, Philip Schlump, Nika Jones, Steven Allen, MoonFruit
// Copyright (c) 2022      Teal.Finance contributors
//
// This file is a modified copy from:
// https://github.com/mr-tron/base58
// The source code has been adapted to support other bases.
//
// SPDX-License-Identifier: MIT

// Package base92 supports cookie tokens.
// The encoded string uses only the 92 accepted characters:
// from 0x20 (space) to 0x7E (~) except " ; and \.
package base92

import "fmt"

// Encode encodes a slice of bytes into a Base92 string
// using the default alphabet.
func Encode(bin []byte) string {
	return EncodeAlphabet(bin, CookieTokenAlphabet)
}

// EncodeAlphabet encodes a slice of bytes into a Base92 string
// using the provided alphabet.
func EncodeAlphabet(bin []byte, alphabet *Alphabet) string {
	size := len(bin)

	zcount := 0
	for zcount < size && bin[zcount] == 0 {
		zcount++
	}

	// It is crucial to make this as short as possible, especially for
	// the usual case of bitcoin addrs
	size = zcount +
		// This is an integer simplification of
		// ceil(log(256)/log(base))
		(size-zcount)*555/406 + 1

	out := make([]byte, size)

	var i, high int
	var carry uint32

	high = size - 1
	for _, b := range bin {
		i = size - 1
		for carry = uint32(b); i > high || carry != 0; i-- {
			carry = carry + 256*uint32(out[i])
			out[i] = byte(carry % uint32(base))
			carry /= uint32(base)
		}
		high = i
	}

	// Determine the additional "zero-gap" in the buffer (aside from zcount)
	for i = zcount; i < size && out[i] == 0; i++ {
	}

	// Now encode the values with actual alphabet in-place
	val := out[i-zcount:]
	size = len(val)
	for i = 0; i < size; i++ {
		out[i] = alphabet.encode[val[i]]
	}

	return string(out[:size])
}

// Decode decodes a Base92 string into a slice of bytes
// using the default alphabet.
func Decode(str string) ([]byte, error) {
	return DecodeAlphabet(str, CookieTokenAlphabet)
}

// DecodeAlphabet decodes the base92 encoded bytes
// using the provided alphabet.
func DecodeAlphabet(str string, alphabet *Alphabet) ([]byte, error) {
	if len(str) == 0 {
		return nil, nil
	}

	zero := alphabet.encode[0]
	b92sz := len(str)

	var zcount int
	for i := 0; i < b92sz && str[i] == zero; i++ {
		zcount++
	}

	var t, c uint64

	// the 32bit algo stretches the result up to 2 times
	binu := make([]byte, 2*((b92sz*406/555)+1))
	outi := make([]uint32, (b92sz+3)/4)

	for _, r := range str {
		if r > 127 {
			return nil, fmt.Errorf("high-bit set on invalid digit")
		}
		if alphabet.decode[r] == -1 {
			return nil, fmt.Errorf("invalid base92 digit (%q)", r)
		}

		c = uint64(alphabet.decode[r])

		for j := len(outi) - 1; j >= 0; j-- {
			t = uint64(outi[j])*uint64(base) + c
			c = t >> 32
			outi[j] = uint32(t & 0xffffffff)
		}
	}

	// initial mask depends on b92sz, on further loops it always starts at 24 bits
	mask := (uint(b92sz%4) * 8)
	if mask == 0 {
		mask = 32
	}
	mask -= 8

	outLen := 0
	for j := 0; j < len(outi); j++ {
		for mask < 32 { // loop relies on uint overflow
			binu[outLen] = byte(outi[j] >> mask)
			mask -= 8
			outLen++
		}
		mask = 24
	}

	// find the most significant byte post-decode, if any
	for msb := zcount; msb < len(binu); msb++ {
		if binu[msb] > 0 {
			return binu[msb-zcount : outLen], nil
		}
	}

	// it's all zeroes
	return binu[:outLen], nil
}
