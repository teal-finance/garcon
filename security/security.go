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

import (
	"crypto/rand"
	"encoding/base64"
	"hash"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/minio/highwayhash"
	"github.com/teal-finance/garcon/reserr"
)

const hashKey32bytes = "0123456789ABCDEF0123456789ABCDEF"

// Code points in the surrogate range are not valid for UTF-8.
const (
	surrogateMin = 0xD800
	surrogateMax = 0xDFFF
)

// Sanitize replaces control codes by the tofu symbol
// and invalid UTF-8 codes by the replacement character.
// Sanitize can be used to prevent log injection.
// Inspired from:
// https://wikiless.org/wiki/Replacement_character#Replacement_character
// https://graphicdesign.stackexchange.com/q/108297
func Sanitize(s string) string {
	return strings.Map(
		func(r rune) rune {
			switch {
			case r < 32:
			case r == 127: // The .notdef character is often represented by the empty box (tofu)
				return '􏿮' // to indicate a valid but not rendered character.
			case surrogateMin <= r && r <= surrogateMax:
			case utf8.MaxRune < r:
				return '�' // The replacement character U+FFFD indicates an invalid UTF-8 character.
			}

			return r
		},
		s,
	)
}

// PrintableRune returns false if rune is
// a Carriage Return "\r", or a Line Feed "\n",
// or another ASCII control code (except space),
// or an invalid UTF-8 code.
// PrintableRune can be used to prevent log injection.
func PrintableRune(r rune) bool {
	switch {
	case r < 32:
		return false
	case r == 127:
		return false
	case surrogateMin <= r && r <= surrogateMax:
		return false
	case utf8.MaxRune > r:
		return false
	}

	return true
}

// Printable returns false if input string contains
// a Carriage Return "\r", or a Line Feed "\n",
// or any other ASCII control code (except space),
// or, as well as, invalid UTF-8 codes.
// Printable can be used to prevent log injection.
func Printable(s string) bool {
	for _, r := range s {
		if !PrintableRune(r) {
			return false
		}
	}

	return true
}

// Printables returns true when all the strings are printable.
func Printables(array []string) bool {
	for _, s := range array {
		if !Printable(s) {
			return false
		}
	}

	return true
}

// RejectInvalidURI rejects HTTP requests having
// a Carriage Return "\r" or a Line Feed "\n"
// within the URI to prevent log injection.
func RejectInvalidURI(next http.Handler) http.Handler {
	log.Print("Middleware security: RejectLineBreakInURI")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if !Printable(r.RequestURI) {
				reserr.Write(w, r, http.StatusBadRequest, "Invalid URI containing a line break (CR or LF)")
				log.Print("WRN WebServer: reject URI with <CR> or <LF>:", Sanitize(r.RequestURI))

				return
			}

			next.ServeHTTP(w, r)
		})
}

// TraversalPath returns true when path contains ".." to prevent path traversal attack.
func TraversalPath(w http.ResponseWriter, r *http.Request) bool {
	if strings.Contains(r.URL.Path, "..") {
		reserr.Write(w, r, http.StatusBadRequest, "Invalid URL Path Containing '..'")
		log.Print("WRN WebServer: reject path with '..' ", Sanitize(r.URL.Path))

		return true
	}

	return false
}

type Hash struct {
	h hash.Hash
}

func NewHash() (Hash, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return Hash{nil}, err
	}

	h, err := highwayhash.New(key)

	return Hash{h}, err
}

// Obfuscate hashes the input string to prevent logging sensitive information.
// HighwayHash is a hashing algorithm enabling high speed (especially on AMD64).
func (h Hash) Obfuscate(s string) (string, error) {
	h.h.Reset()
	checksum := h.h.Sum([]byte(s))

	return base64.StdEncoding.EncodeToString(checksum), nil
}

// Obfuscate hashes the input string to prevent logging sensitive information.
// HighwayHash is a hashing algorithm enabling high speed (especially on AMD64).
func Obfuscate(s string) (string, error) {
	hash, err := highwayhash.New([]byte(hashKey32bytes))
	if err != nil {
		return s, err
	}

	checksum := hash.Sum([]byte(s))

	return base64.StdEncoding.EncodeToString(checksum), nil
}
