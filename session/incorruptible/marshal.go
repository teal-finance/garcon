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

// Package incorruptible serialize a DToken with the incorruptible format.
// An incorruptible is provided by a garcon, a drink served by a waiter.
// The incorruptible uses grapefruit and orange juice with lemonade.
// (see https://www.shakeitdrinkit.com/incorruptible-cocktail-1618.html)
// Here the incorruptible format starts with a magic code (2 bytes),
// followed by the expiry time, the client IP, the user-defined values,
// and ends with ramdom salt as padding for a final size aligned on 32 bits.
package incorruptible

import (
	"fmt"
	"math/rand"

	"github.com/klauspost/compress/s2"

	"github.com/teal-finance/garcon/session/dtoken"
	"github.com/teal-finance/garcon/session/incorruptible/bits"
)

const (
	paddingMaxSize = 8
	enablePadding  = false

	lengthMayCompress  = 100
	lengthMustCompress = 180
)

type Serializer struct {
	ipLength     int
	nValues      int // number of values
	valTotalSize int // sum of the value lengths
	payloadSize  int // size in bytes of the uncompressed payload
	compressed   bool
}

func newSerializer(dt dtoken.DToken) (s Serializer) {
	s.ipLength = len(dt.IP) // can be 0, 4 or 16

	s.nValues = len(dt.Values)

	s.valTotalSize = s.nValues
	for _, v := range dt.Values {
		s.valTotalSize += len(v)
	}

	s.payloadSize = bits.ExpirySize + s.ipLength + s.valTotalSize

	s.compressed = doesCompress(s.payloadSize)

	return s
}

// doesCompress decides to compress or not the payload.
// The compression decision is a bit randomized
// to limit the "chosen plaintext" attack.
func doesCompress(payloadSize int) bool {
	switch {
	case payloadSize < lengthMayCompress:
		return false
	case payloadSize < lengthMustCompress:
		return (0 == rand.Intn(1))
	default:
		return true
	}
}

func Marshal(dt dtoken.DToken, magic uint8) ([]byte, error) {
	s := newSerializer(dt)

	b, err := s.putHeaderExpiryIP(magic, dt)
	if err != nil {
		return nil, err
	}

	b, err = s.appendValues(b, dt)
	if err != nil {
		return nil, err
	}
	if len(b) != bits.HeaderSize+s.payloadSize {
		return nil, fmt.Errorf("unexpected length got=%d want=%d", len(b), bits.HeaderSize+s.payloadSize)
	}

	if s.compressed {
		c := s2.Encode(nil, b[bits.HeaderSize:])
		n := copy(b[bits.HeaderSize:], c)
		if n != len(c) {
			return nil, fmt.Errorf("unexpected copied bytes got=%d want=%d", n, len(c))
		}
		b = b[:bits.HeaderSize+n]
	}

	if enablePadding {
		b = s.appendPadding(b)
	}

	return b, nil
}

func (s Serializer) allocateBuffer() []byte {
	length := bits.HeaderSize + bits.ExpirySize
	capacity := length + s.ipLength + s.valTotalSize

	if enablePadding {
		capacity += paddingMaxSize
	}

	return make([]byte, length, capacity)
}

func (s Serializer) putHeaderExpiryIP(magic uint8, dt dtoken.DToken) ([]byte, error) {
	b := s.allocateBuffer()

	m, err := bits.NewMetadata(s.ipLength, s.compressed, s.nValues)
	if err != nil {
		return nil, err
	}

	m.PutHeader(b, magic)

	err = bits.PutExpiry(b, dt.Expiry)
	if err != nil {
		return nil, err
	}

	b = bits.AppendIP(b, dt.IP)

	return b, nil
}

func (s Serializer) appendValues(b []byte, dt dtoken.DToken) ([]byte, error) {
	for _, v := range dt.Values {
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
	missing += 4 * rand.Intn(paddingMaxSize/4-1)

	for i := 1; i < missing; i++ {
		b = append(b, uint8(rand.Intn(256)))
	}

	// last byte is the padding length
	b = append(b, uint8(missing))

	return b
}
