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

package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/teal-finance/server/cors"
	"github.com/teal-finance/server/limiter"
	"github.com/teal-finance/server/metrics"
	"github.com/teal-finance/server/opa"
	"github.com/teal-finance/server/reserr"
)

type Server struct {
	Version string
	ResErr  reserr.ResErr

	// CORS
	AllowedOrigins []string // used for CORS

	// OPA
	OPAFilenames []string

	metrics metrics.Metrics
}

// RunServer runs the HTTP server in foreground.
// Optionally it also starts a metrics server in background (if export port > 0).
// The metrics server is for use with Prometheus or another compatible monitoring tool.
func (s *Server) RunServer(h http.Handler, port, expPort, maxReqBurst, maxReqPerMinute int, devMode bool) error {
	middlewares, connState := s.metrics.StartServer(expPort, devMode)

	reqLimiter := limiter.New(maxReqBurst, maxReqPerMinute, devMode, s.ResErr)

	middlewares = middlewares.Append(LogRequests, reqLimiter.Limit, Header(s.Version))

	if len(s.OPAFilenames) > 0 {
		compiler, err := opa.Load(s.OPAFilenames)
		if err != nil {
			return err
		}

		policy := opa.Policy{Compiler: compiler, ResErr: s.ResErr}
		middlewares = middlewares.Append(policy.Auth)
	}

	middlewares = middlewares.Append(cors.HandleCORS(s.AllowedOrigins))

	addr := ":" + strconv.Itoa(port)

	log.Print("HTTP server listening on http://localhost", addr)

	// main server: REST API or any HTTP web server
	server := http.Server{
		Addr:              addr,
		Handler:           middlewares.Then(h),
		TLSConfig:         nil,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       1 * time.Second,
		MaxHeaderBytes:    444, // 444 bytes should be enough
		TLSNextProto:      nil,
		ConnState:         connState,
		ErrorLog:          log.Default(),
		BaseContext:       nil,
		ConnContext:       nil,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Print("ERROR: Install ncat and ss: sudo apt install ncat iproute2")
		log.Printf("ERROR: Try to listen port %v: sudo ncat -l %v", port, port)
		log.Printf("ERROR: Get the process using port %v: sudo ss -pan | grep %v", port, port)

		return err
	}

	return nil
}

// Header sets the Server HTTP header in the response.
func Header(version string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		log.Print("Middleware response HTTP header: Set Server ", version)

		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Server", version)
				next.ServeHTTP(w, r)
			})
	}
}

// LogRequests logs the incoming HTTP requests.
func LogRequests(next http.Handler) http.Handler {
	log.Print("Middleware logger: log requested URLs and remote addresses")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Printf("in  %v %v %v", r.Method, r.RequestURI, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
}
