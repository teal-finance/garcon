// SPDX-License-Identifier: CC0-1.0
// Creative Commons Zero v1.0 Universal - No Rights Reserved
// <https://creativecommons.org/publicdomain/zero/1.0>
//
// To the extent possible under law, the Teal.Finance/Garcon contributors
// have waived all copyright and related/neighboring rights to this
// file "low-level/main.go" to be freely used without any restriction.

package main

import (
	"flag"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/emo"
	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/gg"
)

var log = emo.NewZone("app")

// Garcon settings
const (
	apiDoc            = "https://my-dns.co/myapp/doc"
	allowedProdOrigin = "https://my-dns.co"
	allowedDevOrigins = "http://localhost:  http://192.168.1."
	serverHeader      = "MyBackendName-1.2.0"
	burst, reqMinute  = 10, 30
	// the HMAC-SHA256 key to decode JWT (to be removed from source code)
	hmacSHA256Hex = "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
	// authCfg is deprecated
	// authCfg = "examples/sample-auth.rego"
)

var (
	pprofPort = gg.EnvInt("PPROF_PORT", 8095)
	expPort   = gg.EnvInt("EXP_PORT", 9095)
)

func main() {
	// the following line collects the CPU-profile and writes it in the file "cpu.pprof"
	defer garcon.ProbeCPU().Stop()

	garcon.StartPProfServer(pprofPort)

	// Uniformize error responses with API doc
	gw := garcon.NewWriter(apiDoc)

	middleware, connState, urls := setMiddlewares(gw)

	// Handles both REST API and static web files
	h := handler(gw, garcon.NewJWTChecker(gw, urls, hmacSHA256Hex, "my-cookie"))
	h = middleware.Then(h)

	runServer(h, connState)
}

func setMiddlewares(gw garcon.Writer) (_ gg.Chain, connState func(net.Conn, http.ConnState), _ []*url.URL) {
	// authCfg is deprecated
	// auth := flag.Bool("auth", false, "Enable OPA authorization specified in file "+authCfg)

	dev := flag.Bool("dev", true, "Use development or production settings")
	flag.Parse()

	// Start a metrics server in background if export port > 0.
	// The metrics server is for use with Prometheus or another compatible monitoring tool.
	middleware, connState := garcon.StartMetricsServer(expPort, "LowLevel")

	// Limit the input request rate per IP
	reqLimiter := garcon.NewRateLimiter(gw, burst, reqMinute, *dev)

	corsConfig := allowedProdOrigin
	if *dev {
		corsConfig = allowedDevOrigins
	}

	allowedOrigins := gg.SplitClean(corsConfig)
	urls := gg.ParseURLs(allowedOrigins)

	middleware = middleware.Append(
		reqLimiter.MiddlewareRateLimiter,
		garcon.MiddlewareServerHeader(serverHeader),
		garcon.MiddlewareCORS(allowedOrigins, nil, nil, *dev),
	)

	// authCfg is deprecated - // Endpoint authentication rules (Open Policy Agent)
	// authCfg is deprecated - if *auth {
	// authCfg is deprecated - 	files := garcon.SplitClean(authCfg)
	// authCfg is deprecated - 	policy, err := garcon.NewPolicy(gw, files)
	// authCfg is deprecated - 	if err != nil {
	// authCfg is deprecated - 		log.Fatal(err)
	// authCfg is deprecated - 	}
	// authCfg is deprecated - 	middleware = middleware.Append(policy.MiddlewareOPA)
	// authCfg is deprecated - }

	return middleware, connState, urls
}

// runServer runs in foreground the main server.
func runServer(h http.Handler, connState func(net.Conn, http.ConnState)) {
	const mainPort = "8080"

	server := http.Server{
		Addr:              ":" + mainPort,
		Handler:           h,
		TLSConfig:         nil,
		ReadTimeout:       1 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       1 * time.Second,
		MaxHeaderBytes:    222,
		TLSNextProto:      nil,
		ConnState:         connState,
		ErrorLog:          log.Default(),
		BaseContext:       nil,
		ConnContext:       nil,
	}

	log.Init("-------------- Open http://localhost" + server.Addr + "/myapp --------------")
	log.Fatal(server.ListenAndServe())
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(gw garcon.Writer, c *garcon.JWTChecker) http.Handler {
	r := chi.NewRouter()

	// Static website files
	ws := garcon.StaticWebServer{Dir: "examples/www", Writer: gw}
	r.Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
	r.With(c.Set).Get("/myapp", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(c.Set).Get("/myapp/", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(c.Chk).Get("/myapp/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
	r.With(c.Chk).Get("/myapp/css/*", ws.ServeDir("text/css; charset=utf-8"))
	r.With(c.Chk).Get("/myapp/images/*", ws.ServeImages())
	r.With(c.Chk).Get("/myapp/version", garcon.ServeVersion())

	// API
	r.With(c.Vet).Get("/path/not/in/cookie", items)
	r.With(c.Vet).Get("/myapp/api/v1/items", items)
	r.With(c.Vet).Get("/myapp/api/v1/ducks", gw.NotImplemented)

	// Other endpoints
	r.NotFound(gw.InvalidPath)

	return r
}

func items(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`["item1","item2","item3"]`))
}
