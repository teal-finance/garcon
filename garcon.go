// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

// Package garcon is a server for API and static website
// including middlewares to manage rate-limit, Cookies, JWT,
// CORS, OPA, web traffic, Prometheus export and PProf.
package garcon

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/incorruptible"
	"github.com/teal-finance/incorruptible/dtoken"
)

// DevOrigins provides the development origins:
// - yarn run vite --port 3000
// - yarn run vite preview --port 5000
// - localhost:8085 on multi devices: web auto-reload using https://github.com/synw/fwr
// - flutter run --web-port=8080
// - 192.168.1.x + any port on tablet: mobile app using fast builtin auto-reload.
var DevOrigins = []*url.URL{
	{Scheme: "http", Host: "localhost:"},
	{Scheme: "http", Host: "192.168.1."},
}

type Garcon struct {
	ConnState      func(net.Conn, http.ConnState)
	Checker        TokenChecker
	ResErr         reserr.ResErr
	AllowedOrigins []string
	Middlewares    Chain
}

type TokenChecker interface {
	// Cookie returns the internal cookie (for test purpose).
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

type parameters struct {
	namespace    string
	docURL       string
	nameVersion  string
	secretKey    []byte
	planPerm     []any
	opaFilenames []string
	urls         []*url.URL
	pprofPort    int
	expPort      int
	reqLogs      int
	reqBurst     int
	reqMinute    int
	devMode      bool
}

func New(opts ...Option) (*Garcon, error) {
	var params parameters

	for _, opt := range opts {
		if opt != nil {
			opt(&params)
		}
	}

	StartPProfServer(params.pprofPort)

	if params.urls == nil {
		params.urls = DevOrigins
	} else if params.devMode {
		params.urls = AppendURLs(params.urls, DevOrigins...)
	}

	if len(params.docURL) > 0 {
		// if docURL is just a path => complete it with the base URL (scheme + host)
		baseURL := params.urls[0].String()
		if !strings.HasPrefix(params.docURL, baseURL) &&
			!strings.Contains(params.docURL, "://") {
			params.docURL = baseURL + params.docURL
		}
	}

	return params.new()
}

func (s *parameters) new() (*Garcon, error) {
	g := Garcon{
		ConnState:      nil,
		Checker:        nil,
		ResErr:         reserr.New(s.docURL),
		AllowedOrigins: OriginsFromURLs(s.urls),
		Middlewares:    nil,
	}

	g.Middlewares, g.ConnState = StartMetricsServer(s.expPort, s.namespace)

	g.Middlewares.Append(RejectInvalidURI)

	switch s.reqLogs {
	case 0:
		break // do not log incoming HTTP requests
	case 1:
		g.Middlewares = g.Middlewares.Append(LogRequest)
	case 2:
		g.Middlewares = g.Middlewares.Append(LogRequestFingerprint)
	}

	if s.reqMinute > 0 {
		reqLimiter := NewReqLimiter(s.reqBurst, s.reqMinute, s.devMode, g.ResErr)
		g.Middlewares = g.Middlewares.Append(reqLimiter.LimitRate)
	}

	g.Middlewares = g.Middlewares.Append(
		ServerHeader(s.nameVersion),
		CORSHandler(g.AllowedOrigins, s.devMode),
	)

	if len(s.secretKey) > 0 {
		if len(s.planPerm) == 1 {
			dt, ok := s.planPerm[0].(dtoken.DToken)
			if ok {
				setIP := (dt.IP != nil)
				g.Checker = g.NewSessionToken(s.urls, s.secretKey, time.Duration(dt.Expiry), setIP)
			}
		}
		if g.Checker == nil {
			g.Checker = g.NewJWTChecker(s.urls, s.secretKey, s.planPerm...)
		}
		// erase the secret, no longer required
		for i := range s.secretKey {
			s.secretKey[i] = byte(rand.Intn(256))
		}
		s.secretKey = nil
	}

	// Authentication rules (Open Policy Agent)
	if len(s.opaFilenames) > 0 {
		policy, err := NewPolicy(s.opaFilenames, g.ResErr)
		if err != nil {
			return &g, err
		}
		g.Middlewares = g.Middlewares.Append(policy.AuthOPA)
	}

	return &g, nil
}

type Option func(*parameters)

func WithURLs(addresses ...string) Option {
	return func(params *parameters) {
		params.urls = ParseURLs(addresses)
	}
}

func WithDocURL(docURL string) Option {
	return func(params *parameters) {
		params.docURL = docURL
	}
}

func WithServerHeader(nameVersion string) Option {
	return func(params *parameters) {
		params.nameVersion = nameVersion
	}
}

// WithJWT requires WithURLs() to set the Cookie name, secure, domain and path.
// WithJWT is not compatible with WithTkn: use only one of them.
func WithJWT(secretKeyHex string, planPerm ...any) Option {
	key, err := hex.DecodeString(secretKeyHex)
	if err != nil {
		log.Panic("WithJWT: cannot decode the HMAC-SHA256 key, please provide hexadecimal format (64 characters) ", err)
	}
	if len(key) != 32 {
		log.Panic("WithJWT: want HMAC-SHA256 key containing 32 bytes, but got ", len(key))
	}

	return func(params *parameters) {
		params.secretKey = key
		params.planPerm = planPerm
	}
}

// WithIncorruptible enables the "session" cookies based on fast and tiny token.
// WithIncorruptible requires WithURLs() to set the Cookie name, secure, domain and path.
// WithIncorruptible is not compatible with WithJWT: use only one of them.
func WithIncorruptible(secretKeyHex string, expiry time.Duration, setIP bool) Option {
	key, err := hex.DecodeString(secretKeyHex)
	if err != nil {
		log.Panic("WithIncorruptible: cannot decode the 128-bit AES key, please provide hexadecimal format (32 characters)")
	}
	if len(key) < 16 {
		log.Panic("WithIncorruptible: want 128-bit AES key containing 16 bytes, but got ", len(key))
	}

	return func(params *parameters) {
		params.secretKey = key
		// ugly trick to store parameters for the "incorruptible" token
		var ip net.IP
		if setIP {
			ip = []byte{}
		}
		params.planPerm = []any{dtoken.DToken{
			Expiry: expiry.Nanoseconds(),
			IP:     ip,
			Values: nil,
		}}
	}
}

func WithOPA(opaFilenames ...string) Option {
	return func(params *parameters) {
		params.opaFilenames = opaFilenames
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

	return func(params *parameters) {
		params.reqLogs = v
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

	return func(params *parameters) {
		params.reqBurst = burst
		params.reqMinute = perMinute
	}
}

func WithPProf(port int) Option {
	return func(params *parameters) {
		params.pprofPort = port
	}
}

func WithProm(port int, namespace string) Option {
	return func(params *parameters) {
		params.expPort = port

		// If namespace is a path or an URL, keep the last basename.
		// Example: keep "myapp" from "https://example.com/path/myapp/"
		namespace = strings.Trim(namespace, "/")
		if i := strings.LastIndex(namespace, "/"); i >= 0 {
			namespace = namespace[i+1:]
		}

		// valid namespace = [a-zA-Z][a-zA-Z0-9_]*
		// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
		re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
		namespace = re.ReplaceAllLiteralString(namespace, "")
		if !unicode.IsLetter(rune(namespace[0])) {
			namespace = "a" + namespace
		}

		params.namespace = re.ReplaceAllLiteralString(namespace, "")
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

	return func(params *parameters) {
		params.devMode = devMode
	}
}

// Run runs the HTTP server(s) in foreground.
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

func (g *Garcon) NewSessionToken(urls []*url.URL, secretKey []byte, expiry time.Duration, setIP bool) *incorruptible.Incorruptible {
	return incorruptible.New(urls, secretKey, expiry, setIP, g.ResErr.Write)
}

func (g *Garcon) NewJWTChecker(urls []*url.URL, secretKey []byte, planPerm ...any) *Checker {
	return NewChecker(urls, g.ResErr, secretKey, planPerm...)
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

var ErrNonPrintable = errors.New("non-printable")

// Value returns the /endpoint/{key} (URL path)
// else the "key" form (HTTP body)
// else the "key" query string (URL)
// else the HTTP header.
func Value(r *http.Request, key, header string) (string, error) {
	v := chi.URLParam(r, key)

	if v == "" {
		v = r.FormValue(key)
	}

	if v == "" {
		v = r.Header.Get(header)
	}

	if i := Printable(v); i >= 0 {
		return v, fmt.Errorf("%s %w at %d", key, ErrNonPrintable, i)
	}

	return v, nil
}

func Values(r *http.Request, key string) ([]string, error) {
	form := r.Form[key]

	if i := Printables(form); i >= 0 {
		return form, fmt.Errorf("%s %w at %d", key, ErrNonPrintable, i)
	}

	// no need to test v because Garcon already verifies the URI
	if v := chi.URLParam(r, key); v != "" {
		return append(form, v), nil
	}

	return form, nil
}
