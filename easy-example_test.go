// CC0-1.0: Creative Commons Zero v1.0 Universal
// No Rights Reserved - (CC) ZERO - (0) PUBLIC DOMAIN
//
// To the extent possible under law, the Teal.Finance contributors
// have waived all copyright and related or neighboring rights
// to this file "easy-example_test.go" to be copied without restrictions.
// Refer to https://creativecommons.org/publicdomain/zero/1.0

package server_test

import (
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/teal-finance/server"
	"github.com/teal-finance/server/fileserver"
	"github.com/teal-finance/server/reserr"
)

func ExampleServer() {
	s := server.Server{
		Version:        "MyApp-1.2.3",
		ResErr:         "https://my-dns.com/doc",
		AllowedOrigins: []string{"http://my-dns.com"},
		OPAFilenames:   []string{"example-auth.rego"},
	}

	// Handles both REST API and static web files
	h := handler(s.ResErr)

	log.Fatal(s.RunServer(h, 8080, 9093, 10, 30, true))
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
