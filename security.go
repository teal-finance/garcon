// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"net/http"

	"github.com/teal-finance/garcon/gg"
)

// MiddlewareRejectUnprintableURI is a middleware rejecting HTTP requests having
// a Carriage Return "\r" or a Line Feed "\n"
// within the URI to prevent log injection.
func (Garcon) MiddlewareRejectUnprintableURI() gg.Middleware {
	return MiddlewareRejectUnprintableURI
}

// MiddlewareRejectUnprintableURI is a middleware rejecting HTTP requests having
// a Carriage Return "\r" or a Line Feed "\n"
// within the URI to prevent log injection.
func MiddlewareRejectUnprintableURI(next http.Handler) http.Handler {
	log.Info("MiddlewareRejectUnprintableURI rejects URI having line breaks or unprintable characters")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if i := gg.Printable(r.RequestURI); i >= 0 {
				WriteErr(w, r, http.StatusBadRequest,
					"Invalid URI with non-printable symbol",
					"position", i)
				log.Warn("reject non-printable URI or with <CR> or <LF>:", gg.Sanitize(r.RequestURI))
				return
			}

			next.ServeHTTP(w, r)
		})
}

// MiddlewareSecureHTTPHeader is a middleware adding recommended HTTP response headers to secure the web application.
func MiddlewareSecureHTTPHeader(secure bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		log.Info("MiddlewareSecureHTTPHeader sets some secure HTTP headers")

		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Content-Type-Options", "nosniff")

				// secure must be false for http://localhost
				if secure {
					w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
				}

				if false {
					w.Header().Set("Content-Security-Policy", "TODO")
					// or
					w.Header().Set("Content-Security-Policy-Report-Only", "TODO")

					w.Header().Set("Referrer-Policy", "TODO")
					w.Header().Set("Forwarded", "TODO")
				}
			})
	}
}
