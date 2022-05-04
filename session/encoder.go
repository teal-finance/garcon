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

package session

import (
	"encoding/ascii85"
	"errors"
	"fmt"

	"github.com/teal-finance/garcon/session/incorruptible"
	"github.com/teal-finance/garcon/session/token"
)

func (ck *Checker) Encode(t token.Token) ([]byte, error) {
	plaintext, err := Marshal(t, ck.magic)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, len(plaintext))
	ck.cipher.Encrypt(ciphertext, plaintext)

	a85 := encodeAscii85(ciphertext)
	return a85, nil
}

func (ck *Checker) Decode(a85 string) (t token.Token, err error) {
	ciphertext, err := decodeAscii85(a85)
	if err != nil {
		return t, err
	}

	if len(ciphertext) < ciphertextMinLen {
		return t, fmt.Errorf("ciphertext too short: %d < min=%d", len(ciphertext), ciphertextMinLen)
	}

	plaintext := make([]byte, len(ciphertext))
	ck.cipher.Decrypt(plaintext, ciphertext)

	magic := incorruptible.MagicCode(plaintext)
	if magic != ck.magic {
		return t, errors.New("bad magic code")
	}

	return incorruptible.Unmarshal(plaintext)
}

func encodeAscii85(bin []byte) []byte {
	max := ascii85.MaxEncodedLen(len(bin))
	a85 := make([]byte, max)
	n := ascii85.Encode(a85, bin)
	return a85[:n]
}

func decodeAscii85(a85 string) ([]byte, error) {
	// Ascii85 encodes 4 bytes 0x0000 by only one byte "z"
	max := 4 * len(a85)
	bin := make([]byte, max)

	n, _, err := ascii85.Decode(bin, []byte(a85), true)
	if err != nil {
		return nil, fmt.Errorf("ascii85.Decode %w", err)
	}

	return bin[:n], nil
}
