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
	"errors"
	"fmt"

	"github.com/teal-finance/garcon/a85"
	"github.com/teal-finance/garcon/session/incorruptible"
	"github.com/teal-finance/garcon/session/token"
)

const (
	a85MinSize        = 16
	ciphertextMinSize = 12
)

func (ck *Checker) Encode(t token.Token) ([]byte, error) {
	plaintext, err := incorruptible.Marshal(t, ck.magic)
	if err != nil {
		return nil, err
	}

	ciphertext, err := ck.cipher.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}

	a := a85.Encode(ciphertext)
	return a, nil
}

func (ck *Checker) Decode(a string) (t token.Token, err error) {
	if len(a) < a85MinSize {
		return t, fmt.Errorf("Ascii85 string too short: %d < min=%d", len(a), a85MinSize)
	}

	ciphertext, err := a85.Decode(a)
	if err != nil {
		return t, err
	}

	if len(ciphertext) < ciphertextMinSize {
		return t, fmt.Errorf("ciphertext too short: %d < min=%d", len(ciphertext), ciphertextMinSize)
	}

	plaintext, err := ck.cipher.Decrypt(ciphertext)
	if err != nil {
		return t, err
	}

	magic := incorruptible.MagicCode(plaintext)
	if magic != ck.magic {
		return t, errors.New("bad magic code")
	}

	return incorruptible.Unmarshal(plaintext)
}
