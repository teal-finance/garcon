// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/incorruptible"
	"github.com/teal-finance/incorruptible/dtoken"
)

// DevOrigins provides the development origins:
// - yarn run vite --port 3000
// - yarn run vite preview --port 5000
// - localhost:8085 on multi devices: web auto-reload using https://github.com/synw/fwr
// - flutter run --web-port=8080
// - 192.168.1.x + any port on tablet: mobile app using fast builtin auto-reload.
//nolint:gochecknoglobals // used as const
var DevOrigins = []*url.URL{
	{Scheme: "http", Host: "localhost:"},
	{Scheme: "http", Host: "192.168.1."},
}

// Garcon is the main struct of the package garcon.
type Garcon struct {
	Namespace      Namespace
	ConnState      func(net.Conn, http.ConnState)
	Checker        TokenChecker
	ErrWriter      ErrWriter
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
	namespace       Namespace
	docURL          string
	version         string
	secretKey       []byte
	planPerm        []any
	opaFilenames    []string
	urls            []*url.URL
	pprofPort       int
	expPort         int
	reqLogVerbosity int
	reqBurst        int
	reqMinute       int
	devMode         bool
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

func (params *parameters) new() (*Garcon, error) {
	g := Garcon{
		Namespace:      params.namespace,
		ConnState:      nil,
		Checker:        nil,
		ErrWriter:      NewErrWriter(params.docURL),
		AllowedOrigins: OriginsFromURLs(params.urls),
		Middlewares:    nil,
	}

	g.Middlewares, g.ConnState = StartMetricsServer(params.expPort, params.namespace)

	g.Middlewares.Append(RejectInvalidURI)

	switch params.reqLogVerbosity {
	case 0:
		break // do not log incoming HTTP requests
	case 1:
		g.Middlewares = g.Middlewares.Append(LogRequest)
	case 2:
		g.Middlewares = g.Middlewares.Append(LogRequestFingerprint)
	}

	if params.reqMinute > 0 {
		reqLimiter := NewReqLimiter(params.reqBurst, params.reqMinute, params.devMode, g.ErrWriter)
		g.Middlewares = g.Middlewares.Append(reqLimiter.LimitRate)
	}

	if params.version != "" {
		g.Middlewares = g.Middlewares.Append(ServerHeader(params.version))
	}

	if len(g.AllowedOrigins) > 0 {
		g.Middlewares = g.Middlewares.Append(CORSHandler(g.AllowedOrigins, params.devMode))
	}

	g.setChecker(params)

	// Authentication rules (Open Policy Agent)
	if len(params.opaFilenames) > 0 {
		policy, err := NewPolicy(params.opaFilenames, g.ErrWriter)
		if err != nil {
			return &g, err
		}
		g.Middlewares = g.Middlewares.Append(policy.AuthOPA)
	}

	return &g, nil
}

func (g *Garcon) setChecker(params *parameters) {
	if len(params.secretKey) > 0 {
		if len(params.planPerm) == 1 {
			dt, ok := params.planPerm[0].(dtoken.DToken)
			if ok {
				setIP := (dt.IP != nil)
				g.Checker = g.NewSessionToken(params.urls, params.secretKey, time.Duration(dt.Expiry), setIP)
			}
		}
		if g.Checker == nil {
			g.Checker = g.NewJWTChecker(params.urls, params.secretKey, params.planPerm...)
		}
		// erase the secret, no longer required
		for i := range params.secretKey {
			params.secretKey[i] = 0
		}
		params.secretKey = nil
	}
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

func WithServerHeader(program string) Option {
	return func(params *parameters) {
		params.version = Version(program)
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

	return func(params *parameters) { params.reqLogVerbosity = v }
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

func WithNamespace(namespace string) Option {
	return func(params *parameters) {
		oldNS := params.namespace
		newNS := NewNamespace(namespace)
		if oldNS != "" && params.namespace != newNS {
			log.Panicf("WithProm(namespace=%q) overrides namespace=%q", newNS, oldNS)
		}
		params.namespace = newNS
	}
}

func WithProm(port int, namespace string) Option {
	setNamespace := WithNamespace(namespace)

	return func(params *parameters) {
		params.expPort = port
		setNamespace(params)
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
	return incorruptible.New(urls, secretKey, expiry, setIP, g.ErrWriter.Write)
}

func (g *Garcon) NewJWTChecker(urls []*url.URL, secretKey []byte, planPerm ...any) *JWTChecker {
	return NewJWTChecker(urls, g.ErrWriter, secretKey, planPerm...)
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
func Value(r *http.Request, key, header string) (string, error) {
	value := chi.URLParam(r, key)
	if value == "" {
		value = r.FormValue(key)
		if value == "" {
			// Check only the first Header,
			// because we do not know how to manage several ones.
			value = r.Header.Get(header)
		}
	}
	if i := Printable(value); i >= 0 {
		return value, fmt.Errorf("%s %w at %d", key, ErrNonPrintable, i)
	}
	return value, nil
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
