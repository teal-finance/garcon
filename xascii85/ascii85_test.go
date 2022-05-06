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

package xascii85

import (
	"reflect"
	"testing"
)

var cases = []struct {
	name string
	bin  []byte
}{
	{"nil", nil},
	{"empty", []byte{}},
	{"zero", []byte{0}},
	{"one", []byte{1}},
	{"two", []byte{2}},
	{"ten", []byte{10}},
	{"2zeros", []byte{0, 0}},
	{"2ones", []byte{1, 1}},
	{"64zeros", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	{"65zeros", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	{"ascii", []byte("c'est une longue chason")},
	{"utf8", []byte("Garçon, un café très fort !")},
}

func TestEncode(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			str := Encode(c.bin)

			ni := len(c.bin)
			if ni > 70 {
				ni = 70 // print max the first 70 bytes
			}
			na := len(str)
			if na > 70 {
				na = 70 // print max the first 70 characters
			}
			t.Logf("bin len=%d [:%d]=%v", len(c.bin), ni, c.bin[:ni])
			t.Logf("str len=%d [:%d]=%q", len(str), na, str[:na])

			got, err := Decode(str)
			if err != nil {
				t.Errorf("Decode() error = %v", err)
				return
			}

			ng := len(got)
			if ng > 70 {
				ng = 70 // print max the first 70 bytes
			}
			t.Logf("got len=%d [:%d]=%v", len(got), ng, got[:ng])

			if (len(got) == 0) && (len(c.bin) == 0) {
				return
			}

			if !reflect.DeepEqual(got, c.bin) {
				t.Errorf("Decode() = %v, want %v", got, c.bin)
			}
		})
	}
}
