// Copyright (C) 2020-2021 TealTicks contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package reqlog logs incoming request URL and requester information.
package reqlog

import (
	"log"
	"net/http"
)

// LogRequests is the middleware to log the incoming HTTP requests.
func LogRequests(next http.Handler) http.Handler {
	log.Print("Middleware logger: requester IP and requested URL")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			LogIPAndURL(r)
			next.ServeHTTP(w, r)
		})
}

// LogVerbose is the middleware to log the incoming HTTP requests and verbose requester information.
func LogVerbose(next http.Handler) http.Handler {
	log.Print("Middleware logger: requested URL, remote IP and also: " + BrowserInfoExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			LogURLAndBrowserInfo(r)
			next.ServeHTTP(w, r)
		})
}

// LogIPAndURL logs the requester IP and the requested URL.
func LogIPAndURL(r *http.Request) {
	log.Printf("in  %v %v %v", r.RemoteAddr, r.Method, r.RequestURI)
}

// LogURLAndBrowserInfo extends LogReqIPAndURL by logging much more requester information.
// Attention! Collecting such requester information is considered as browser fingerprints.
// When fingerprinting is used to identify users, it is part of the personal data
// and must comply with GDPR. In that case the website must have a legitimate reason to do so.
// Before enabling the fingerprinting, the user must understand it and give their freely-given informed consent such as the settings change from “no” to “yes”.
func LogURLAndBrowserInfo(r *http.Request) {
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
		line += " " + a
	}

	if c := r.Header.Get("Cookie"); c != "" {
		line += " " + c
	}

	log.Print(line)
}

// BrowserInfoExplanation provides a description of the logged HTTP headers.
const BrowserInfoExplanation = `
1. Accept-Language, the language preferred by the user. 
2. User-Agent, name and version of the browser and OS. 
3. R=Referer, the website from which the request originated. 
4. A=Accept, the content types the browser prefers. 
5. E=Accept-Encoding, the compression formats the browser supports. 
6. Connection, can be empty, "keep-alive" or "close". 
7, DNT (Do Not Track) is being dropped by web standards and browsers. 
8. Cache-Control, how the browser is caching data. 
9. Authorization and/or Cookie content.`
