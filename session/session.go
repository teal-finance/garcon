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
// Safer because of random salt in the tokens and understandable/auditable source code.
// Shorter because of Base92 (no Base64), compression and index instead of key names.
// Faster because of AES (no RSA) and custom bar-metal serializer.
package session

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/teal-finance/garcon/aead"
	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/session/dtoken"
)

type Session struct {
	resErr      reserr.ResErr
	expiry      time.Duration
	setIP       bool          // if true => put the remote IP in the token
	dtoken      dtoken.DToken // the "tiny" token
	cookie      http.Cookie
	isLocalhost bool
	cipher      aead.Cipher
	magic       byte
}

const (
	authScheme        = "Bearer "
	secretTokenScheme = "i:" // See RFC 8959, "i" means "incorruptible" format
	prefixScheme      = authScheme + secretTokenScheme

	// secondsPerYear = 31556952 // average including leap years
	// nsPerYear      = secondsPerYear * 1_000_000_000.
)

func New(urls []*url.URL, resErr reserr.ResErr, secretKey [16]byte, expiry time.Duration, setIP bool) *Session {
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
		dtoken:      dtoken.DToken{Expiry: 0, IP: nil, Values: nil},
		cookie:      emptyCookie("session", secure, dns, path),
		isLocalhost: isLocalhost(urls),
		cipher:      cipher,
		magic:       secretKey[0],
	}

	// serialize the "tiny" token (with encryption and Base92 encoding)
	base92, err := s.Encode(s.dtoken)
	if err != nil {
		log.Panic("Encode(emptyToken) ", err)
	}

	// insert this generated token in the cookie
	s.cookie.Value = secretTokenScheme + base92

	return &s
}

func (s Session) NewCookie(dt dtoken.DToken) (http.Cookie, error) {
	base92, err := s.Encode(dt)
	if err != nil {
		return s.cookie, err
	}

	cookie := s.NewCookieFromToken(base92, dt.ExpiryTime())
	return cookie, nil
}

func (s Session) NewCookieFromToken(base92 string, expiry time.Time) http.Cookie {
	cookie := s.cookie
	cookie.Value = secretTokenScheme + base92

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
		base92, err := s.Encode(dt)
		if err != nil {
			log.Panic(err)
		}
		cookie.Value = secretTokenScheme + base92
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
			log.Print("Session dev mode accept absence of cookie ", urls[0])
			return true
		}
	}

	log.Print("Session prod mode because no http://localhost in first of ", urls)
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
