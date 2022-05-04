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

package a85

import (
	"encoding/ascii85"
	"fmt"
)

func Encode(bin []byte) []byte {
	max := ascii85.MaxEncodedLen(len(bin))
	a85 := make([]byte, max)
	n := ascii85.Encode(a85, bin)
	return a85[:n]
}

func Decode(a85 string) ([]byte, error) {
	max := maxDecodedLen(len(a85))
	bin := make([]byte, max)

	n, _, err := ascii85.Decode(bin, []byte(a85), true)
	if err != nil {
		return nil, fmt.Errorf("ascii85.Decode %w", err)
	}

	return bin[:n], nil
}

// MaxDecodedSize returns the maximum length of a decoding of n binary bytes.
// Ascii85 encodes 4 bytes 0x0000 by only one byte "z".
func maxDecodedLen(n int) int { return 4 * n }
