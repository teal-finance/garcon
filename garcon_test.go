// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon_test

import (
	"reflect"
	"testing"

	"github.com/teal-finance/garcon"
)

func clone[T any](slice []T) []T {
	return append([]T{}, slice...)
}

func TestExtractWords(t *testing.T) {
	t.Parallel()

	dico := []string{"AAA", "Bbb", "ccc", "DDD"}

	cases := []struct {
		name       string
		csv        string
		dictionary []string
		want       []string
	}{
		{"empty", "", clone(dico), []string{}},
		{"all", "all", clone(dico), clone(dico)},
		{"duplicates", "a,b,c,d,a,b,c,d,a,b,c,d", clone(dico), clone(dico)},
		{"spaces", "  , , ,,, ,,,, , , ", clone(dico), []string{}},
		{"spaces-b", "  , , ,,, ,,,,b, , ", clone(dico), []string{"Bbb"}},
		{"spaces-bb", "  , , b,, ,,,,b, , ", clone(dico), []string{"Bbb"}},
		{"spaces--b", "  , , b ,,, ,,,,, , ", clone(dico), []string{"Bbb"}},
		{"spaces-all", "  , , ,,, ,,,,all, , ", clone(dico), clone(dico)},
		{"spaces-d-all", "  d , ,,, ,,,,all, , ", clone(dico), clone(dico)},
		{"not-found", "aaaa,g,h,i'j,kkkkkkkkkkkk", clone(dico), []string{}},
	}
	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := garcon.ExtractWords(c.csv, c.dictionary); !reflect.DeepEqual(got, c.want) {
				t.Errorf("ExtractWords() = %v, want %v", got, c.want)
			}
		})
	}
}
