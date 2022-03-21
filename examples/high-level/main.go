// SPDX-License-Identifier: CC0-1.0+
// Creative Commons Zero v1.0 Universal - No Rights Reserved
//
// To the extent possible under law, the Teal.Finance contributors
// have waived all copyright and related/neighboring rights to this
// file "high-level/main.go" to be freely used without any restriction.
// See <https://creativecommons.org/publicdomain/zero/1.0>

package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/jwtperm"
	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/webserver"
)

// Garcon settings
const (
	authCfg                      = "examples/sample-auth.rego"
	mainPort, pprofPort, expPort = 8080, 8093, 9093
	burst, perMinute             = 10, 30

	// the HMAC-SHA256 key to decode JWT (to be removed from source code)
	hmacSHA256 = "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
)

func main() {
	auth := flag.Bool("auth", false, "Enable OPA authorization specified in file "+authCfg)
	prod := flag.Bool("prod", false, "Use settings for production")
	flag.Parse()

	opaFilenames := []string{}
	if *auth {
		opaFilenames = []string{authCfg}
	}

	var addr string
	if *prod {
		addr = "https://my-dns.co/myapp/"
	} else {
		addr = "http://localhost:" + strconv.Itoa(mainPort) + "/myapp"
	}

	g, err := garcon.New(
		garcon.WithURLs(addr),
		garcon.WithDocURL("/doc"),
		garcon.WithServerHeader("MyApp-1.2.0"),
		garcon.WithJWT([]byte(hmacSHA256), "FreePlan", 10, "PremiumPlan", 100),
		garcon.WithOPA(opaFilenames...),
		garcon.WithReqLogs(),
		garcon.WithLimiter(burst, perMinute),
		garcon.WithPProf(pprofPort),
		garcon.WithProm(expPort, "https://example.com/path/myapp/"),
		garcon.WithDev(!*prod),
	)

	if err != nil {
		log.Fatal(err)
	}

	// handles both REST API and static web files
	h := handler(g.ResErr, g.JWT)

	err = g.Run(h, mainPort)
	log.Fatal(err)
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
