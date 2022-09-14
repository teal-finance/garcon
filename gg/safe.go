// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package gg

import (
	"crypto/rand"
	"encoding/base64"
	"hash"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/minio/highwayhash"
)

// The code points in the surrogate range are not valid for UTF-8.
const (
	surrogateMin = 0xD800
	surrogateMax = 0xDFFF
)

// Sanitize replaces control codes by the tofu symbol
// and invalid UTF-8 codes by the replacement character.
// Sanitize can be used to prevent log injection.
//
// Inspired from:
// - https://wikiless.org/wiki/Replacement_character#Replacement_character
// - https://graphicdesign.stackexchange.com/q/108297
func Sanitize(slice ...string) string {
	// most common case: one single string
	if len(slice) == 1 {
		return sanitize(slice[0])
	}

	// other cases: zero or multiple strings => use the slice representation
	str := strings.Join(slice, ", ")
	return "[" + sanitize(str) + "]"
}

func sanitize(str string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case surrogateMin <= r && r <= surrogateMax, r > utf8.MaxRune:
			// The replacement character U+FFFD indicates an invalid UTF-8 character.
			return '�'
		case unicode.IsPrint(r):
			return r
		default: // r < 32, r == 127
			// The empty box (tofu) symbolizes the .notdef character
			// indicating a valid but not rendered character.
			return '􏿮'
		}
	}, str)
}

func sanitizeFaster(str string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r < 32, r == 127: // The .notdef character is often represented by the empty box (tofu)
			return '􏿮' // to indicate a valid but not rendered character.
		case surrogateMin <= r && r <= surrogateMax, utf8.MaxRune < r:
			return '�' // The replacement character U+FFFD indicates an invalid UTF-8 character.
		}
		return r
	}, str)
}

// SplitCleanedLines splits on linefeed,
// drops redundant blank lines,
// replaces the non-printable runes by spaces,
// trims leading/trailing/redundant spaces,.
func SplitCleanedLines(str string) []string {
	// count number of lines in the returned txt
	n, m, max := 1, 0, 0
	r1, r2 := '\n', '\n'
	for _, r0 := range str {
		if r0 == '\r' {
			continue
		}
		if r0 == '\n' {
			if (r1 == '\n') && (r2 == '\n') {
				continue // skip redundant line feeds
			}
			n++
			if max < m {
				max = m // max line length
			}
			m = 0
		}
		r1, r2 = r0, r1
		m++
	}

	txt := make([]string, 0, n)
	line := make([]rune, 0, max)

	r1, r2 = '\n', '\n'
	wasSpace := true
	blank := false
	for _, r0 := range str {
		if r0 == '\r' {
			continue
		}
		if r0 == '\n' {
			if (r1 == '\n') && (r2 == '\n') {
				continue
			}
			if len(txt) > 0 || len(line) > 0 {
				if len(line) == 0 {
					blank = true
				} else {
					txt = append(txt, string(line))
					line = line[:0]
				}
			}
			wasSpace = true
			r1, r2 = r0, r1
			continue
		}
		r1, r2 = r0, r1

		// also replace non-printable characters by spaces
		isSpace := !unicode.IsPrint(r0) || unicode.IsSpace(r0)
		if isSpace {
			if wasSpace {
				continue // skip redundant whitespaces
			}
		} else {
			if wasSpace && len(line) > 0 {
				line = append(line, ' ')
			}
			line = append(line, r0)
			if blank {
				blank = false
				txt = append(txt, "")
			}
		}
		wasSpace = isSpace
	}

	if len(line) > 0 {
		txt = append(txt, string(line))
	}
	if len(txt) == 0 {
		return nil
	}
	return txt
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
// a Carriage Return "\r", a Line Feed "\n",
// another ASCII control code (except space),
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

// printable returns the position of
// a Carriage Return "\r", or a Line Feed "\n",
// or any other ASCII control code (except space),
// or, as well as, an invalid UTF-8 code.
// printable returns -1 if the string
// is safely printable preventing log injection.
func printable(s string) int {
	for p, r := range s {
		if !PrintableRune(r) {
			return p
		}
	}
	return -1
}

// Printable returns -1 when all the strings are safely printable
// else returns the position of the rejected character.
//
// The non printable characters are:
//
//   - Carriage Return "\r"
//   - Line Feed "\n"
//   - other ASCII control codes (except space)
//   - invalid UTF-8 codes
//
// Printable can be used to preventing log injection.
//
// When multiple strings are passed,
// the returned position is sum with the string index multiplied by 1000.
func Printable(array ...string) int {
	for i, s := range array {
		if p := printable(s); p >= 0 {
			return i*1000 + p
		}
	}
	return -1
}

func RandomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Panicf("RandomBytes(%d) %s", n, err)
	}
	return b
}

//nolint:gochecknoglobals // set at startup time, used as constant during runtime
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
	return base64.RawURLEncoding.EncodeToString(digest), nil
}
