// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"log"
	"net/http"
	"strings"

	"github.com/rs/cors"
)

// MiddlewareCORS is a middleware to handle Cross-Origin Resource Sharing (CORS).
func (g *Garcon) MiddlewareCORS() Middleware {
	if len(g.origins) == 0 {
		log.Panic("Missing Origins: Please call garcon.WithURLs() before using MiddlewareCORS()")
	}
	return MiddlewareCORS(g.origins, g.devMode)
}

// MiddlewareCORS uses restrictive CORS values.
func MiddlewareCORS(origins []string, debug bool) func(next http.Handler) http.Handler {
	options := cors.Options{
		AllowedOrigins:         nil,
		AllowOriginFunc:        nil,
		AllowOriginRequestFunc: nil,
		AllowedMethods:         []string{http.MethodGet, http.MethodPost},
		AllowedHeaders:         []string{"Origin", "Accept", "Content-Type", "Authorization", "Cookie"},
		ExposedHeaders:         nil,
		MaxAge:                 3600 * 24, // https://developer.mozilla.org/docs/Web/HTTP/Headers/Access-Control-Max-Age
		AllowCredentials:       true,
		OptionsPassthrough:     false,
		OptionsSuccessStatus:   http.StatusNoContent,
		Debug:                  debug, // verbose logs
	}

	InsertSchema(origins)

	if len(origins) == 1 {
		options.AllowOriginFunc = oneOrigin(origins[0])
	} else {
		options.AllowOriginFunc = multipleOriginPrefixes(origins)
	}

	log.Printf("INF CORS: Methods=%v Headers=%v Credentials=%v MaxAge=%v",
		options.AllowedMethods, options.AllowedHeaders, options.AllowCredentials, options.MaxAge)

	return cors.New(options).Handler
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

func oneOrigin(addr string) func(string) bool {
	log.Print("INF CORS: Set one origin: ", addr)
	return func(origin string) bool {
		return origin == addr
	}
}

func multipleOriginPrefixes(addrPrefixes []string) func(origin string) bool {
	log.Print("INF CORS: Set origin prefixes: ", addrPrefixes)

	return func(origin string) bool {
		for _, prefix := range addrPrefixes {
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}

		log.Print("INF CORS: Refuse ", origin, " without prefixes ", addrPrefixes)
		return false
	}
}
