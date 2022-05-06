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
// Shorter because of Ascii85 (no Base64), compression and index instead of key names.
// Faster because of AES (no RSA) and custom bar-metal serializer.
package session

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/teal-finance/garcon/aead"
	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/security"
	"github.com/teal-finance/garcon/session/dtoken"
)

type Checker struct {
	resErr     reserr.ResErr
	dtoken     dtoken.DToken
	cookie     http.Cookie
	devOrigins []string
	cipher     aead.Cipher
	magic      byte
}

const (
	authScheme        = "Bearer "
	secretTokenScheme = "i:" // See RFC 8959, "i" means "incorruptible" format
	prefixScheme      = authScheme + secretTokenScheme

	invalidCookie = "invalid cookie"
	expiredRToken = "Refresh token has expired (or invalid)"

	secondsPerYear = 31556952 // average including leap years
	nsPerYear      = secondsPerYear * 1_000_000_000
)

var (
	ErrUnauthorized = errors.New("token not authorized")
	ErrNoTokenFound = errors.New("no token found")
)

func New(urls []*url.URL, resErr reserr.ResErr, secretKey [16]byte) *Checker {
	if len(urls) == 0 {
		log.Panic("No urls => Cannot set Cookie domain")
	}

	secure, dns, path := extractMainDomain(urls[0])

	cipher, err := aead.New(secretKey)
	if err != nil {
		log.Panic("AES NewCipher ", err)
	}

	ck := Checker{
		resErr:     resErr,
		dtoken:     dtoken.DToken{Expiry: 0, IP: nil, Values: nil}, // the "tiny" token
		cookie:     emptyCookie("session", secure, dns, path),
		devOrigins: extractDevOrigins(urls),
		cipher:     cipher,
		magic:      secretKey[0],
	}

	// serialize the "tiny" token (with encryption and Ascii85 encoding)
	a85, err := ck.Encode(ck.dtoken)
	if err != nil {
		log.Panic("Encode(emptyToken) ", err)
	}

	// insert this generated token in the cookie
	ck.cookie.Value = secretTokenScheme + string(a85)

	return &ck
}

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

func extractDevURLs(urls []*url.URL) (devURLs []*url.URL) {
	if len(urls) == 1 {
		log.Print("Token required for single domain: ", urls)
		return nil
	}

	for i, u := range urls {
		if u == nil {
			log.Panic("Unexpected nil in URL slide: ", urls)
		}

		if u.Scheme == HTTP {
			return urls[i:]
		}
	}

	return nil
}

func extractDevOrigins(urls []*url.URL) (devOrigins []string) {
	if len(urls) > 0 && urls[0].Scheme == "http" && urls[0].Host == "localhost" {
		return []string{"*"} // Accept absence of cookie for http://localhost
	}

	devURLS := extractDevURLs(urls)

	if len(devURLS) == 0 {
		return nil
	}

	devOrigins = make([]string, 0, len(urls))

	for _, u := range urls {
		o := u.Scheme + "://" + u.Host
		devOrigins = append(devOrigins, o)
	}

	log.Print("Session not required for dev. origins: ", devOrigins)
	return devOrigins
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
		MaxAge:     secondsPerYear,
		Secure:     secure,
		HttpOnly:   true,
		SameSite:   http.SameSiteStrictMode,
		Raw:        "",
		Unparsed:   nil,
	}
}

func (ck *Checker) IsDevOrigin(r *http.Request) bool {
	if len(ck.devOrigins) == 0 {
		return false
	}

	if len(ck.devOrigins) > 0 {
		origin := r.Header.Get("Origin")
		sanitized := security.Sanitize(origin)

		for _, prefix := range ck.devOrigins {
			if prefix == "*" {
				log.Print("No token but addr=http://localhost => Accept any origin=", sanitized)
				return true
			}
			if strings.HasPrefix(origin, prefix) {
				log.Printf("No token but origin=%v is a valid dev origin", sanitized)
				return true
			}
		}

		log.Print("No token and origin=", sanitized, " has not prefixes ", ck.devOrigins)
	}

	return false
}

// Set puts a "session" cookie when the request has no valid "incorruptible" token.
// The token is searched the "session" cookie and in the first "Authorization" header.
// The "session" cookie (that is added in the response) contains the "tiny" token.
// Finally, Set stores the decoded token in the request context.
func (ck *Checker) Set(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dt, err := ck.DecodeToken(r)
		if err != nil {
			dt = ck.dtoken // default token
			ck.cookie.Expires = time.Now().Add(nsPerYear)
			http.SetCookie(w, &ck.cookie)
		}
		next.ServeHTTP(w, dt.PutInCtx(r))
	})
}

