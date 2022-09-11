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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/teal-finance/emo"
	"github.com/teal-finance/garcon/gg"
	"github.com/teal-finance/incorruptible"
)

var log = emo.NewZone("garcon")

type Garcon struct {
	ServerName ServerName
	Writer     Writer
	docURL     string
	urls       []*url.URL
	origins    []string
	pprofPort  int
	devMode    bool
}

func (g Garcon) IsDevMode() bool { return g.devMode }

func New(opts ...Option) *Garcon {
	var g Garcon
	for _, opt := range opts {
		if opt != nil {
			opt(&g)
		}
	}

	StartPProfServer(g.pprofPort)

	// namespace fallback = retrieve it from first URL
	if g.ServerName == "" && len(g.urls) > 0 {
		g.ServerName = ExtractName(g.urls[0].String())
	}

	// set CORS origins
	if len(g.urls) == 0 {
		g.urls = DevOrigins()
	} else if g.devMode {
		g.urls = gg.AppendURLs(g.urls, DevOrigins()...)
	}
	g.origins = gg.OriginsFromURLs(g.urls)

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

type Option func(*Garcon)

func WithServerName(str string) Option {
	return func(g *Garcon) {
		g.ServerName = ExtractName(str)
	}
}

func WithDocURL(docURL string) Option {
	return func(g *Garcon) {
		g.docURL = docURL
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

	return func(g *Garcon) {
		g.devMode = devMode
	}
}

func WithPProf(port int) Option {
	return func(g *Garcon) {
		g.pprofPort = port
	}
}

func WithURLs(addresses ...string) Option {
	return func(g *Garcon) {
		g.urls = gg.ParseURLs(addresses)
	}
}

// ListenAndServe runs the HTTP server(s) in foreground.
// Optionally it also starts a metrics server in background (if export port > 0).
// The metrics server is for use with Prometheus or another compatible monitoring tool.
func ListenAndServe(server *http.Server) error {
	log.Print("Server listening on http://localhost" + server.Addr)

	err := server.ListenAndServe()

	_, port, e := net.SplitHostPort(server.Addr)
	if e == nil {
		log.Error("Install ncat and ss: sudo apt install ncat iproute2")
		log.Errorf("Try to listen port %v: sudo ncat -l %v", port, port)
		log.Errorf("Get the process using port %v: sudo ss -pan | grep %v", port, port)
	}

	return err
}

// Server returns a default http.Server ready to handle API endpoints, static web pages...
func Server(h http.Handler, port int, connState ...func(net.Conn, http.ConnState)) http.Server {
	if len(connState) == 0 {
		connState = []func(net.Conn, http.ConnState){nil}
	}

	return http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           h,
		TLSConfig:         nil,
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Minute, // Garcon.MiddlewareRateLimiter() delays responses, so people (attackers) who click frequently will wait longer.
		IdleTimeout:       time.Second,
		MaxHeaderBytes:    444, // 444 bytes should be enough
		TLSNextProto:      nil,
		ConnState:         connState[0],
		ErrorLog:          log.Default(),
		BaseContext:       nil,
		ConnContext:       nil,
	}
}

// TokenChecker is the common interface to Incorruptible and JWTChecker.
type TokenChecker interface {
	// Set is a middleware setting a cookie in the response when the request has no valid token.
	// Set searches the token in a cookie and in the first "Authorization" header.
	// Finally, Set stores the decoded token fields within the request context.
	Set(next http.Handler) http.Handler

	// Chk is a middleware accepting requests only if it has a valid cookie:
	// other requests are rejected with http.StatusUnauthorized.
	// Chk does not verify the "Authorization" header.
	// See also the Vet() function if the token should also be verified in the "Authorization" header.
	// Finally, Chk stores the decoded token fields within the request context.
	// In dev. mode, Chk accepts any request but does not store invalid tokens.
	Chk(next http.Handler) http.Handler

	// Vet is a middleware accepting accepting requests having a valid token
	// either in the cookie or in the first "Authorization" header:
	// other requests are rejected with http.StatusUnauthorized.
	// Vet also stores the decoded token in the request context.
	// In dev. mode, Vet accepts any request but does not store invalid tokens.
	Vet(next http.Handler) http.Handler

	// Cookie returns a default cookie to facilitate testing.
	Cookie(i int) *http.Cookie
}

// IncorruptibleChecker uses cookies based the fast and tiny Incorruptible token.
// IncorruptibleChecker requires g.WithURLs() to set the Cookie secure, domain and path.
func (g *Garcon) IncorruptibleChecker(secretKeyHex string, maxAge int, setIP bool) *incorruptible.Incorruptible {
	if len(secretKeyHex) != 32 {
		log.Panic("Want AES-128 key composed by 32 hexadecimal digits, but got", len(secretKeyHex), "digits")
	}
	key, err := hex.DecodeString(secretKeyHex)
	if err != nil {
		log.Panic("Cannot decode the 128-bit AES key, please provide 32 hexadecimal digits:", err)
	}

	return g.IncorruptibleCheckerBin(key, maxAge, setIP)
}

// IncorruptibleChecker uses cookies based the fast and tiny Incorruptible token.
// IncorruptibleChecker requires g.WithURLs() to set the Cookie secure, domain and path.
func (g *Garcon) IncorruptibleCheckerBin(secretKeyBin []byte, maxAge int, setIP bool) *incorruptible.Incorruptible {
	if len(secretKeyBin) != 16 {
		log.Panic("Want AES-128 key composed by 16 bytes, but got", len(secretKeyBin), "bytes")
	}

	if len(g.urls) == 0 {
		log.Panic("Missing URLs => Set first the URLs with garcon.WithURLs()")
	}

	cookieName := string(g.ServerName)
	return incorruptible.New(g.Writer.WriteErr, g.urls, secretKeyBin, cookieName, maxAge, setIP)
}

// JWTChecker requires WithURLs() to set the Cookie name, secure, domain and path.
func (g *Garcon) JWTChecker(secretKeyHex string, planPerm ...any) *JWTChecker {
	if len(g.urls) == 0 {
		log.Panic("Missing URLs => Set first the URLs with garcon.WithURLs()")
	}

	return NewJWTChecker(g.Writer, g.urls, secretKeyHex, planPerm...)
}
