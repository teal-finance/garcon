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
	"reflect"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"64zeros", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"65zeros", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"ascii", []byte("c'est une longue chason")},
		{"utf8", []byte("Garçon, un café très fort !")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Encode(c.input)

			t.Log("len(a)", len(a))

			ni := len(c.input)
			if ni > 70 {
				ni = 70 // print max the first 70 bytes
			}
			na := len(a)
			if na > 70 {
				na = 70 // print max the first 70 characters
			}
			t.Logf("i[:%d] %v", na, c.input[:ni])
			t.Logf("a[:%d] %v", na, a[:na])

			got, err := Decode(string(a))
			if err != nil {
				t.Errorf("Decode() error = %v", err)
				return
			}

			if (len(got) == 0) && (len(c.input) == 0) {
				return
			}

			if !reflect.DeepEqual(got, c.input) {
				t.Errorf("Unmarshal() = %v, want %v", got, c.input)
			}
		})
	}
}
