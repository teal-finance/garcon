// #region <editor-fold desc="Preamble">
// Copyright (c) 2021-2022 Teal.Finance contributors
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

// Package reqlog logs incoming request URL and browser fingerprints.
package reqlog

import (
	"log"
	"net/http"

	"github.com/teal-finance/garcon/security"
)

// LogRequests is the middleware to log the incoming HTTP requests.
func LogRequests(next http.Handler) http.Handler {
	log.Print("Middleware logger: requester IP and requested URL")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			LogIPMethodURL(r)
			next.ServeHTTP(w, r)
		})
}

// LogVerbose is the middleware to log the incoming HTTP requests and verbose browser fingerprints.
func LogVerbose(next http.Handler) http.Handler {
	log.Print("Middleware logger: requested URL, remote IP and also: " + FingerprintExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Print(IPMethodURLFingerprint(r))
			next.ServeHTTP(w, r)
		})
}

// LogIPMethodURL logs the requester IP and the requested URL.
func LogIPMethodURL(r *http.Request) {
	log.Print("in  ", r.RemoteAddr, " ", r.Method, " ", r.RequestURI)
}

// IPMethodURLFingerprint extends LogReqIPAndURL by logging browser fingerprints.
// Attention! When fingerprinting is used to identify users, it is part of the personal data
// and must comply with GDPR. In that case, the website must have a legitimate reason to do so.
// Before enabling the fingerprinting, the user must understand it
// and give their freely-given informed consent such as the settings change from “no” to “yes”.
func IPMethodURLFingerprint(r *http.Request) string {
	line := "in  " +
		r.RemoteAddr + " " +
		r.Method + " " +
		r.RequestURI + " " +
		// 1. Accept-Language, the language preferred by the user.
		r.Header.Get("Accept-Language") + " " +
		// 2. User-Agent, name and version of the browser and OS.
		r.Header.Get("User-Agent")

	// 3. R=Referer, the website from which the request originated.
	if referer := r.Header.Get("Referer"); referer != "" {
		line += " R=" + referer
	}

	// 4. A=Accept, the content types the browser prefers.
	if a := r.Header.Get("Accept"); a != "" {
		line += " A=" + a
	}

	// 5. E=Accept-Encoding, the compression formats the browser supports.
	if ae := r.Header.Get("Accept-Encoding"); ae != "" {
		line += " E=" + ae
	}

	// 6. Connection, can be empty, "keep-alive" or "close".
	if c := r.Header.Get("Connection"); c != "" {
		line += " " + c
	}

	// 7, DNT (Do Not Track) is being dropped by web standards and browsers.
	if r.Header.Get("DNT") != "" {
		line += " DNT"
	}

	// 8. Cache-Control, how the browser is caching data.
	if cc := r.Header.Get("Cache-Control"); cc != "" {
		line += " " + cc
	}

	// 9. Authorization and/or Cookie content.

	if a := r.Header.Get("Authorization"); a != "" {
		checksum, err := security.Obfuscate(a)
		if err == nil {
			line += " " + checksum
		} else {
			log.Print("WAR Cannot create HighwayHash ", err)
		}
	}

	if c := r.Header.Get("Cookie"); c != "" {
		line += " " + c
	}

	return security.Sanitize(line)
}

// FingerprintExplanation provides a description of the logged HTTP headers.
const FingerprintExplanation = `
1. Accept-Language, the language preferred by the user. 
2. User-Agent, name and version of the browser and OS. 
3. R=Referer, the website from which the request originated. 
4. A=Accept, the content types the browser prefers. 
5. E=Accept-Encoding, the compression formats the browser supports. 
6. Connection, can be empty, "keep-alive" or "close". 
7. DNT (Do Not Track) can be used by Firefox (dropped by web standards). 
8. Cache-Control, how the browser is caching data. 
9. Authorization (obfuscated) and/or Cookie content.`
