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

package garcon

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/teal-finance/garcon/chain"
	"github.com/teal-finance/garcon/cors"
	"github.com/teal-finance/garcon/limiter"
	"github.com/teal-finance/garcon/metrics"
	"github.com/teal-finance/garcon/opa"
	"github.com/teal-finance/garcon/pprof"
	"github.com/teal-finance/garcon/reserr"
)

type Garcon struct {
	Version string
	ResErr  reserr.ResErr

	// CORS
	AllowedOrigins []string // used for CORS

	// OPA
	OPAFilenames []string

	metrics metrics.Metrics
}

// DevOrigins provides the development origins:
//  - yarn run vite --port 3000
//  - yarn run vite preview --port 5000
//  - localhost:8085 on multidevices: web autoreload using https://github.com/synw/fwr
//  - flutter run --web-port=8080
//  - 192.168.1.x + any port on tablet: mobile app using fast builtin autoreload
var DevOrigins = []string{"http://localhost:", "http://192.168.1."}

func (s *Garcon) Setup(pprofPort, expPort, reqBurst, reqMinute int, devMode bool) (chain.Chain, func(net.Conn, http.ConnState), error) {
	pprof.StartServer(pprofPort)

	middlewares, connState := s.metrics.StartServer(expPort, devMode)

	if reqMinute == 0 {
		middlewares = middlewares.Append(LogRequests)
	} else {
		reqLimiter := limiter.New(reqBurst, reqMinute, devMode, s.ResErr)
		middlewares = middlewares.Append(reqLimiter.Limit)
	}

	if devMode {
		s.AllowedOrigins = append(s.AllowedOrigins, DevOrigins...)
	}

	middlewares = middlewares.Append(
		ServerHeader(s.Version),
		cors.Handler(s.AllowedOrigins, devMode),
	)

	// Endpoint authentication rules (Open Policy Agent)
	policy, err := opa.New(s.OPAFilenames, s.ResErr)
	if err != nil {
		return middlewares, connState, err
	}

	if policy.Ready() {
		middlewares = middlewares.Append(policy.Auth)
	}

	return middlewares, connState, nil
}

// RunServer runs the HTTP server(s) in foreground.
// Optionally it also starts a metrics server in background (if export port > 0).
// The metrics server is for use with Prometheus or another compatible monitoring tool.
func (s *Garcon) Run(h http.Handler, port, pprofPort, expPort, reqBurst, reqMinute int, devMode bool) error {
	middlewares, connState, err := s.Setup(pprofPort, expPort, reqBurst, reqMinute, devMode)
	if err != nil {
		return err
	}

	// main garcon: REST API or other web servers
	server := http.Server{
		Addr:              ":" + strconv.Itoa(port),
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

	log.Print("Server listening on http://localhost", server.Addr)

	err = server.ListenAndServe()

	log.Print("ERROR: Install ncat and ss: sudo apt install ncat iproute2")
	log.Printf("ERROR: Try to listen port %v: sudo ncat -l %v", port, port)
	log.Printf("ERROR: Get the process using port %v: sudo ss -pan | grep %v", port, port)

	return err
}

// ServerHeader sets the Server HTTP header in the response.
func ServerHeader(version string) func(next http.Handler) http.Handler {
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
			log.Printf("in  %v %v %v", r.Method, r.RemoteAddr, r.RequestURI)
			next.ServeHTTP(w, r)
		})
}

func isSeparator(c rune) bool {
	switch c {
	case ',', '\t', '\n', '\v', '\f', '\r':
		return true
	}

	return false
}

// SplitClean splits the values and trim them.
func SplitClean(values string) []string {
	splitValues := strings.FieldsFunc(values, isSeparator)

	cleanValues := make([]string, 0, len(splitValues))

	for _, v := range splitValues {
		v = strings.TrimSpace(v)
		if v != "" {
			cleanValues = append(cleanValues, v)
		}
	}

	return cleanValues
}
