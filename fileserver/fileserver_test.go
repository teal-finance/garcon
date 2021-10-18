// Teal.Finance/Server is an opinionated complete HTTP server.
// Copyright (C) 2021 Teal.Finance contributors
//
// This file is part of Teal.Finance/Server, licensed under LGPL-3.0-or-later.
//
// Teal.Finance/Server is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 of the License, or (at your option) any later version.
//
// Teal.Finance/Server is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.

package fileserver

import (
	"testing"
)

func Test_extIndex(t *testing.T) {
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
		t.Run(c.name, func(t *testing.T) {
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
