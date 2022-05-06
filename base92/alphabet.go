// Copyright (c) 2017-2020 Denis Subbotin, Philip Schlump, Nika Jones, Steven Allen, MoonFruit
// Copyright (c) 2022      Teal.Finance contributors
//
// This file is a modified copy from:
// https://github.com/mr-tron/base58
// The source code has been adapted to support other bases.
//
// SPDX-License-Identifier: MIT

package base92

import "log"

const alphabet = " !" + // double-quote " removed
	"#$%&'()*+,-./0123456789:" + // semi-colon ; removed
	"<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[" + // back-slash \ removed
	"]^_`abcdefghijklmnopqrstuvwxyz{|}~"

// base is 92.
const base = len(alphabet)

// CookieTokenAlphabet is the bitcoin base92 alphabet.
var CookieTokenAlphabet = NewAlphabet(alphabet)

// Alphabet is an optimized form of the encoding characters.
type Alphabet struct {
	decode [128]int8
	encode [base]byte
}

// NewAlphabet creates a new alphabet.
//
// It panics if the passed string is not 92 bytes long, isn't valid ASCII,
// or does not contain 92 distinct characters.
func NewAlphabet(s string) *Alphabet {
	if len(s) != base {
		log.Panicf("alphabets must be %d bytes long", base)
	}

	ret := new(Alphabet)
	copy(ret.encode[:], s)
	for i := range ret.decode {
		ret.decode[i] = -1
	}

	distinct := 0
	for i, b := range ret.encode {
		if ret.decode[b] == -1 {
			distinct++
		}
		ret.decode[b] = int8(i)
	}

	if distinct != base {
		log.Panicf("provided alphabet does not consist of %d distinct characters", base)
	}

	return ret
}
