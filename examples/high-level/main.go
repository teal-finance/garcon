// CC0-1.0: Creative Commons Zero v1.0 Universal
// No Rights Reserved - (CC) ZERO - (0) PUBLIC DOMAIN
//
// To the extent possible under law, the Teal.Finance contributors
// have waived all copyright and related or neighboring rights
// to this file "easy-example_test.go" to be copied without restrictions.
// Refer to https://creativecommons.org/publicdomain/zero/1.0

package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/fileserver"
	"github.com/teal-finance/garcon/reserr"
)

// Garcon settings
const authCfg = "examples/sample-auth.rego"
const mainPort, pprofPort, expPort = 8080, 8093, 9093
const burst, reqMinute = 10, 30
const devMode = true

func main() {
	auth := flag.Bool("auth", false, "Enable OPA authorization specified in file "+authCfg)
	flag.Parse()

	// other Garcon settings
	s := garcon.Garcon{
		Version:        "MyBackendName-1.2.0",
		ResErr:         "https://my.dns.co/doc",
		AllowedOrigins: []string{"https://my.dns.co"},
	}

	if *auth {
		s.OPAFilenames = []string{authCfg}
	}

	// handles both REST API and static web files
	h := handler(s.ResErr)

	err := s.Run(h, mainPort, pprofPort, expPort, burst, reqMinute, devMode)
	log.Fatal(err)
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(resErr reserr.ResErr) http.Handler {
	r := chi.NewRouter()

	// Static website files
	fs := fileserver.FileServer{Dir: "examples/www", ResErr: resErr}
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
