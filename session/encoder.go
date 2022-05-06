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

	"github.com/teal-finance/garcon/base92"
	"github.com/teal-finance/garcon/session/dtoken"
	"github.com/teal-finance/garcon/session/incorruptible"
	"github.com/teal-finance/garcon/session/incorruptible/bits"
)

const (
	base92MinSize     = 20
	ciphertextMinSize = 16
)

func (s *Session) Encode(dt dtoken.DToken) (string, error) {
	plaintext, err := incorruptible.Marshal(dt, s.magic)
	if err != nil {
		return "", err
	}

	ciphertext, err := s.cipher.Encrypt(plaintext)
	if err != nil {
		return "", err
	}

	str := base92.Encode(ciphertext)
	return str, nil
}

func (s *Session) Decode(str string) (dt dtoken.DToken, err error) {
	if len(str) < base92MinSize {
		return dt, fmt.Errorf("Base92 string too short: %d < min=%d", len(str), base92MinSize)
	}

	ciphertext, err := base92.Decode(str)
	if err != nil {
		return dt, err
	}

	if len(ciphertext) < ciphertextMinSize {
		return dt, fmt.Errorf("ciphertext too short: %d < min=%d", len(ciphertext), ciphertextMinSize)
	}

	plaintext, err := s.cipher.Decrypt(ciphertext)
	if err != nil {
		return dt, err
	}

	magic := bits.MagicCode(plaintext)
	if magic != s.magic {
		return dt, errors.New("bad magic code")
	}

	return incorruptible.Unmarshal(plaintext)
}
