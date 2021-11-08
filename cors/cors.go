// Teal.Finance/Garcon is an opinionated boilerplate API and website server.
// Copyright (C) 2021 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon, licensed under LGPL-3.0-or-later.
//
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 of the License, or (at your option) any later version.
//
// Teal.Finance/Garcon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.

package cors

import (
	"log"
	"net/http"
	"strings"

	"github.com/rs/cors"
)

// Handler uses restrictive CORS values.
func Handler(origins []string, debug bool) func(next http.Handler) http.Handler {
	options := cors.Options{
		AllowedOrigins:         []string{},
		AllowOriginFunc:        nil,
		AllowOriginRequestFunc: nil,
		AllowedMethods:         []string{http.MethodGet},
		AllowedHeaders:         []string{"Origin", "Accept", "Content-Type", "Authorization", "Cookie"},
		ExposedHeaders:         []string{},
		MaxAge:                 60,
		AllowCredentials:       true,
		OptionsPassthrough:     false,
		Debug:                  debug, // verbose logs
	} // https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age

	InsertSchema(origins)

	if len(origins) == 1 {
		options.AllowOriginFunc = oneOrigin(origins[0])
	} else {
		options.AllowOriginFunc = multipleOriginPrefixes(origins)
	}

	log.Printf("Middleware CORS: %+v", options)

	return cors.New(options).Handler
}

func InsertSchema(domains []string) {
	for i, dns := range domains {
		if !strings.HasPrefix(dns, "https://") &&
			!strings.HasPrefix(dns, "http://") {
			domains[i] = "http://" + dns
		}
	}
}

func oneOrigin(addr string) func(origin string) bool {
	log.Print("CORS: Set one origin: ", addr)

	return func(origin string) bool {
		return origin == addr
	}
}

func multipleOriginPrefixes(addrPrefixes []string) func(origin string) bool {
	log.Print("CORS: Set origin prefixes: ", addrPrefixes)

	return func(origin string) bool {
		for _, prefix := range addrPrefixes {
			if strings.HasPrefix(origin, prefix) {
				log.Printf("CORS: Accept %v because starts with prefix %v", origin, prefix)

				return true
			}

			log.Printf("CORS: %v does not begin with %v", origin, prefix)
		}

		log.Printf("CORS: Refuse %v because different from %v", origin, addrPrefixes)

		return false
	}
}
