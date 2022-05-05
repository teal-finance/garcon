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

package bits

import (
	"fmt"
	"math/rand"
	"net"
)

const (
	HeaderSize    = magicCodeSize + saltSize + metadataSize
	magicCodeSize = 1
	saltSize      = 1
	metadataSize  = 1

	// Metadata coding in byte #2.
	maskIPv4     = 0b_1000_0000
	maskCompress = 0b_0100_0000
	maskNValues  = 0b_0011_1111

	MaxValues int = maskNValues
)

func MagicCode(b []byte) uint8 {
	return b[0]
}

type Metadata byte

func GetMetadata(b []byte) Metadata {
	return Metadata(b[2])
}

func NewMetadata(ipLength int, compressed bool, nValues int) (Metadata, error) {
	var m byte

	if ipLength == 4 {
		m |= maskIPv4
	}

	if compressed {
		m |= maskCompress
	}

	if nValues < 0 {
		return 0, fmt.Errorf("negative nValues %d", nValues)
	}
	if nValues > MaxValues {
		return 0, fmt.Errorf("too much values %d > %d", nValues, MaxValues)
	}

	m |= uint8(nValues)

	return Metadata(m), nil
}

func (m Metadata) PayloadMinSize() int {
	return ExpirySize + m.ipLength() + m.NValues()
}

// putHeader fills the magic code, the salt and the metadata.
func (m Metadata) PutHeader(b []byte, magic uint8) {
	b[0] = magic
	b[1] = byte(rand.Intn(256)) // random salt
	b[2] = byte(m)
}

func (m Metadata) ipLength() int {
	if (m & maskIPv4) == 0 {
		return net.IPv6len
	}
	return net.IPv4len
}

func (m Metadata) IsCompressed() bool {
	c := m & maskCompress
	return c != 0
}

func (m Metadata) NValues() int {
	n := m & maskNValues
	return int(n)
}

func PutExpiry(b []byte, unix int64) error {
	internal, err := unixToInternalExpiry(unix)
	if err != nil {
		return err
	}

	putInternalExpiry(b, internal)

	return nil
}

func DecodeExpiry(b []byte) ([]byte, int64) {
	internal := internalExpiry(b)
	unix := internalExpiryToUnix(internal)
	return b[ExpirySize:], unix
}

func putInternalExpiry(b []byte, e uint32) {
	// Expiry is store just after the header
	b[HeaderSize+0] = byte(e)
	b[HeaderSize+1] = byte(e >> 8)
}

func internalExpiry(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8
}

func AppendIP(b []byte, ip net.IP) []byte {
	return append(b, ip...)
}

func (m Metadata) DecodeIP(b []byte) ([]byte, net.IP) {
	n := m.ipLength()
	ip := b[:n]
	return b[n:], ip
}
