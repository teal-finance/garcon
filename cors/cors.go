// Teal.Finance/Server is an opinionated complete HTTP server.
// Copyright (C) 2021 Teal.Finance contributors
//
// This file is part of Teal.Finance/Server, licensed under LGPL-3.0-or-later.
//
// Teal.Finance/Server is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 of the License, or (at your option) any later version.
//
// Teal.Finance/Server is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.

package cors

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// HandleCORS uses restrictive CORS values.
func HandleCORS(origins []string) func(next http.Handler) http.Handler {
	options := cors.Options{
		AllowedOrigins:     []string{},               // No need because use function
		AllowOriginFunc:    nil,                      // Function is set below
		AllowedMethods:     []string{http.MethodGet}, // The most restrictive
		AllowedHeaders:     []string{"Origin", "Accept", "Content-Type", "Authorization", "Cookie"},
		ExposedHeaders:     []string{},
		AllowCredentials:   true,
		OptionsPassthrough: false, // false = this middleware stops OPTION requests
		Debug:              true,
		MaxAge:             60, // Cache the response for 1 minute
	} // https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age

	for i, dns := range origins {
		origins[i] = insertSchema(dns)
	}

	if len(origins) == 1 {
		options.AllowOriginFunc = oneOrigin(origins[0])
	} else {
		options.AllowOriginFunc = multipleOriginPrefixes(origins)
	}

	log.Printf("Middleware CORS: %+v", options)

	return cors.Handler(options)
}

func insertSchema(domain string) string {
	if !strings.HasPrefix(domain, "https://") &&
		!strings.HasPrefix(domain, "http://") {
		return "http://" + domain
	}

	return domain
}

func oneOrigin(addr string) func(r *http.Request, origin string) bool {
	log.Print("CORS: Set origin: ", addr)

	return func(r *http.Request, origin string) bool {
		return origin == addr
	}
}

func multipleOriginPrefixes(prefixes []string) func(r *http.Request, origin string) bool {
	log.Print("CORS: Set origin prefixes: ", prefixes)

	return func(r *http.Request, origin string) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(origin, prefix) {
				log.Printf("CORS: Accept %v because starts with prefix %v", origin, prefix)

				return true
			}

			log.Printf("CORS: %v does not begin with %v", origin, prefix)
		}

		log.Printf("CORS: Refuse %v because different from %v", origin, prefixes)

		return false
	}
}