// Chk accepts requests only if it has a valid cookie.
// Chk does not verify the "Authorization" header.
// See the Vet() function to also verify the "Authorization" header.
// Chk also stores the decoded token in the request context.
// In dev. testing, Chk accepts any request but does not store invalid tokens.
func (ck *Checker) Chk(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dt, err := ck.DecodeTokenFromCookie(r)
		if err == nil { // OK: put the token in the request context
			r = dt.PutInCtx(r)
		} else if !ck.IsDevOrigin(r) {
			ck.resErr.Write(w, r, http.StatusUnauthorized, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Vet accepts requests having a valid token either in
// the "session" cookie or in the first "Authorization" header.
// Vet also stores the decoded token in the request context.
// In dev. testing, Vet accepts any request but does not store invalid tokens.
func (ck *Checker) Vet(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dt, err := ck.DecodeToken(r)
		if err == nil { // OK: put the token in the request context
			r = dt.PutInCtx(r)
		} else if !ck.IsDevOrigin(r) {
			ck.resErr.Write(w, r, http.StatusUnauthorized, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (ck *Checker) DecodeToken(r *http.Request) (dt dtoken.DToken, err error) {
	var bearer bool

	ascii85, err := ck.TokenFromCookie(r)
	if err != nil {
		var errBearer error
		ascii85, errBearer = ck.TokenFromBearer(r)
		if errBearer != nil {
			err = fmt.Errorf("no 'incorruptible' token "+
				"in either the first 'Authorization' HTTP header "+
				"or in the cookie %q because: %w and %v",
				ck.cookie.Name, errBearer, err.Error())
			return dt, err
		}
		bearer = true
	}

	if ck.equalDefaultToken(ascii85) {
		return ck.dtoken, nil
	}

	dt, err = ck.Decode(ascii85)
	if (err != nil) && !bearer {
		ascii85, errBearer := ck.TokenFromBearer(r)
		if errBearer != nil {
			err = fmt.Errorf("no valid 'incorruptible' token "+
				"in either the first 'Authorization' HTTP header "+
				"or in the cookie %q because: %w and %v",
				ck.cookie.Name, errBearer, err.Error())
			return dt, err
		}

		dt, errBearer = ck.Decode(ascii85)
		if errBearer != nil {
			err = fmt.Errorf("no valid 'incorruptible' token "+
				"in either the first 'Authorization' HTTP header "+
				"or in the cookie %q because: %w and %v",
				ck.cookie.Name, errBearer, err.Error())
			return dt, err
		}

		err = nil
	}

	return dt, err
}

func (ck *Checker) DecodeTokenFromCookie(r *http.Request) (dt dtoken.DToken, err error) {
	ascii85, err := ck.TokenFromCookie(r)
	if err != nil {
		return dt, err
	}

	if ck.equalDefaultToken(ascii85) {
		return ck.dtoken, nil
	}

	return ck.Decode(ascii85)
}

func (ck *Checker) DecodeTokenFromBearer(r *http.Request) (dt dtoken.DToken, err error) {
	ascii85, err := ck.TokenFromBearer(r)
	if err != nil {
		return dt, err
	}

	if ck.equalDefaultToken(ascii85) {
		return ck.dtoken, nil
	}

	return ck.Decode(ascii85)
}

func (ck *Checker) TokenFromCookie(r *http.Request) (ascii85 string, err error) {
	cookie, err := r.Cookie(ck.cookie.Name)
	if err != nil {
		return "", err
	}

	if !cookie.HttpOnly {
		return "", errors.New("no HttpOnly cookie")
	}
	if cookie.SameSite != ck.cookie.SameSite {
		return "", fmt.Errorf("want cookie SameSite=%v but got %v", ck.cookie.SameSite, cookie.SameSite)
	}
	if cookie.Secure != ck.cookie.Secure {
		return "", fmt.Errorf("want cookie Secure=%v but got %v", ck.cookie.Secure, cookie.Secure)
	}

	ascii85, err = trimTokenScheme(cookie.Value)
	if err != nil {
		return "", err
	}

	return ascii85, nil
}

func (ck *Checker) TokenFromBearer(r *http.Request) (ascii85 string, err error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", errors.New("no 'Authorization: " + secretTokenScheme + "xxxxxxxx' in the request header")
	}

	ascii85, err = trimBearerScheme(auth)
	if err != nil {
		return "", err
	}

	return ascii85, err
}

// equalDefaultToken compares with the default token
// by skiping the token scheme.
func (ck *Checker) equalDefaultToken(ascii85 string) bool {
	const n = len(secretTokenScheme)
	return (ascii85 == ck.cookie.Value[n:])
}

func trimTokenScheme(uri string) (ascii85 string, err error) {
	const n = len(secretTokenScheme)
	if len(uri) < n+a85MinSize {
		return "", fmt.Errorf("token URI too short (%d bytes) want %d", len(uri), n+a85MinSize)
	}
	if uri[:n] != secretTokenScheme {
		return "", fmt.Errorf("want token URI '"+secretTokenScheme+"xxxxxxxx' got %q", uri)
	}
	return uri[n:], nil
}

func trimBearerScheme(auth string) (ascii85 string, err error) {
	const n = len(prefixScheme)
	if len(auth) < n+a85MinSize {
		return "", fmt.Errorf("bearer too short (%d bytes) want %d", len(auth), n+a85MinSize)
	}
	if auth[:n] != prefixScheme {
		return "", fmt.Errorf("want '"+prefixScheme+"xxxxxxxx' got %s", auth)
	}
	return auth[n:], nil
}
