// CC0-1.0: Creative Commons Zero v1.0 Universal
// No Rights Reserved - (CC) ZERO - (0) PUBLIC DOMAIN
//
// To the extent possible under law, the Teal.Finance contributors
// have waived all copyright and related or neighboring rights to this
// example file "low-level/main.go" to be modified without restrictions.
// Refer to https://creativecommons.org/publicdomain/zero/1.0

package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/chain"
	"github.com/teal-finance/garcon/cors"
	"github.com/teal-finance/garcon/jwtperm"
	"github.com/teal-finance/garcon/limiter"
	"github.com/teal-finance/garcon/metrics"
	"github.com/teal-finance/garcon/opa"
	"github.com/teal-finance/garcon/pprof"
	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/webserver"
)

// Garcon settings
const (
	apiDoc            = "https://my-dns.co/myapp/doc"
	allowedProdOrigin = "https://my-dns.co"
	allowedDevOrigins = "http://localhost:  http://192.168.1."
	serverHeader      = "MyBackendName-1.2.0"
	authCfg           = "examples/sample-auth.rego"
	pprofPort         = 8093
	expPort           = 9093
	burst, reqMinute  = 10, 30
	// the HMAC-SHA256 key to decode JWT (to be removed from source code)
	hmacSHA256 = "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
)

func main() {
	// the following line collects the CPU-profile and writes it in the file "cpu.pprof"
	defer pprof.ProbeCPU().Stop()

	pprof.StartServer(pprofPort)

	// Uniformize error responses with API doc
	resErr := reserr.New(apiDoc)

	middlewares, connState, urls := setMiddlewares(resErr)

	// Handles both REST API and static web files
	h := handler(resErr, jwtperm.New(urls, resErr, []byte(hmacSHA256)))
	h = middlewares.Then(h)

	runServer(h, connState)
}

func setMiddlewares(resErr reserr.ResErr) (middlewares chain.Chain, connState func(net.Conn, http.ConnState), urls []*url.URL) {
	auth := flag.Bool("auth", false, "Enable OPA authorization specified in file "+authCfg)
	dev := flag.Bool("dev", true, "Use development or production settings")
	flag.Parse()

	// Start a metrics server in background if export port > 0.
	// The metrics server is for use with Prometheus or another compatible monitoring tool.
	metrics := metrics.Metrics{}
	middlewares, connState = metrics.StartServer(expPort, *dev)

	// Limit the input request rate per IP
	reqLimiter := limiter.New(burst, reqMinute, *dev, resErr)

	corsConfig := allowedProdOrigin
	if *dev {
		corsConfig = allowedDevOrigins
	}

	allowedOrigins := garcon.SplitClean(corsConfig)
	urls = garcon.ParseURLs(allowedOrigins)

	middlewares = middlewares.Append(
		reqLimiter.Limit,
		garcon.ServerHeader(serverHeader),
		cors.Handler(allowedOrigins, *dev),
	)

	// Endpoint authentication rules (Open Policy Agent)
	if *auth {
		files := garcon.SplitClean(authCfg)
		policy, err := opa.New(files, resErr)
		if err != nil {
			log.Fatal(err)
		}

		if policy.Ready() {
			middlewares = middlewares.Append(policy.Auth)
		}
	}

	return middlewares, connState, urls
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

	log.Print("Server listening on http://localhost", server.Addr)

	log.Fatal(server.ListenAndServe())
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(resErr reserr.ResErr, jc *jwtperm.Checker) http.Handler {
	r := chi.NewRouter()

	// Static website files
	ws := webserver.WebServer{Dir: "examples/www", ResErr: resErr}
	r.Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
	r.With(jc.Set).Get("/myapp", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(jc.Set).Get("/myapp/", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(jc.Chk).Get("/myapp/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
	r.With(jc.Chk).Get("/myapp/css/*", ws.ServeDir("text/css; charset=utf-8"))
	r.With(jc.Chk).Get("/myapp/images/*", ws.ServeImages())

	// API
	r.With(jc.Vet).Get("/api/v1/items", items)
	r.With(jc.Vet).Get("/myapp/api/v1/items", items)
	r.With(jc.Vet).Get("/myapp/api/v1/ducks", resErr.NotImplemented)

	// Other endpoints
	r.NotFound(resErr.InvalidPath)

	return r
}

func items(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`["item1","item2","item3"]`))
}
