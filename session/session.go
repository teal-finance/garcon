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
// Safer because of random salt generated for each token and understandable/auditable source code.
// Shorter because of Ascii85 (no Base64), compression and index instead of key names.
// Faster because of AES (no RSA) and custom bar-metal serializer.
package session

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/teal-finance/garcon/aead"
	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/security"
)

type Checker struct {
	resErr     reserr.ResErr
	cookie     http.Cookie
	devOrigins []string
	magic      byte
	cipher     aead.Cipher
}

const (
	authScheme        = "Bearer "
	secretTokenScheme = "i1" // See RFC 8959, i = the "incorruptible" format, 1 = the v1 version
	prefixScheme      = authScheme + secretTokenScheme + ":"

	invalidCookie = "invalid cookie"
	expiredRToken = "Refresh token has expired (or invalid)"

	oneYearInSeconds = 31556952 // average including leap years
	oneYearInNS      = oneYearInSeconds * 1_000_000_000
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
	cookie := createCookie("session", secure, dns, path, "a85 TODO")

	cipher, err := aead.New(secretKey)
	if err != nil {
		log.Panic("AES NewCipher ", err)
	}

	return &Checker{
		resErr:     resErr,
		cookie:     cookie,
		devOrigins: extractDevOrigins(urls),
		magic:      secretKey[0],
		cipher:     cipher,
	}
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

func createCookie(name string, secure bool, dns, path, a85 string) http.Cookie {
	// remove trailing slash
	if path != "" && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	log.Print("Create cookie ", name, " secure=", secure, " domain=", dns, " path=", path)

	return http.Cookie{
		Name:       name,
		Value:      a85,
		Path:       path,
		Domain:     dns,
		Expires:    time.Time{},
		RawExpires: "",
		MaxAge:     oneYearInSeconds,
		Secure:     secure,
		HttpOnly:   true,
		SameSite:   http.SameSiteStrictMode,
		Raw:        "",
		Unparsed:   []string{},
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

type Perm struct{}

// --------------------------------------
// Read/write permissions to/from context

// From gets the permission information from the request context.
func From(r *http.Request) Perm {
	perm, ok := r.Context().Value(permKey).(Perm)
	if !ok {
		log.Print("WRN token No permissions within the context ", r.URL.Path)
	}
	return perm
}

var permKey struct{}

// StoreInContext stores the permission info within the request context.
func (perm Perm) StoreInContext(r *http.Request) *http.Request {
	parentCtx := r.Context()
	childCtx := context.WithValue(parentCtx, permKey, perm)
	return r.WithContext(childCtx)
}
