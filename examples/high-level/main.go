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
	authCfg                      = "examples/sample-auth.rego"
	mainPort, pprofPort, expPort = 8080, 8093, 9093
	burst, perMinute             = 10, 30

	// the HMAC-SHA256 key to decode JWT (to be removed from source code)
	hmacSHA256 = "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
	aes128bits = "00112233445566778899aabbccddeeff"
)

func main() {
	garcon.SetVersionFlag()
	auth := flag.Bool("auth", false, "Enable OPA authorization specified in file "+authCfg)
	prod := flag.Bool("prod", false, "Use settings for production")
	jwt := flag.Bool("jwt", false, "Use JWT in lieu of the incorruptible token")
	flag.Parse()

	garcon.LogVersion()

	opaFilenames := []string{}
	if *auth {
		opaFilenames = []string{authCfg}
	}

	var addr string
	if *prod {
		addr = "https://my-dns.co/myapp"
	} else {
		addr = "http://localhost:" + strconv.Itoa(mainPort) + "/myapp"
	}

	tokenOption := garcon.WithIncorruptible(aes128bits, 60, true)
	if *jwt {
		tokenOption = garcon.WithJWT(hmacSHA256, "FreePlan", 10, "PremiumPlan", 100)
	}

	g, err := garcon.New(
		tokenOption,
		garcon.WithURLs(addr),
		garcon.WithDocURL("/doc"),
		garcon.WithServerHeader("MyApp"),
		garcon.WithOPA(opaFilenames...),
		garcon.WithReqLogs(),
		garcon.WithLimiter(burst, perMinute),
		garcon.WithPProf(pprofPort),
		garcon.WithProm(expPort, "https://example.com/path/myapp/"),
		garcon.WithDev(!*prod),
		nil, // just to test "none" option
	)
	if err != nil {
		log.Fatal(err)
	}

	// handles both REST API and static web files
	h := handler(g, addr)

	err = g.Run(h, mainPort)
	log.Fatal(err)
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(g *garcon.Garcon, addr string) http.Handler {
	r := chi.NewRouter()
	c := g.Checker

	// Static website files
	ws := garcon.NewStaticWebServer("examples/www", g.Writer)
	r.Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
	r.With(c.Set).Get("/myapp", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(c.Set).Get("/myapp/", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(c.Chk).Get("/myapp/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
	r.With(c.Chk).Get("/myapp/css/*", ws.ServeDir("text/css; charset=utf-8"))
	r.With(c.Chk).Get("/myapp/images/*", ws.ServeImages())
	r.With(c.Chk).Get("/myapp/version", g.ServeVersion())

	// Contact-form
	wf := garcon.NewContactForm(addr, "", g.Writer)
	r.With(c.Set).Post("/myapp", wf.NotifyWebForm())

	// API
	r.With(c.Vet).Get("/path/not/in/cookie", items)
	r.With(c.Vet).Get("/myapp/api/v1/items", items)
	r.With(c.Vet).Get("/myapp/api/v1/ducks", g.Writer.NotImplemented)

	// Other endpoints
	r.NotFound(g.Writer.InvalidPath)

	return r
}

func items(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`["item1","item2","item3"]`))
}
