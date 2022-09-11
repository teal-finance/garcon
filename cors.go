// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/cors"

	"github.com/teal-finance/garcon/gg"
)

// MiddlewareCORS is a middleware to handle Cross-Origin Resource Sharing (CORS).
func (g *Garcon) MiddlewareCORS() gg.Middleware {
	return g.MiddlewareCORSWithMethodsHeaders(nil, nil)
}

// MiddlewareCORSWithMethodsHeaders is a middleware to handle Cross-Origin Resource Sharing (CORS).
func (g *Garcon) MiddlewareCORSWithMethodsHeaders(methods, headers []string) gg.Middleware {
	return MiddlewareCORS(g.origins, methods, headers, g.devMode)
}

// MiddlewareCORS uses restrictive CORS values.
func MiddlewareCORS(origins, methods, headers []string, debug bool) func(next http.Handler) http.Handler {
	c := newCORS(origins, methods, headers, debug)
	if c.Log != nil {
		c.Log = corsLogger{}
	}
	return c.Handler
}

type corsLogger struct{}

func (corsLogger) Printf(fmt string, a ...any) {
	if strings.Contains(fmt, "Actual request") {
		return
	}
	log.Securityf("CORS "+fmt, a...)
}

// DevOrigins provides the development origins:
// - yarn run vite --port 3000
// - yarn run vite preview --port 5000
// - localhost:8085 on multi devices: web auto-reload using https://github.com/synw/fwr
// - flutter run --web-port=8080
// - 192.168.1.x + any port on tablet: mobile app using fast builtin auto-reload.
func DevOrigins() []*url.URL {
	return []*url.URL{
		{Scheme: "http", Host: "localhost:"},
		{Scheme: "http", Host: "192.168.1."},
	}
}

func newCORS(origins, methods, headers []string, debug bool) *cors.Cors {
	if len(methods) == 0 {
		// original default: http.MethodGet, http.MethodPost, http.MethodHead
		methods = []string{http.MethodGet, http.MethodPost, http.MethodDelete}
	}
	if len(headers) == 0 {
		// original default: "Origin", "Accept", "Content-Type", "X-Requested-With"
		headers = []string{"Origin", "Content-Type", "Authorization"}
	}

	options := cors.Options{
		AllowedOrigins:         nil,
		AllowOriginFunc:        allowOriginFunc(origins),
		AllowOriginRequestFunc: nil,
		AllowedMethods:         methods,
		AllowedHeaders:         headers,
		ExposedHeaders:         nil,
		MaxAge:                 3600 * 24, // https://developer.mozilla.org/docs/Web/HTTP/Headers/Access-Control-Max-Age
		AllowCredentials:       true,
		OptionsPassthrough:     false,
		OptionsSuccessStatus:   http.StatusNoContent,
		Debug:                  debug, // verbose logs
	}

	log.Security("CORS Methods:", options.AllowedMethods)
	log.Security("CORS Headers:", options.AllowedHeaders)
	log.Securityf("CORS Credentials=%v MaxAge=%v", options.AllowCredentials, options.MaxAge)

	return cors.New(options)
}

func allowOriginFunc(origins []string) func(string) bool {
	InsertSchema(origins)
	switch len(origins) {
	case 0:
		return allOrigins()
	case 1:
		return oneOrigin(origins[0])
	default:
		return multipleOriginPrefixes(origins)
	}
}

// InsertSchema inserts "http://" when HTTP schema is missing.
func InsertSchema(urls []string) {
	for i, u := range urls {
		if !strings.HasPrefix(u, "https://") &&
			!strings.HasPrefix(u, "http://") {
			urls[i] = "http://" + u
		}
	}
}

func allOrigins() func(string) bool {
	log.Security("CORS Allow all origins")
	return func(origin string) bool {
		return true
	}
}

func oneOrigin(allowedOrigin string) func(string) bool {
	log.Security("CORS Allow one origin:", allowedOrigin)
	return func(origin string) bool {
		if origin == allowedOrigin {
			return true
		}

		log.Security("CORS Refuse " + origin + " is not " + allowedOrigin)
		return false
	}
}

func multipleOriginPrefixes(addrPrefixes []string) func(origin string) bool {
	log.Security("CORS Allow origin prefixes:", addrPrefixes)

	return func(origin string) bool {
		for _, prefix := range addrPrefixes {
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}

		log.Security("CORS Refuse", origin, "without prefixes", addrPrefixes)
		return false
	}
}
