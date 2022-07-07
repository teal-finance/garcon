// Copyright (c) 2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

package garcon_test

import (
	"testing"

	"github.com/teal-finance/garcon"
)

func TestPrintableRune(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		r    rune
		want bool
	}{
		{"valid", 't', true},
		{"invalid", '\t', false},
	}

	for _, c := range cases {
		c := c // parallel test

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := garcon.PrintableRune(c.r); got != c.want {
				t.Errorf("PrintableRune(%v) = %v, want %v", c.r, got, c.want)
			}
		})
	}
}
