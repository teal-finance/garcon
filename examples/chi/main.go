// SPDX-License-Identifier: CC0-1.0
// Creative Commons Zero v1.0 Universal - No Rights Reserved
// <https://creativecommons.org/publicdomain/zero/1.0>
//
// To the extent possible under law, the Teal.Finance/Garcon contributors
// have waived all copyright and related/neighboring rights to this
// file "complete/main.go" to be freely used without any restriction.

package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/garcon"
)

const port = ":22000"

func main() {
	endpoint := flag.String("post-endpoint", "/", "The endpoint for the POST request.")
	flag.Parse()

	g := garcon.New(garcon.WithNamespace("ChiServer"))

	middleware := garcon.NewChain(
		g.MiddlewareLogRequest("safe"),
		g.MiddlewareServerHeader(),
	)

	router := chi.NewRouter()
	router.Post(*endpoint, post)
	router.MethodNotAllowed(others) // handle other methods of the above POST endpoint
	router.NotFound(others)         // handle all other endpoints

	handler := middleware.Then(router)

	server := http.Server{
		Addr:    port,
		Handler: handler,
	}

	log.Print("INF Server listening on http://localhost", port)
	log.Fatal(server.ListenAndServe())
}

func post(w http.ResponseWriter, _ *http.Request) {
	log.Print("INF router.Post()")
	_, _ = w.Write([]byte("<html><body> router.Post() </body></html>"))
}

func others(w http.ResponseWriter, _ *http.Request) {
	log.Print("INF router.NotFound()")
	_, _ = w.Write([]byte("<html><body> router.MethodNotAllowed() </body></html>"))
}
