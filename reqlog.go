// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

// Package reqlog logs incoming request URL and browser fingerprint.
package garcon

import (
	"log"
	"net/http"
)

// LogRequest is the middleware to log incoming HTTP request.
func LogRequest(next http.Handler) http.Handler {
	log.Print("Middleware logger: requester IP and requested URL")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logIPMethodURL(r)
			next.ServeHTTP(w, r)
		})
}

// LogRequestFingerprint is the middleware to log
// incoming HTTP request and browser fingerprint.
func LogRequestFingerprint(next http.Handler) http.Handler {
	log.Print("Middleware logger: requested URL, remote IP and also: " + FingerprintExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Print(fingerprint(r))
			next.ServeHTTP(w, r)
		})
}

// logIPMethodURL logs the requester IP and requested URL.
func logIPMethodURL(r *http.Request) {
	log.Print("in  ", r.RemoteAddr, " ", r.Method, " ", r.RequestURI)
}

// fingerprint logs like logIPMethodURL and also logs the browser fingerprint.
// Attention! fingerprint provides personal data that may identify users.
// To comply with GDPR, the website data owner must have a legitimate reason to do so.
// Before enabling the fingerprinting, the user must understand it
// and give their freely-given informed consent such as the settings change from “no” to “yes”.
func fingerprint(r *http.Request) string {
	line := "in  " +
		r.RemoteAddr + " " +
		r.Method + " " +
		r.RequestURI + " " +
		// 1. Accept-Language, the language preferred by the user.
		SafeHeader(r, "Accept-Language") + " " +
		// 2. User-Agent, name and version of the browser and OS.
		SafeHeader(r, "User-Agent")

	// 3. R=Referer, the website from which the request originated.
	if referer := SafeHeader(r, "Referer"); referer != "" {
		line += " R=" + referer
	}

	// 4. A=Accept, the content types the browser prefers.
	if a := SafeHeader(r, "Accept"); a != "" {
		line += " A=" + a
	}

	// 5. E=Accept-Encoding, the compression formats the browser supports.
	if ae := SafeHeader(r, "Accept-Encoding"); ae != "" {
		line += " E=" + ae
	}

	// 6. Connection, can be empty, "keep-alive" or "close".
	if c := SafeHeader(r, "Connection"); c != "" {
		line += " " + c
	}

	// 7, DNT (Do Not Track) is being dropped by web standards and browsers.
	if r.Header.Get("DNT") != "" {
		line += " DNT"
	}

	// 8. Cache-Control, how the browser is caching data.
	if cc := SafeHeader(r, "Cache-Control"); cc != "" {
		line += " " + cc
	}

	// 9. Authorization and/or Cookie content.

	if a := SafeHeader(r, "Authorization"); a != "" {
		checksum, err := Obfuscate(a)
		if err == nil {
			line += " " + checksum
		} else {
			log.Print("WRN Cannot create HighwayHash ", err)
		}
	}

	if c := SafeHeader(r, "Cookie"); c != "" {
		line += " " + c
	}

	return Sanitize(line)
}

// FingerprintMD provide the browser fingerprint in markdown format.
// Attention: read the .
func FingerprintMD(r *http.Request) string {
	return "" +
		headerMD(r, "Accept-Language") + // language preferred by the user
		headerMD(r, "User-Agent") + // name and version of browser and OS
		headerMD(r, "Referer") + // URL from which the request originated
		headerMD(r, "Accept") + // content types the browser prefers
		headerMD(r, "Accept-Encoding") + // compression formats the browser supports
		headerMD(r, "Connection") + // can be: empty, "keep-alive" or "close"
		headerMD(r, "DNT") + // "Do Not Track" is being dropped by web standards and browsers
		headerMD(r, "Cache-Control") + // how the browser is caching data
		headerMD(r, "Authorization") + // Attention: may contain confidential data
		headerMD(r, "Cookie") // Attention: may contain confidential data
}

func headerMD(r *http.Request, header string) string {
	str := SafeHeader(r, header)
	if str != "" {
		str = "\n" + "* " + header + ": " + str
	}
	return str
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
