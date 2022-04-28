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
