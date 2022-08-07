// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

// Package garcon is a server for API and static website
// including middlewares to manage rate-limit, Cookies, JWT,
// CORS, OPA, web traffic, Prometheus export and PProf.
package garcon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/incorruptible"
)

type Garcon struct {
	Namespace       Namespace
	Writer          Writer
	docURL          string
	version         string
	secretKey       []byte
	checkerCfg      []any
	opaFilenames    []string
	urls            []*url.URL
	origins         []string
	pprofPort       int
	expPort         int
	reqLogVerbosity int
	reqBurst        int
	reqMinute       int
	devMode         bool
}

func New(opts ...Option) *Garcon {
	var g Garcon
	for _, opt := range opts {
		if opt != nil {
			opt(&g)
		}
	}

	StartPProfServer(g.pprofPort)

	if g.urls == nil {
		g.urls = DevOrigins()
	} else if g.devMode {
		g.urls = AppendURLs(g.urls, DevOrigins()...)
	}
	g.origins = OriginsFromURLs(g.urls)

	if len(g.docURL) > 0 {
		// if docURL is just a path => complete it with the base URL (scheme + host)
		baseURL := g.urls[0].String()
		if !strings.HasPrefix(g.docURL, baseURL) &&
			!strings.Contains(g.docURL, "://") {
			g.docURL = baseURL + g.docURL
		}
	}
	g.Writer = NewWriter(g.docURL)

	return &g
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

// ListenAndServe runs the HTTP server(s) in foreground.
// Optionally it also starts a metrics server in background (if export port > 0).
// The metrics server is for use with Prometheus or another compatible monitoring tool.
func (g *Garcon) ListenAndServe(h http.Handler, port int) error {
	server := g.Server(h, port)

	log.Print("Server listening on http://localhost", server.Addr)

	err := server.ListenAndServe()

	log.Print("ERR: Install ncat and ss: sudo apt install ncat iproute2")
	log.Printf("ERR: Try to listen port %v: sudo ncat -l %v", port, port)
	log.Printf("ERR: Get the process using port %v: sudo ss -pan | grep %v", port, port)

	return err
}

// Server returns a default http.Server ready to handle API endpoints, static web pages...
func (g *Garcon) Server(h http.Handler, port int) http.Server {
	h, connState := g.Handler(h)

	return http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           h,
		TLSConfig:         nil,
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Minute, // Garcon.Limiter delays responses, so people (attackers) who click frequently will wait longer.
		IdleTimeout:       time.Second,
		MaxHeaderBytes:    444, // 444 bytes should be enough
		TLSNextProto:      nil,
		ConnState:         connState,
		ErrorLog:          log.Default(),
		BaseContext:       nil,
		ConnContext:       nil,
	}
}

func (g *Garcon) Handler(h http.Handler) (http.Handler, func(net.Conn, http.ConnState)) {
	chain, connState := g.ChainMiddleware()
	g.EraseSecretKey()
	return chain.Then(h), connState
}

func (g *Garcon) ChainMiddleware() (Chain, func(net.Conn, http.ConnState)) {
	chain, connState := g.StartMetricsServer()

	chain = chain.Append(RejectInvalidURI)

	reqLogger := g.RequestLogger()
	if reqLogger != nil {
		chain = chain.Append(reqLogger)
	}

	rateLimiter := g.RateLimiter()
	if rateLimiter != nil {
		chain = chain.Append(rateLimiter)
	}

	serverSetter := g.ServerSetter()
	if serverSetter != nil {
		chain = chain.Append(serverSetter)
	}

	cors := g.CORSHandler()
	if cors != nil {
		chain = chain.Append(cors)
	}

	opa := g.OPAHandler()
	if opa != nil {
		chain = chain.Append(opa)
	}

	return chain, connState
}

type TokenChecker interface {
	// Cookie returns a default cookie (make testing easier).
	Cookie(i int) *http.Cookie

	// Set sets a cookie in the response when the request has no valid token.
	// Set searches the token in a cookie and in the first "Authorization" header.
	// Finally, Set stores the token attributes in the request context.
	Set(next http.Handler) http.Handler

	// Chk accepts requests only if it has a valid cookie.
	// Chk does not verify the "Authorization" header.
	// See the Vet() function to also verify the "Authorization" header.
	// Chk also stores the token attributes in the request context.
	// In dev. testing, Chk accepts any request but does not store invalid tokens.
	Chk(next http.Handler) http.Handler

	// Vet accepts requests having a valid token either in
	// the cookie or in the first "Authorization" header.
	// Vet also stores the decoded token in the request context.
	// In dev. testing, Vet accepts any request but does not store invalid tokens.
	Vet(next http.Handler) http.Handler
}

