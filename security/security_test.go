// #region <editor-fold desc="Preamble">
// Copyright (c) 2022 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon,
// an opinionated boilerplate API and website server,
// licensed under LGPL-3.0-or-later.
// SPDX-License-Identifier: LGPL-3.0-or-later
//
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 or any later version, at the licensee’s option.
//
// Teal.Finance/Garcon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
//
// For more details, see the LICENSE file (alongside the source files)
// or the GNU General Public License: <https://www.gnu.org/licenses/>
// #endregion </editor-fold>

package security

import "testing"

func TestPrintableRune(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		want bool
	}{
		{"valid", 't', true},
		{"invalid", '\t', false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PrintableRune(c.r); got != c.want {
				t.Errorf("PrintableRune(%v) = %v, want %v", c.r, got, c.want)
			}
		})
	}
}
