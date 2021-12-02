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
	"github.com/teal-finance/garcon/jwtperm"
	"github.com/teal-finance/garcon/limiter"
	"github.com/teal-finance/garcon/metrics"
	"github.com/teal-finance/garcon/opa"
	"github.com/teal-finance/garcon/pprof"
	"github.com/teal-finance/garcon/reqlog"
	"github.com/teal-finance/garcon/reserr"
)

// DevOrigins provides the development origins:
// - yarn run vite --port 3000
// - yarn run vite preview --port 5000
// - localhost:8085 on multidevices: web autoreload using https://github.com/synw/fwr
// - flutter run --web-port=8080
// - 192.168.1.x + any port on tablet: mobile app using fast builtin autoreload.
var DevOrigins = []string{"http://localhost:", "http://192.168.1."}

type Garcon struct {
	ConnState      func(net.Conn, http.ConnState)
	JWT            *jwtperm.Checker
	ResErr         reserr.ResErr
	AllowedOrigins []string
	Middlewares    chain.Chain
	metrics        metrics.Metrics
}

type settings struct {
	origins      []string
	docURL       string
	nameVersion  string
	secretKey    string
	planPerm     []interface{}
	opaFilenames []string
	pprofPort    int
	expPort      int
	reqLogs      int
	reqBurst     int
	reqMinute    int
	devMode      bool
}

func New(opts ...Option) (*Garcon, error) {
	s := settings{
		origins:      DevOrigins,
		docURL:       "",
		nameVersion:  "",
		secretKey:    "",
		planPerm:     nil,
		opaFilenames: nil,
		pprofPort:    0,
		expPort:      0,
		reqLogs:      0,
		reqBurst:     0,
		reqMinute:    0,
		devMode:      false,
	}

	for _, opt := range opts {
		opt(&s)
	}

	pprof.StartServer(s.pprofPort)

	if s.origins == nil {
		s.origins = DevOrigins
	} else if s.devMode {
		s.origins = AppendPrefixes(s.origins, DevOrigins)
	}

	if len(s.docURL) > 0 &&
		!strings.HasPrefix(s.docURL, s.origins[0]) &&
		!strings.Contains(s.docURL, "://") {
		s.docURL = s.origins[0] + s.docURL
	}

	return s.new()
}

func (s settings) new() (*Garcon, error) {
	g := Garcon{
		AllowedOrigins: s.origins,
		ResErr:         reserr.New(s.docURL),
		metrics:        metrics.Metrics{},
		Middlewares:    nil,
		ConnState:      nil,
		JWT:            nil,
	}

	g.Middlewares, g.ConnState = g.metrics.StartServer(s.expPort, s.devMode)

	switch s.reqLogs {
	case 0:
		break // do not log incoming HTTP requests
	case 1:
		g.Middlewares = g.Middlewares.Append(reqlog.LogRequests)
	case 2:
		g.Middlewares = g.Middlewares.Append(reqlog.LogVerbose)
	}

	if s.reqMinute > 0 {
		reqLimiter := limiter.New(s.reqBurst, s.reqMinute, s.devMode, g.ResErr)
		g.Middlewares = g.Middlewares.Append(reqLimiter.Limit)
	}

	g.Middlewares = g.Middlewares.Append(
		ServerHeader(s.nameVersion),
		cors.Handler(g.AllowedOrigins, s.devMode),
	)

	g.JWT = g.NewJWTChecker(s.secretKey, s.planPerm...)

	// Authentication rules (Open Policy Agent)
	policy, err := opa.New(s.opaFilenames, g.ResErr)
	if err != nil {
		return &g, err
	}

	if policy.Ready() {
		g.Middlewares = g.Middlewares.Append(policy.Auth)
	}

	return &g, nil
}

type Option func(*settings)

func WithOrigins(origins ...string) Option {
	return func(s *settings) {
		s.origins = origins
	}
}

func WithDocURL(pathOrURL string) Option {
	return func(s *settings) {
		s.docURL = pathOrURL
	}
}

