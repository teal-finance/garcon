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
			LogReqIPAndURL(r)
			next.ServeHTTP(w, r)
		})
}

// LogVerbose is the middleware to log the incoming HTTP requests and verbose requester information.
func LogVerbose(next http.Handler) http.Handler {
	log.Print("Middleware logger: requested URL, remote IP and also: " + RequesterInfoExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			LogURLAndRequesterInfo(r)
			next.ServeHTTP(w, r)
		})
}

// LogReqIPAndURL logs the requester IP and the requested URL.
func LogReqIPAndURL(r *http.Request) {
	log.Printf("in  %v %v %v", r.RemoteAddr, r.Method, r.RequestURI)
}

// LogURLAndRequesterInfo is similar to LogReqIPAndURL, but also logs much more requester information.
func LogURLAndRequesterInfo(r *http.Request) {
	log.Printf("in  %v %v %v R=%q L=%q U=%q A=%q E=%q C=%q",
		r.RemoteAddr, r.Method, r.RequestURI,
		r.Header.Get("Referer"), r.Header.Get("Accept-Language"),
		r.Header.Get("User-Agent"), r.Header.Get("Accept"),
		r.Header.Get("Accept-Encoding"), r.Header.Get("Accept-Charset"))
}

// RequesterInfoExplanation provides a description of the logged HTTP headers.
const RequesterInfoExplanation = `
R=Referer, the website from which the request originated. 
L=Accept-Language, the language preferred by the user. 
U=User-Agent, name and version of the browser and OS. 
A=Accept, the content types the browser prefers. 
E=Accept-Encoding, the compression formats the browser supports. 
C=Accept-Charset, the character-set the browser prefers.`
