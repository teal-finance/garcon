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

package token

import (
	"math"
	"strconv"
	"testing"
)

var cases = []struct {
	name    string
	i       int
	v       uint64
	wantErr bool
	t       Token
}{
	{"v=0", 0, 0, false, Token{}},
	{"v=1", 0, 1, false, Token{}},
	{"v=255", 0, 255, false, Token{}},
	{"v=256", 0, 256, false, Token{}},
	{"v=65000", 0, 65000, false, Token{}},
	{"v=66000", 0, 66000, false, Token{}},
	{"v=2²⁴", 0, 1 << 24, false, Token{}},
	{"v=2³³", 0, 1 << 33, false, Token{}},
	{"v=MAX", 0, math.MaxUint64, false, Token{}},

	{"i=1", 1, 9, false, Token{}},
	{"i=2", 2, 9, false, Token{}},
	{"i=9", 9, 9, false, Token{}},
	{"i=63", 63, 9, false, Token{}},
	{"i=64", 64, 9, true, Token{}},

	{"i=1 len=5", 1, 9, false, Token{Values: make([][]byte, 5)}},
	{"i=1 len=5", 1, 9, false, Token{Values: make([][]byte, 5)}},
	{"i=4 len=5", 4, 9, false, Token{Values: make([][]byte, 5)}},
	{"i=5 len=5", 5, 9, false, Token{Values: make([][]byte, 5)}},
	{"i=6 len=5", 6, 9, false, Token{Values: make([][]byte, 5)}},
	{"i=9 len=5", 9, 9, false, Token{Values: make([][]byte, 5)}},
	{"i=63 len=5", 63, 9, false, Token{Values: make([][]byte, 5)}},
	{"i=64 len=5", 64, 9, true, Token{Values: make([][]byte, 5)}},

	{"i=1 cap=5", 1, 9, false, Token{Values: make([][]byte, 0, 5)}},
	{"i=1 cap=5", 1, 9, false, Token{Values: make([][]byte, 0, 5)}},
	{"i=4 len=5", 4, 9, false, Token{Values: make([][]byte, 0, 5)}},
	{"i=5 len=5", 5, 9, false, Token{Values: make([][]byte, 0, 5)}},
	{"i=6 len=5", 6, 9, false, Token{Values: make([][]byte, 0, 5)}},
	{"i=9 cap=5", 9, 9, false, Token{Values: make([][]byte, 0, 5)}},
	{"i=63 cap=5", 63, 9, false, Token{Values: make([][]byte, 0, 5)}},
	{"i=64 cap=5", 64, 9, true, Token{Values: make([][]byte, 0, 5)}},
}

func TestToken_Uint64(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.t.SetUint64(c.i, c.v); (err != nil) != c.wantErr {
				t.Errorf("Token.SetUint64() error = %v, wantErr %v", err, c.wantErr)
			}

			v, err := c.t.Uint64(c.i)
			if (err != nil) != c.wantErr {
				t.Errorf("Token.Uint64() error = %v, wantErr %v", err, c.wantErr)
			}

			if !c.wantErr && (v != c.v) {
				t.Errorf("Mismatch integer got %v, want %v", v, c.v)
			}
		})
	}
}

func TestToken_Bool(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v1 := ((c.v % 2) == 0)

			if err := c.t.SetBool(c.i, v1); (err != nil) != c.wantErr {
				t.Errorf("Token.SetUint64() error = %v, wantErr %v", err, c.wantErr)
			}

			v2, err := c.t.Bool(c.i)
			if (err != nil) != c.wantErr {
				t.Errorf("Token.Uint64() error = %v, wantErr %v", err, c.wantErr)
			}

			if !c.wantErr && (v2 != v1) {
				t.Errorf("Mismatch integer got %v, want %v", v2, v1)
			}
		})
	}
}

func TestToken_String(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v1 := ""
			if c.v > 3 {
				v1 += strconv.FormatUint(c.v, 10) + c.name
			}

			if err := c.t.SetString(c.i, v1); (err != nil) != c.wantErr {
				t.Errorf("Token.SetUint64() error = %v, wantErr %v", err, c.wantErr)
			}

			v2, err := c.t.String(c.i)
			if (err != nil) != c.wantErr {
				t.Errorf("Token.Uint64() error = %v, wantErr %v", err, c.wantErr)
			}

			if !c.wantErr && (v2 != v1) {
				t.Errorf("Mismatch integer got %v, want %v", v2, v1)
			}
		})
	}
}