func (g *Garcon) TokenChecker() TokenChecker {
	if len(g.secretKey) == 0 {
		return nil
	}

	m := g.NewIncorruptible()
	if m != nil {
		return m
	}

	return g.NewJWTChecker()
}

func (g *Garcon) NewIncorruptible() *incorruptible.Incorruptible {
	if len(g.checkerCfg) != 3 {
		return nil
	}

	name, ok1 := g.checkerCfg[0].(string)
	maxAge, ok2 := g.checkerCfg[0].(int)
	setIP, ok3 := g.checkerCfg[0].(bool)

	if name == "" {
		name = string(g.Namespace)
	}

	if ok1 && ok2 && ok3 {
		return incorruptible.New(name, g.urls, g.secretKey, maxAge, setIP, g.Writer.WriteErr)
	}

	return nil
}

func (g *Garcon) NewJWTChecker() *JWTChecker {
	return NewJWTChecker(g.urls, g.Writer, g.secretKey, g.checkerCfg...)
}

// OverwriteBufferContent is to erase secrets when they are no longer required.
func OverwriteBufferContent(b []byte) {
	//nolint:gosec // does not matter if written bytes are not good random values
	_, _ = rand.Read(b)
}

// EraseSecretKey should be called as soon as secretKey is no longer required.
func (g *Garcon) EraseSecretKey() {
	OverwriteBufferContent(g.secretKey)
	g.secretKey = nil
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

func appendOnePrefix(origins []string, prefix string) []string {
	for i, url := range origins {
		// if `url` is already a prefix of `prefix` => stop
		if len(url) <= len(prefix) {
			if url == prefix[:len(url)] {
				return origins
			}
			continue
		}

		// preserve origins[0]
		if i == 0 {
			continue
		}

		// if `prefix` is a prefix of `url` => update origins[i]
		if url[:len(prefix)] == prefix {
			origins[i] = prefix // replace `o` by `p`
			return origins
		}
	}

	return append(origins, prefix)
}

func AppendURLs(urls []*url.URL, prefixes ...*url.URL) []*url.URL {
	for _, p := range prefixes {
		urls = appendOneURL(urls, p)
	}
	return urls
}

func appendOneURL(urls []*url.URL, prefix *url.URL) []*url.URL {
	for i, url := range urls {
		if url.Scheme != prefix.Scheme {
			continue
		}

		// if `url` is already a prefix of `prefix` => stop
		if len(url.Host) <= len(prefix.Host) {
			if url.Host == prefix.Host[:len(url.Host)] {
				return urls
			}
			continue
		}

		// preserve urls[0]
		if i == 0 {
			continue
		}

		// if `prefix` is a prefix of `url` => update urls[i]
		if url.Host[:len(prefix.Host)] == prefix.Host {
			urls[i] = prefix // replace `u` by `prefix`
			return urls
		}
	}

	return append(urls, prefix)
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

var ErrNonPrintable = errors.New("non-printable")

// Value returns the /endpoint/{key} (URL path)
// else the "key" form (HTTP body)
// else the "key" query string (URL)
// else the HTTP header.
// Value requires chi.URLParam().
func Value(r *http.Request, key, header string) (string, error) {
	value := chi.URLParam(r, key)
	if value == "" {
		value = r.FormValue(key)
		if value == "" && header != "" {
			// Check only the first Header,
			// because we do not know how to manage several ones.
			value = r.Header.Get(header)
		}
	}

	if i := printable(value); i >= 0 {
		return value, fmt.Errorf("%s %w at %d", key, ErrNonPrintable, i)
	}
	return value, nil
}

// Values requires chi.URLParam().
func Values(r *http.Request, key string) ([]string, error) {
	form := r.Form[key]

	if i := Printable(form...); i >= 0 {
		return form, fmt.Errorf("%s %w at %d", key, ErrNonPrintable, i)
	}

	// no need to test v because Garcon already verifies the URI
	if v := chi.URLParam(r, key); v != "" {
		return append(form, v), nil
	}

	return form, nil
}

// DecodeJSONBody unmarshals the JSON from the request body.
func DecodeJSONBody[T json.Unmarshaler](r *http.Request, msg T) error {
	if r.Body == nil {
		return errors.New("empty body")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("cannot read body %w", err)
	}

	err = msg.UnmarshalJSON(body)
	if err != nil {
		return fmt.Errorf("bad JSON %w", err)
	}

	return nil
}
