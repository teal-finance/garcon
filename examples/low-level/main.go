// CC0-1.0: Creative Commons Zero v1.0 Universal
// No Rights Reserved - (CC) ZERO - (0) PUBLIC DOMAIN
//
// To the extent possible under law, the Teal.Finance contributors
// have waived all copyright and related or neighboring rights
// to this file "full-example_test.go" to be copied without restrictions.
// Refer to https://creativecommons.org/publicdomain/zero/1.0

package main

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/teal-finance/server"
	"github.com/teal-finance/server/chain"
	"github.com/teal-finance/server/cors"
	"github.com/teal-finance/server/fileserver"
	"github.com/teal-finance/server/limiter"
	"github.com/teal-finance/server/metrics"
	"github.com/teal-finance/server/opa"
	"github.com/teal-finance/server/pprof"
	"github.com/teal-finance/server/reserr"
)

func main() {
	const pprofPort = 8093
	pprof.StartServer(pprofPort)

	// Uniformize error responses with API doc
	resErr := reserr.New("https://my.dns.co/doc")

	middlewares, connState := setMiddlewares(resErr)

	// Handles both REST API and static web files
	h := handler(resErr)
	h = middlewares.Then(h)

	runServer(h, connState)
}

func setMiddlewares(resErr reserr.ResErr) (middlewares chain.Chain, connState func(net.Conn, http.ConnState)) {
	const expPort = 9093
	const burst, reqPerMinute = 10, 30
	const devMode = true

	if devMode {
		// the following line writes a CPU-profile file of the function setMiddlewares()
		defer pprof.ProbeCPU().Stop()
	}

	// Start a metrics server in background if export port > 0.
	// The metrics server is for use with Prometheus or another compatible monitoring tool.
	metrics := metrics.Metrics{}
	middlewares, connState = metrics.StartServer(expPort, devMode)

	// Limit the input request rate per IP
	reqLimiter := limiter.New(burst, reqPerMinute, devMode, resErr)

	// CORS
	allowedOrigins := server.SplitClean("https://my.dns.co")

	middlewares = middlewares.Append(
		server.LogRequests,
		reqLimiter.Limit,
		server.Header("MyServerName-1.2.0"),
		cors.Handler(allowedOrigins, devMode),
	)

	// Endpoint authentication rules (Open Policy Agent)
	files := server.SplitClean("example-auth.rego")
	policy, err := opa.New(files, resErr)
	if err != nil {
		log.Fatal(err)
	}

	if policy.Ready() {
		middlewares = middlewares.Append(policy.Auth)
	}

	return middlewares, connState
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
func handler(resErr reserr.ResErr) http.Handler {
	r := chi.NewRouter()

	// Static website files
	fs := fileserver.FileServer{Dir: "/var/www/my-site", ResErr: resErr}
	r.Get("/", fs.ServeFile("index.html", "text/html; charset=utf-8"))
	r.Get("/js/*", fs.ServeDir("text/javascript; charset=utf-8"))
	r.Get("/css/*", fs.ServeDir("text/css; charset=utf-8"))
	r.Get("/images/*", fs.ServeImages())

	// API
	r.Get("/api/v1/items", items)
	r.Get("/api/v1/ducks", resErr.NotImplemented)

	// Other endpoints
	r.NotFound(resErr.InvalidPath)

	return r
}

func items(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`["item1","item2","item3"]`))
}
