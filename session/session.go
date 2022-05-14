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

// Package session provides a safer, shorter, faster session cookie.
//
// :dart: Purpose
//
// Safer because of random salt in the tokens and understandable/auditable source code.
// Shorter because of Base91 (no Base64), compression and index instead of key names.
// Faster because of AES (no RSA) and custom bar-metal serializer.
//
// :closed_lock_with_key: Encryption
//
// The current trend about symmetric encryption is to prefer ChaCha20Poly1305 (server side).
// In addition to its cryptographic qualities, ChaCha20 is easy to configure,
// and requires few CPU/memory resources.
//
// On the other hand, on AMD and Intel processors, AES is faster (optimized instructions).
// Moreover, the Go crypto allows to configure AES well.
//
// See also: https://go.dev/blog/tls-cipher-suites
//
// Therefore this package uses only AES-GCM 128 bits (256 bits is not yet relevant in 2022).
// This may change for a future version, please share your thoughts.
//
// :cookie: Session cookie
//
// The serialization uses a format invented for the occasion
// which is called "incorruptible" (a mocktail that Garçon de café likes to serve).
// The format is: MagicCode (1 byte) + Radom (1 byte) + Presence bits (1 byte)
// + Token expiration (0 or 3 bytes) + Client IP (0, 4 or 16 bytes)
// + up to 31 other custom values (from 0 to 7900 bytes).
//
// See https://pkg.go.dev/github.com/teal-finance/garcon/session/incorruptible
//
// Optionally, some random padding can be appended. This feature is currently disabled.
//
// When the token is too long, its payload is compressed with Snappy S2.
//
// Then, the 128 bits AES-GCM encryption.
// This adds 16 bytes of header, including the authentication.
//
// Finally, the Base91 encoding, adding some more bytes.
//
// In the end, an "incorruptible" of 3 bytes (the minimum) becomes a Base91 of 21 or 22 bytes.
//
// :no_entry_sign: Limitations
//
// It works very well with a single server: the secrets could be generated at startup.
//
// On the other hand, in an environment with load-balancer,
// or with an authentication server, you have to share the encryption key.
// In this last case, the Quid solution (signature verified by public key) is to be preferred.
package session

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	// basexx "github.com/teal-finance/BaseXX/base92"
	basexx "github.com/mtraver/base91"
	"github.com/teal-finance/garcon/aead"
	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/session/dtoken"
)

type Session struct {
	resErr reserr.ResErr
	expiry time.Duration
	setIP  bool          // if true => put the remote IP in the token
	dtoken dtoken.DToken // the "tiny" token
	cookie http.Cookie
	isDev  bool
	cipher aead.Cipher
	magic  byte
	baseXX *basexx.Encoding
}

const (
	authScheme        = "Bearer "
	secretTokenScheme = "i:" // See RFC 8959, "i" means "incorruptible" format
	prefixScheme      = authScheme + secretTokenScheme

	// secondsPerYear = 31556952 // average including leap years
	// nsPerYear      = secondsPerYear * 1_000_000_000.
)

func New(urls []*url.URL, resErr reserr.ResErr, secretKey []byte, expiry time.Duration, setIP bool) *Session {
	if len(urls) == 0 {
		log.Panic("No urls => Cannot set Cookie domain")
	}

	secure, dns, path := extractMainDomain(urls[0])

	cipher, err := aead.New(secretKey)
	if err != nil {
		log.Panic("AES NewCipher ", err)
	}

	s := Session{
		resErr: resErr,
		expiry: expiry,
		setIP:  setIP,
		// the "tiny" token is the default token
		dtoken: dtoken.DToken{Expiry: 0, IP: nil, Values: nil},
		cookie: emptyCookie("session", secure, dns, path),
		isDev:  isLocalhost(urls),
		cipher: cipher,
		magic:  secretKey[0],
		baseXX: basexx.NewEncoding(noSpaceDoubleQuoteSemicolon),
	}

	// serialize the "tiny" token (with encryption and Base91 encoding)
	base91, err := s.Encode(s.dtoken)
	if err != nil {
		log.Panic("Encode(emptyToken) ", err)
	}

	// insert this generated token in the cookie
	s.cookie.Value = secretTokenScheme + base91

	return &s
}

func (s Session) NewCookie(dt dtoken.DToken) (http.Cookie, error) {
	base91, err := s.Encode(dt)
	if err != nil {
		return s.cookie, err
	}

	cookie := s.NewCookieFromToken(base91, dt.ExpiryTime())
	return cookie, nil
}

func (s Session) NewCookieFromToken(base91 string, expiry time.Time) http.Cookie {
	cookie := s.cookie
	cookie.Value = secretTokenScheme + base91

	if expiry.IsZero() {
		cookie.Expires = time.Time{} // time.Now().Add(nsPerYear)
	} else {
		cookie.Expires = expiry
	}

	return cookie
}

func (s Session) SetCookie(w http.ResponseWriter, r *http.Request) dtoken.DToken {
	dt := s.dtoken     // copy the "tiny" token
	cookie := s.cookie // copy the default cookie

	if s.expiry <= 0 {
		cookie.Expires = time.Time{} // time.Now().Add(nsPerYear)
	} else {
		cookie.Expires = time.Now().Add(s.expiry)
		dt.SetExpiry(s.expiry)
	}

	if s.setIP {
		err := dt.SetRemoteIP(r)
		if err != nil {
			log.Panic(err)
		}
	}

	requireNewEncoding := (s.expiry > 0) || s.setIP
	if requireNewEncoding {
		base91, err := s.Encode(dt)
		if err != nil {
			log.Panic(err)
		}
		cookie.Value = secretTokenScheme + base91
	}

	http.SetCookie(w, &cookie)
	return dt
}

// Supported URL shemes.
const (
	HTTP  = "http"
	HTTPS = "https"
)

func extractMainDomain(url *url.URL) (secure bool, dns, path string) {
	if url == nil {
		log.Panic("No URL => Cannot set Cookie domain")
	}

	switch {
	case url.Scheme == HTTP:
		secure = false

	case url.Scheme == HTTPS:
		secure = true

	default:
		log.Panic("Unexpected scheme in ", url)
	}

	return secure, url.Hostname(), url.Path
}

func isLocalhost(urls []*url.URL) bool {
	if len(urls) > 0 && urls[0].Scheme == "http" {
		host, _, _ := net.SplitHostPort(urls[0].Host)
		if host == "localhost" {
			log.Print("Session DevMode accept missing/invalid token ", urls[0])
			return true
		}
	}

	log.Print("Session ProdMode (require valid token) because no http://localhost in first of ", urls)
	return false
}

func emptyCookie(name string, secure bool, dns, path string) http.Cookie {
	if path != "" && path[len(path)-1] == '/' {
		path = path[:len(path)-1] // remove trailing slash
	}

	return http.Cookie{
		Name:       name,
		Value:      "", // emptyCookie because no token
		Path:       path,
		Domain:     dns,
		Expires:    time.Time{},
		RawExpires: "",
		MaxAge:     0, // secondsPerYear,
		Secure:     secure,
		HttpOnly:   true,
		SameSite:   http.SameSiteStrictMode,
		Raw:        "",
		Unparsed:   nil,
	}
}
