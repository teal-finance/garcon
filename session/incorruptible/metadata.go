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

package incorruptible

import (
	"fmt"
	"math/rand"
	"net"
)

const (
	// Metadata bits in byte #2.
	maskIPv4     = 0b_1000_0000
	maskCompress = 0b_0100_0000
	maskNValues  = 0b_0011_1111
)

func MagicCode(b []byte) uint8 {
	return b[0]
}

// putHeader fills the magic code, some randome salt and the metadata.
func (s Serializer) putHeader(b []byte, magic uint8) error {
	if s.nValues > maskNValues {
		return fmt.Errorf("too much values %d > %d", s.nValues, maskNValues)
	}

	b[0] = magic
	b[1] = byte(rand.Intn(256))
	b[2] = s.newMetadata()

	return nil
}

type metadata byte

// putHeader fills the magic code, some randome salt and the metadata.
func (s Serializer) newMetadata() byte {
	var m byte

	if s.ipLength == 4 {
		m |= maskIPv4
	}

	if s.compressed {
		m |= maskCompress
	}

	m |= uint8(s.nValues) // nValues must be less than maskNValues

	return m
}

func extractMetadata(b []byte) metadata {
	return metadata(b[2])
}

func (m metadata) ipLength() int {
	if (m & maskIPv4) == 0 {
		return net.IPv6len
	}
	return net.IPv4len
}

func (m metadata) isCompressed() bool {
	c := m & maskCompress
	return c != 0
}

func (m metadata) nValues() int {
	n := m & maskNValues
	return int(n)
}

func expiry(b []byte) uint32 {
	expiry := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
	return expiry
}

func putExpiryTime(b []byte, expiry uint32) {
	b[3] = byte(expiry)
	b[4] = byte(expiry >> 8)
	b[5] = byte(expiry >> 16)
}

func ip(b []byte, ipLen int) net.IP {
	i := expirySize
	j := expirySize + ipLen
	return b[i:j]
}

func appendIP(b []byte, ip net.IP) []byte {
	return append(b, ip...)
}
