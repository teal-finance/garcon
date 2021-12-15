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
	"net/url"
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
var DevOrigins = []*url.URL{
	{Scheme: "http", Host: "localhost:"},
	{Scheme: "http", Host: "192.168.1."},
}

type Garcon struct {
	ConnState      func(net.Conn, http.ConnState)
	JWT            *jwtperm.Checker
	ResErr         reserr.ResErr
	AllowedOrigins []string
	Middlewares    chain.Chain
	metrics        metrics.Metrics
}

type parameters struct {
	urls         []*url.URL
	docURL       string
	nameVersion  string
	secretKey    []byte
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
	p := parameters{
		urls:         nil,
		docURL:       "",
		nameVersion:  "",
		secretKey:    nil,
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
		opt(&p)
	}

	pprof.StartServer(p.pprofPort)

	if p.urls == nil {
		p.urls = DevOrigins
	} else if p.devMode {
		p.urls = AppendURLs(p.urls, DevOrigins...)
	}

	if len(p.docURL) > 0 {
		// if docURL is just a path => complet it with the base URL (scheme + host)
		baseURL := p.urls[0].String()
		if !strings.HasPrefix(p.docURL, baseURL) &&
			!strings.Contains(p.docURL, "://") {
			p.docURL = baseURL + p.docURL
		}
	}

	return p.new()
}

func (s parameters) new() (*Garcon, error) {
	g := Garcon{
		AllowedOrigins: OriginsFromURLs(s.urls),
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

	g.JWT = g.NewJWTChecker(s.urls, s.secretKey, s.planPerm...)

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

type Option func(*parameters)

func WithURLs(urls ...string) Option {
	return func(p *parameters) {
		p.urls = ParseURLs(urls)
	}
}

func WithDocURL(pathOrURL string) Option {
	return func(p *parameters) {
		p.docURL = pathOrURL
	}
}

func WithServerHeader(nameVersion string) Option {
	return func(p *parameters) {
		p.nameVersion = nameVersion
	}
}

// WithJWT requires WithURLs() to set the Cookie name, secure, domain and path.
func WithJWT(secretKey []byte, planPerm ...interface{}) Option {
	return func(p *parameters) {
		p.secretKey = secretKey
		p.planPerm = planPerm
	}
}

func WithOPA(opaFilenames ...string) Option {
	return func(p *parameters) {
		p.opaFilenames = opaFilenames
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

	return func(p *parameters) {
		p.reqLogs = v
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

	return func(p *parameters) {
		p.reqBurst = burst
		p.reqMinute = perMinute
	}
}

func WithPProf(port int) Option {
	return func(p *parameters) {
		p.pprofPort = port
	}
}

func WithProm(port int) Option {
	return func(p *parameters) {
		p.expPort = port
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

	return func(p *parameters) {
		p.devMode = devMode
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

func (g *Garcon) NewJWTChecker(urls []*url.URL, secretKey []byte, planPerm ...interface{}) *jwtperm.Checker {
	return jwtperm.New(urls, g.ResErr, secretKey, planPerm...)
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

func AppendPrefixes(origins []string, prefixes ...string) []string {
	for _, p := range prefixes {
		origins = appendOnePrefix(origins, p)
	}

	return origins
}

func appendOnePrefix(origins []string, p string) []string {
	for i, o := range origins {
		// if `o` is already a prefix of `p` => stop
		if len(o) <= len(p) {
			if o == p[:len(o)] {
				return origins
			}

			continue
		}

		// preserve origins[0]
		if i == 0 {
			continue
		}

		// if `p` a prefix of `o` => update origins[i]
		if o[:len(p)] == p {
			origins[i] = p // replace `o` by `p`

			return origins
		}
	}

	return append(origins, p)
}

func AppendURLs(urls []*url.URL, prefixes ...*url.URL) []*url.URL {
	for _, p := range prefixes {
		urls = appendOneURL(urls, p)
	}

	return urls
}

func appendOneURL(urls []*url.URL, p *url.URL) []*url.URL {
	for i, u := range urls {
		if u.Scheme != p.Scheme {
			continue
		}

		// if `u` is already a prefix of `p` => stop
		if len(u.Host) <= len(p.Host) {
			if u.Host == p.Host[:len(u.Host)] {
				return urls
			}

			continue
		}

		// preserve urls[0]
		if i == 0 {
			continue
		}

		// if `p` a prefix of `u` => update urls[i]
		if u.Host[:len(p.Host)] == p.Host {
			urls[i] = p // replace `u` by `p`

			return urls
		}
	}

	return append(urls, p)
}

func ParseURLs(origins []string) []*url.URL {
	urls := make([]*url.URL, 0, len(origins))

	for _, o := range origins {
		u, err := url.ParseRequestURI(o) // strip #fragment
		if err != nil {
			log.Panic("WithURLs: ", err)
		}

		if u.Host == "" {
			log.Panic("WithURLs: missing host in ", o)
		}

		urls = append(urls, u)
	}

	return urls
}

func OriginsFromURLs(urls []*url.URL) []string {
	origins := make([]string, 0, len(urls))

	for _, u := range urls {
		o := u.Scheme + "://" + u.Host
		origins = append(origins, o)
	}

	return origins
}
