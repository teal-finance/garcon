// SPDX-License-Identifier: CC0-1.0
// Creative Commons Zero v1.0 Universal - No Rights Reserved
// <https://creativecommons.org/publicdomain/zero/1.0>
//
// To the extent possible under law, the Teal.Finance/Garcon contributors
// have waived all copyright and related/neighboring rights to this
// file "high-level/main.go" to be freely used without any restriction.

package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/garcon"
)

// Garcon settings
const (
	opaFile                      = "examples/sample-auth.rego"
	mainPort, pprofPort, expPort = 8080, 8093, 9093
	burst, perMinute             = 10, 30

	// the HMAC-SHA256 key to decode JWT (do not put your secret keys in your code)
	hmacSHA256 = "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
	aes128bits = "00112233445566778899aabbccddeeff"
)

func main() {
	defer garcon.ProbeCPU().Stop() // collects the CPU-profile and writes it in the file "cpu.pprof"

	garcon.LogVersion()
	garcon.SetVersionFlag()
	auth := flag.Bool("auth", false, "Enable OPA authorization specified in file "+opaFile)
	prod := flag.Bool("prod", false, "Use settings for production")
	jwt := flag.Bool("jwt", false, "Use JWT in lieu of the Incorruptible token")
	flag.Parse()

	var addr string
	if *prod {
		addr = "https://my-dns.co/myapp"
	} else {
		addr = "http://localhost:" + strconv.Itoa(mainPort) + "/myapp"
	}

	g := garcon.New(
		garcon.WithURLs(addr),
		garcon.WithDocURL("/doc"),
		garcon.WithPProf(pprofPort),
		garcon.WithDev(!*prod),
		nil, // just to test "none" option
	)

	var ck garcon.TokenChecker
	if *jwt {
		ck = g.JWTChecker(hmacSHA256, "FreePlan", 10, "PremiumPlan", 100)
	} else {
		ck = g.IncorruptibleChecker(aes128bits, 60, true)
	}

	middleware, connState := g.StartMetricsServer(expPort)
	middleware = middleware.Append(g.MiddlewareRejectUnprintableURI())
	middleware = middleware.Append(g.MiddlewareLogRequest("fingerprint"))
	middleware = middleware.Append(g.MiddlewareRateLimiter(burst, perMinute))
	middleware = middleware.Append(g.MiddlewareServerHeader("MyApp"))
	middleware = middleware.Append(g.MiddlewareCORS())
	middleware = middleware.Append(g.MiddlewareLogDuration(true))

	if *auth {
		middleware = middleware.Append(g.MiddlewareOPA(opaFile))
	}

	// handles both REST API and static web files
	r := handler(g, addr, ck)
	h := middleware.Then(r)

	server := garcon.Server(h, mainPort, connState)

	log.Print("-------------- Open http://localhost:8080/myapp --------------")
	err := garcon.ListenAndServe(&server)
	log.Fatal(err)
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(g *garcon.Garcon, addr string, ck garcon.TokenChecker) http.Handler {
	r := chi.NewRouter()

	// Static website files
	ws := g.NewStaticWebServer("examples/www")
	r.Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
	r.With(ck.Set).Get("/myapp", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(ck.Set).Get("/myapp/", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(ck.Chk).Get("/myapp/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
	r.With(ck.Chk).Get("/myapp/css/*", ws.ServeDir("text/css; charset=utf-8"))
	r.With(ck.Chk).Get("/myapp/images/*", ws.ServeImages())
	r.With(ck.Chk).Get("/myapp/version", garcon.ServeVersion())

	// Contact-form
	wf := g.NewContactForm(addr)
	r.With(ck.Set).Post("/myapp", wf.Notify(""))

	// API
	r.With(ck.Vet).Get("/path/not/in/cookie", items)
	r.With(ck.Vet).Get("/myapp/api/v1/items", items)
	r.With(ck.Vet).Get("/myapp/api/v1/ducks", g.Writer.NotImplemented)

	// Other endpoints
	r.NotFound(g.Writer.InvalidPath)

	return r
}

func items(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`["item1","item2","item3"]`))
}
