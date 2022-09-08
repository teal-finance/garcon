// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon_test

import (
	"reflect"
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

func TestSplitCleanedLines(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		arg  string
		want []string
	}{
		{"empty", "", nil},
		{"space", " ", nil},
		{"space+control", " \t  \v  \a  ", nil},
		{"space+control+8", " \t  8\v  \a  ", []string{"8"}},
		{"space+control+88", " \t  88\v  \a  ", []string{"88"}},
		{"space+control+88+8", " \t  88\v  \a   \t  8  \v  \a  ", []string{"88 8"}},
		{"LF", "\n", nil},
		{"2LF", "\n\n", nil},
		{"LFCR", "\n\r", nil},
		{"LFCRLFCR", "\n\r\n\n\r\r\n\r", nil},
		{"space+LFCR", " \n\r", nil},
		{"LFCR+space", "\n\r ", nil},
		{"LFCR+space+LRCR", "\n\r \n\r", nil},
		{"space+LFCR+space+LRCR+space", "   \n \r \n \r   ", nil},
		{"complex", "aa\r\nbb", []string{"aa", "bb"}},
		{"complex", "aa\r\nbb\r\n", []string{"aa", "bb"}},
		{"complex", "\b   \n \r aa\n\t\tbb\t\t\t\tbb\t\t \r\n\n\n\n\n  c c   c     c      c    \n\n\n   ", []string{"aa", "bb bb", "", "c c c c c"}},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got := garcon.SplitCleanedLines(c.arg)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("SplitCleanedLines() = %q, want %q", got, c.want)
			}
		})
	}
}
