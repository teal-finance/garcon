// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"log"
	"net/http"
	"time"
)

// LogRequest is the middleware to log the requester IP and the requested URL.
func LogRequest(next http.Handler) http.Handler {
	log.Print("INF Middleware logger: requester IP and requested URL")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Print(ipMethodURL(r))
			next.ServeHTTP(w, r)
		})
}

// LogSafeRequest is similar to LogRequest but sanitize the URL.
func LogSafeRequest(next http.Handler) http.Handler {
	log.Print("INF Middleware logger: requester IP and sanitized URL")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Print(safeIPMethodURL(r))
			next.ServeHTTP(w, r)
		})
}

// LogRequestFingerprint is the middleware to log
// incoming HTTP request and browser fingerprint.
func LogRequestFingerprint(next http.Handler) http.Handler {
	log.Print("INF Middleware logger: requested URL and browser fingerprint: " + FingerprintExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// double space after "in" is for padding with "out" logs
			log.Print(ipMethodURL(r) + fingerprint(r))
			next.ServeHTTP(w, r)
		})
}

// LogSafeRequestFingerprint is similar to LogRequestFingerprint but sanitize the URL.
func LogSafeRequestFingerprint(next http.Handler) http.Handler {
	log.Print("INF Middleware logger: sanitized URL and browser fingerprint: " + FingerprintExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// double space after "in" is for padding with "out" logs
			log.Print(safeIPMethodURL(r) + fingerprint(r))
			next.ServeHTTP(w, r)
		})
}

// LogDuration logs the requested URL along with the time to handle it.
func LogDuration(next http.Handler) http.Handler {
	log.Print("INF Middleware logger: requester IP, requested URL and duration")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		d := time.Since(start)
		log.Print(ipMethodURLDuration(r, d))
	})
}

// LogSafeDuration is similar to LogDuration but also sanitizes the URL.
func LogSafeDuration(next http.Handler) http.Handler {
	log.Print("INF Middleware logger: requester IP, sanitized URL and duration")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		d := time.Since(start)
		log.Print(safeIPMethodURLDuration(r, d))
	})
}

func ipMethodURL(r *http.Request) string {
	// double space after "in" is for padding with "out" logs
	return "INF in  " + r.RemoteAddr + " " + r.Method + " " + r.RequestURI
}

func safeIPMethodURL(r *http.Request) string {
	return "INF in  " + r.RemoteAddr + " " + r.Method + " " + Sanitize(r.RequestURI)
}

func ipMethodURLDuration(r *http.Request, d time.Duration) string {
	return "INF out " + r.RemoteAddr + " " + r.Method + " " +
		r.RequestURI + " " + d.String()
}

func safeIPMethodURLDuration(r *http.Request, d time.Duration) string {
	return "INF out " + r.RemoteAddr + " " + r.Method + " " +
		Sanitize(r.RequestURI) + " " + d.String()
}

// FingerprintExplanation provides a description of the logged HTTP headers.
const FingerprintExplanation = `
1. Accept-Language, the language preferred by the user. 
2. User-Agent, name and version of the browser and OS. 
3. R=Referer, the website from which the request originated. 
4. A=Accept, the content types the browser prefers. 
5. E=Accept-Encoding, the compression formats the browser supports. 
6. Connection, can be empty, "keep-alive" or "close". 
7. Cache-Control, how the browser is caching data. 
8. URI=Upgrade-Insecure-Requests, the browser can upgrade from HTTP to HTTPS. 
9. Via avoids request loops and identifies protocol capabilities. 
10. Authorization or Cookie (both should not be present at the same time). 
11. DNT (Do Not Track) is being dropped by web browsers.`

// fingerprint logs like logIPMethodURL and also logs the browser fingerprint.
// Attention! fingerprint provides personal data that may identify users.
// To comply with GDPR, the website data owner must have a legitimate reason to do so.
// Before enabling the fingerprinting, the user must understand it
// and give their freely-given informed consent such as the settings change from "no" to "yes".
func fingerprint(r *http.Request) string {
	// double space after "in" is for padding with "out" logs
	line := " " +
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
		headerTxt(r, "Connection", "", "") +
		// 7. Cache-Control, how the browser is caching data.
		headerTxt(r, "Cache-Control", "", "") +
		// 8. Upgrade-Insecure-Requests, the browser can upgrade from HTTP to HTTPS
		headerTxt(r, "Upgrade-Insecure-Requests", "UIR=", "1") +
		// 9. Via avoids request loops and identifies protocol capabilities
		headerTxt(r, "Via", "Via=", "") +
		// 10. Authorization and Cookie: both should not be present at the same time
		headerTxt(r, "Authorization", "", "") +
		headerTxt(r, "Cookie", "", "")

	// 11, DNT (Do Not Track) is being dropped by web browsers.
	if r.Header.Get("DNT") != "" {
		line += " DNT"
	}

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