func WithServerHeader(nameVersion string) Option {
	return func(s *settings) {
		s.nameVersion = nameVersion
	}
}

func WithJWT(secretKey string, planPerm ...interface{}) Option {
	return func(s *settings) {
		s.secretKey = secretKey
		s.planPerm = planPerm
	}
}

func WithOPA(opaFilenames ...string) Option {
	return func(s *settings) {
		s.opaFilenames = opaFilenames
	}
}

func WithReqLogs(verbosity ...int) Option {
	v := 1

	if len(verbosity) > 0 {
		if len(verbosity) >= 2 {
			log.Panic("garcon.WithReqLogs() must be called with zero or one argument")
		}

		v = verbosity[0]
		if v < 0 || v > 2 {
			log.Panicf("garcon.WithReqLogs(verbosity=%v) currently accepts values [0, 1, 2] only", v)
		}
	}

	return func(s *settings) {
		s.reqLogs = v
	}
}

func WithLimiter(values ...int) Option {
	var burst, perMinute int

	switch len(values) {
	case 0:
		burst = 20
		perMinute = 4 * burst
	case 1:
		burst = values[0]
		perMinute = 4 * burst
	case 2:
		burst = values[0]
		perMinute = values[1]
	default:
		log.Panic("garcon.WithLimiter() must be called with less than three arguments")
	}

	return func(s *settings) {
		s.reqBurst = burst
		s.reqMinute = perMinute
	}
}

func WithPProf(port int) Option {
	return func(s *settings) {
		s.pprofPort = port
	}
}

func WithProm(port int) Option {
	return func(s *settings) {
		s.expPort = port
	}
}

func WithDev(enable ...bool) Option {
	devMode := true
	if len(enable) > 0 {
		devMode = enable[0]

		if len(enable) >= 2 {
			log.Panic("garcon.WithDev() must be called with zero or one argument")
		}
	}

	return func(s *settings) {
		s.devMode = devMode
	}
}

// RunServer runs the HTTP server(s) in foreground.
// Optionally it also starts a metrics server in background (if export port > 0).
// The metrics server is for use with Prometheus or another compatible monitoring tool.
func (g *Garcon) Run(h http.Handler, port int) error {
	// main garcon: REST API or other web servers
	server := http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           g.Middlewares.Then(h),
		TLSConfig:         nil,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       1 * time.Second,
		MaxHeaderBytes:    444, // 444 bytes should be enough
		TLSNextProto:      nil,
		ConnState:         g.ConnState,
		ErrorLog:          log.Default(),
		BaseContext:       nil,
		ConnContext:       nil,
	}

	log.Print("Server listening on http://localhost", server.Addr)

	err := server.ListenAndServe()

	log.Print("ERR: Install ncat and ss: sudo apt install ncat iproute2")
	log.Printf("ERR: Try to listen port %v: sudo ncat -l %v", port, port)
	log.Printf("ERR: Get the process using port %v: sudo ss -pan | grep %v", port, port)

	return err
}

func (g *Garcon) NewJWTChecker(secretKey string, planPerm ...interface{}) *jwtperm.Checker {
	return jwtperm.New(g.AllowedOrigins, g.ResErr, secretKey, planPerm...)
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

func isSeparator(c rune) bool {
	switch c {
	case ',', ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}

	return false
}

func AppendPrefixes(slice, prefixes []string) []string {
	for _, p := range prefixes {
		slice = insertPrefix(slice, p)
	}

	return slice
}

func insertPrefix(slice []string, p string) []string {
	for i, s := range slice {
		// check if `s` is a prefix of `p`
		if len(s) <= len(p) {
			if s == p[:len(s)] {
				return slice
			}
		} else // is `p` a prefix of `s`?  (preserve i==0)
		if i > 0 && s[:len(p)] == p {
			slice[i] = p // replace `s` by `p`

			return slice
		}
	}

	return append(slice, p)
}
