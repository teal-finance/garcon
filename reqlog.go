// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
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
		SafeHeader(r, "User-Agent") +
		// 3. R=Referer, the website from which the request originated.
		headerTxt(r, "Referer", "R=", "") +
		// 4. A=Accept, the content types the browser prefers.
		headerTxt(r, "Accept", "A=", "") +
		// 5. E=Accept-Encoding, the compression formats the browser supports.
		headerTxt(r, "Accept-Encoding", "E=", "") +
		// 6. Connection, can be empty, "keep-alive" or "close".
		headerTxt(r, "Connection", "", "")
	// 7, DNT (Do Not Track) is being dropped by web standards and browsers.
	if r.Header.Get("DNT") != "" {
		line += " DNT"
	}
	line += "" +
		// 8. Cache-Control, how the browser is caching data.
		headerTxt(r, "Cache-Control", "", "") +
		// 9. Upgrade-Insecure-Requests, the client can upgrade from HTTP to HTTPS
		headerTxt(r, "Upgrade-Insecure-Requests", "UIR", "1") +
		// 10. Via avoids request loops and identifies protocol capabilities
		headerTxt(r, "Via", "Via=", "") +
		// 11. Authorization and/or Cookie content.
		headerTxt(r, "Authorization", "", "") +
		headerTxt(r, "Cookie", "", "")

	return line
}

// FingerprintMD provide the browser fingerprint in markdown format.
// Attention: read the .
func FingerprintMD(r *http.Request) string {
	return "" +
		headerMD(r, "Accept-Encoding") + // compression formats the browser supports
		headerMD(r, "Accept-Language") + // language preferred by the user
		headerMD(r, "Accept") + // content types the browser prefers
		headerMD(r, "Authorization") + // Attention: may contain confidential data
		headerMD(r, "Cache-Control") + // how the browser is caching data
		headerMD(r, "Connection") + // can be: empty, "keep-alive" or "close"
		headerMD(r, "Cookie") + // Attention: may contain confidential data
		headerMD(r, "DNT") + // "Do Not Track" is being dropped by web standards and browsers
		headerMD(r, "Referer") + // URL from which the request originated
		headerMD(r, "User-Agent") + // name and version of browser and OS
		headerMD(r, "Via") // avoid request loops and identify protocol capabilities
}

func headerTxt(r *http.Request, header, key, skip string) string {
	v := SafeHeader(r, header)
	if v == skip {
		return ""
	}
	return " " + key + "=" + v
}

func headerMD(r *http.Request, header string) string {
	v := SafeHeader(r, header)
	if v == "" {
		return ""
	}
	return "\n" + "* " + header + ": " + v
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
9. URI=Upgrade-Insecure-Requests, the client can upgrade from HTTP to HTTPS 
10. Via avoids request loops and identifies protocol capabilities 
11. Authorization and/or Cookie content (obfuscated).`
