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
	"net"

	"github.com/klauspost/compress/s2"

	"github.com/teal-finance/garcon/session/incorruptible/bits"
	"github.com/teal-finance/garcon/session/token"
)

func Unmarshal(b []byte) (t token.Token, err error) {
	if len(b) < bits.HeaderSize+bits.ExpirySize+net.IPv4len {
		return t, fmt.Errorf("not enough bytes (%d) for header+expiry+IP", len(b))
	}

	m := bits.GetMetadata(b)
	b = b[bits.HeaderSize:] // drop header

	if enablePadding {
		b, err = dropPadding(b)
		if err != nil {
			return t, err
		}
	}

	if m.IsCompressed() {
		b, err = s2.Decode(nil, b)
		if err != nil {
			return t, fmt.Errorf("s2.Decode %w", err)
		}
	}

	if len(b) < m.PayloadMinSize() {
		return t, fmt.Errorf("not enough bytes for payload %d < %d", len(b), m.PayloadMinSize())
	}

	b, t.Expiry = bits.DecodeExpiry(b)
	b, t.IP = m.DecodeIP(b)

	t.Values, err = parseValues(b, m.NValues())
	if err != nil {
		return t, err
	}

	return t, nil
}

func parseValues(b []byte, nV int) ([][]byte, error) {
	values := make([][]byte, 0, nV)

	for i := 0; i < nV; i++ {
		if len(b) < (nV - i) {
			return nil, fmt.Errorf("not enough bytes (%d) at length #%d", len(b), i)
		}

		n := b[0] // number of bytes representing the value
		b = b[1:] // drop the byte containing the length of the value

		if len(b) < int(n) {
			return nil, fmt.Errorf("not enough bytes (%d) at value #%d", len(b), i)
		}

		v := b[:n] // extract the value in raw form
		b = b[n:]  // drop the bytes containing the value

		values = append(values, v)
	}

	if len(b) > 0 {
		return nil, fmt.Errorf("unexpected remaining %d bytes", len(b))
	}

	return values, nil
}

func dropPadding(b []byte) ([]byte, error) {
	paddingSize := int(b[len(b)-1]) // last byte is the padding length
	if paddingSize > paddingMaxSize {
		return nil, fmt.Errorf("too much padding bytes (%d)", paddingSize)
	}

	b = b[:len(b)-paddingSize] // drop padding
	return b, nil
}
