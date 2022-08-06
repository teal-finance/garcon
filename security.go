// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"crypto/rand"
	"encoding/base64"
	"hash"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	// HighwayHash mixes inputs with AVX2 multiply and permute instructions
	// to be 5x faster than SipHash, but it has a weak collision resistance.
	// It can be used to prevent hash-flooding attacks or authenticate short-lived messages.
	"github.com/minio/highwayhash"
)

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
func Sanitize(str string) string {
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
		str,
	)
}

// SafeHeader stringifies a safe list of HTTP header values.
func SafeHeader(r *http.Request, header string) string {
	values := r.Header.Values(header)

	if len(values) == 0 {
		return ""
	}

	if len(values) == 1 {
		return Sanitize(values[0])
	}

	str := "["
	for i := range values {
		if i > 0 {
			str += " "
		}
		str += Sanitize(values[i])
	}
	str += "]"

	return str
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
	case r >= utf8.MaxRune:
		return false
	}
	return true
}

// printable returns the position (index) of
// a Carriage Return "\r", or a Line Feed "\n",
// or any other ASCII control code (except space),
// or, as well as, bn invalid UTF-8 code.
// printable returns -1 if the string
// is safely printable preventing log injection.
func printable(s string) int {
	for i, r := range s {
		if !PrintableRune(r) {
			return i
		}
	}
	return -1
}

// Printable returns -1 when all the strings are printable
// else returns the position of the rejected character.
func Printable(array ...string) int {
	for _, s := range array {
		if i := printable(s); i >= 0 {
			return i
		}
	}
	return -1
}

// RejectInvalidURI rejects HTTP requests having
// a Carriage Return "\r" or a Line Feed "\n"
// within the URI to prevent log injection.
func RejectInvalidURI(next http.Handler) http.Handler {
	log.Print("Middleware security: RejectLineBreakInURI")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if i := printable(r.RequestURI); i >= 0 {
				WriteErr(w, r, http.StatusBadRequest,
					"Invalid URI with non-printable symbol",
					"position", i)
				log.Print("WRN: reject non-printable URI or with <CR> or <LF>:", Sanitize(r.RequestURI))
				return
			}

			next.ServeHTTP(w, r)
		})
}

// TraversalPath returns true when path contains ".." to prevent path traversal attack.
func TraversalPath(w http.ResponseWriter, r *http.Request) bool {
	if strings.Contains(r.URL.Path, "..") {
		WriteErr(w, r, http.StatusBadRequest, "URL contains '..'")
		log.Print("WRN: reject path with '..' ", Sanitize(r.URL.Path))
		return true
	}
	return false
}

func RandomBytes(n int) []byte {
	key := make([]byte, n)
	if _, err := rand.Read(key); err != nil {
		log.Panic("RandomBytes: ", err)
	}
	return key
}

var hasherKey = RandomBytes(32)

// NewHash is based on HighwayHash, a hashing algorithm enabling high speed (especially on AMD64).
// See the study on HighwayHash and some other hash functions: https://github.com/fwessels/HashCompare
func NewHash() (hash.Hash, error) {
	h, err := highwayhash.New64(hasherKey)
	return h, err
}

// Obfuscate hashes the input string to prevent logging sensitive information.
func Obfuscate(str string) (string, error) {
	h, err := NewHash()
	if err != nil {
		return "", err
	}
	digest := h.Sum([]byte(str))
	return base64.StdEncoding.EncodeToString(digest), nil
}
