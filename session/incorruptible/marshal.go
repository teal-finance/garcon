// #region <editor-fold desc="Preamble">
// Copyright (c) 2022 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon, an API and website server.
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 or any later version, at the licensee’s option.
// SPDX-License-Identifier: LGPL-3.0-or-later
//
// Teal.Finance/Garcon is distributed WITHOUT ANY WARRANTY.
// For more details, see the LICENSE file (alongside the source files)
// or online at <https://www.gnu.org/licenses/lgpl-3.0.html>
// #endregion </editor-fold>

// Package incorruptible serialize a Token to the incorruptible format.
// An incorruptible is provided by a garcon, a drink served by a waiter.
// The incorruptible uses grapefruit and orange juice with lemonade.
// (see https://www.shakeitdrinkit.com/incorruptible-cocktail-1618.html)
// Here the incorruptible format starts with a magic code (2 bytes),
// followed by the expiry time, the client IP, the user-defined values,
// and ends with ramdom salt as padding for a final size aligned on 32 bits.
package incorruptible

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"unsafe"

	"github.com/klauspost/compress/s2"

	"github.com/teal-finance/garcon/session/token"
)

const (
	magicCodeLen  = 1
	saltLen       = 1
	metadataLen   = 1
	headerLen     = magicCodeLen + saltLen + metadataLen
	expiryLen     = int(unsafe.Sizeof(int64(0))) // int64 = 8 bytes
	paddingMaxLen = 8

	lengthMayCompress  = 100
	lengthMustCompress = 180
)

type Serializer struct {
	ipLength   int
	nValues    int // number of values
	valLenSum  int // sum of the value lengths
	payloadLen int // size in bytes of the uncompressed payload
	compressed bool
}

func newSerializer(t token.Token) (s Serializer) {
	s.ipLength = len(t.IP)

	s.nValues = len(t.Values)

	s.valLenSum = s.nValues
	for _, v := range t.Values {
		s.valLenSum += len(v)
	}

	s.payloadLen = expiryLen + s.ipLength + s.valLenSum

	s.compressed = doesCompress(s.payloadLen)

	return s
}

// doesCompress decides to compress or not the payload.
// The compression decision is a bit randomized
// to limit the "chosen plaintext" attack.
func doesCompress(payloadLen int) bool {
	switch {
	case payloadLen < lengthMayCompress:
		return false
	case payloadLen < lengthMustCompress:
		return (0 == rand.Intn(1))
	default:
		return true
	}
}

func Marshal(t token.Token, magic uint8) ([]byte, error) {
	s := newSerializer(t)
	b := s.buffer()

	if err := s.putHeader(b, magic); err != nil {
		return nil, err
	}

	putExpiryTime(b, t.Expiry)
	b = appendIP(b, t.IP)

	var err error
	if b, err = s.appendValues(b, t); err != nil {
		return nil, err
	}

	if len(b) != headerLen+s.payloadLen {
		return nil, fmt.Errorf("unexpected length got=%d want=%d", len(b), headerLen+s.payloadLen)
	}

	if s.compressed {
		c := s2.Encode(nil, b[headerLen:])
		n := copy(b[headerLen:], c)
		if n != len(c) {
			return nil, fmt.Errorf("unexpected copied bytes got=%d want=%d", n, len(c))
		}
		b = b[:headerLen+n]
	}

	b = s.appendPadding(b)
	return b, nil
}

func (s Serializer) buffer() []byte {
	length := headerLen + expiryLen
	capacity := length + s.ipLength + s.valLenSum + paddingMaxLen
	return make([]byte, length, capacity)
}

func putExpiryTime(b []byte, expiry uint64) {
	binary.BigEndian.PutUint64(b[headerLen:], expiry)
}

func appendIP(b []byte, ip net.IP) []byte {
	return append(b, ip...)
}

func (s Serializer) appendValues(b []byte, t token.Token) ([]byte, error) {
	for _, v := range t.Values {
		if len(v) > 255 {
			return nil, fmt.Errorf("too large %d > 255", v)
		}
		b = append(b, uint8(len(v)))
		b = append(b, v...)
	}
	return b, nil
}

// appendPadding adds random padding bytes.
// Ascii85 encoding is based on 4-byte block (32 bits).
// This function optimizes the Ascii85 encoding.
func (s *Serializer) appendPadding(b []byte) []byte {
	trailing := len(b) % 4
	missing := 4 - trailing
	missing += 4 * rand.Intn(paddingMaxLen/4-1)

	for i := 1; i < missing; i++ {
		b = append(b, uint8(rand.Intn(256)))
	}

	// last byte is the padding length
	b = append(b, uint8(missing))

	return b
}
