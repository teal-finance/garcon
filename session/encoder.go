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
	"log"
	"time"

	"github.com/teal-finance/garcon/session/dtoken"
	"github.com/teal-finance/garcon/session/incorruptible"
	"github.com/teal-finance/garcon/session/incorruptible/bits"
)

const (
	base92MinSize     = 26
	ciphertextMinSize = 22

	// no space, double-quote ", semi-colon ; and back-slash \
	noSpaceDoubleQuoteSemicolon = "" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789!#$%&()*+,-./:<=>?@[]^_`{|}~'"

	doPrint = false
)

func (s *Session) Encode(dt dtoken.DToken) (string, error) {
	printDT("Encode", dt, errors.New(""))

	plaintext, err := incorruptible.Marshal(dt, s.magic)
	if err != nil {
		return "", err
	}
	printBin("Encode plaintext", plaintext)

	ciphertext := s.cipher.Encrypt(plaintext)
	printBin("Encode ciphertext", ciphertext)

	str := s.baseXX.EncodeToString(ciphertext)
	printStr("Encode BaseXX", str)
	return str, nil
}

func (s *Session) Decode(str string) (dtoken.DToken, error) {
	printStr("Decode BaseXX", str)

	var dt dtoken.DToken
	if len(str) < base92MinSize {
		return dt, fmt.Errorf("BaseXX string too short: %d < min=%d", len(str), base92MinSize)
	}

	ciphertext, err := s.baseXX.DecodeString(str)
	if err != nil {
		return dt, err
	}
	printBin("Decode ciphertext", ciphertext)

	if len(ciphertext) < ciphertextMinSize {
		return dt, fmt.Errorf("ciphertext too short: %d < min=%d", len(ciphertext), ciphertextMinSize)
	}

	plaintext, err := s.cipher.Decrypt(ciphertext)
	if err != nil {
		return dt, err
	}
	printBin("Decode plaintext", plaintext)

	magic := bits.MagicCode(plaintext)
	if magic != s.magic {
		return dt, errors.New("bad magic code")
	}

	dt, err = incorruptible.Unmarshal(plaintext)
	printDT("Decode", dt, err)
	return dt, err
}

func printStr(name, s string) {
	if doPrint {
		n := len(s)
		if n > 30 {
			n = 30
		}
		log.Printf("Session%s len=%d %q", name, len(s), s[:n])
	}
}

func printBin(name string, b []byte) {
	if doPrint {
		n := len(b)
		if n > 30 {
			n = 30
		}
		log.Printf("Session%s len=%d cap=%d %x", name, len(b), cap(b), b[:n])
	}
}

func printDT(name string, dt dtoken.DToken, err error) {
	if doPrint {
		log.Printf("Session%s dt %v %v n=%d err=%s", name,
			time.Unix(dt.Expiry, 0), dt.IP, len(dt.Values), err)
	}
}
