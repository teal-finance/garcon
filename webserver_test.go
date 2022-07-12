// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

//nolint:testpackage // test unexported function
package garcon

import (
	"testing"
)

func Test_extIndex(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		ext  string
	}{
		{"regular folder and filename", "folder/file.ext", "ext"},
		{"without folder", "file.ext", "ext"},
		{"filename without extension", "folder/file", ""},
		{"empty path has no extension", "", ""},
		{"valid folder but empty filename", "folder/", ""},
		{"ignore dot in folder", "folder.ext/file", ""},
		{"ignore dot in folder even when no file", "folder.ext/", ""},
		{"filename ending with a dot has no extension", "ending-dot.", ""},
		{"filename ending with a double dot has no extension", "double-dot..", ""},
		{"only consider the last dot", "a..b.c..ext", "ext"},
		{"filename starting with a dot has an extension", ".gitignore", "gitignore"},
	}

	for _, c := range cases {
		c := c // parallel test

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			extPos := extIndex(c.path)
			max := len(c.path)
			if extPos < 0 || extPos > max {
				t.Errorf("extIndex() = %v out of range [0..%v]", extPos, max)
			}

			got := c.path[extPos:]
			if got != c.ext {
				t.Errorf("extIndex() = %v, want %v", got, c.ext)
			}
		})
	}
}
