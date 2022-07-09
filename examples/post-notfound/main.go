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

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/garcon"
)

const port = ":22000"

func main() {
	endpoint := flag.String("post-endpoint", "/", "The endpoint for the POST request.")
	flag.Parse()

	handler := chi.NewRouter()
	handler.Post(*endpoint, post)
	handler.NotFound(notFound)

	chain := garcon.NewChain(
		garcon.LogRequest,
		garcon.LogDuration)

	server := http.Server{
		Addr:    port,
		Handler: chain.Then(handler),
	}

	log.Print("Server listening on http://localhost", port)

	log.Fatal(server.ListenAndServe())
}

func post(w http.ResponseWriter, _ *http.Request) {
	log.Print("handler.Post()")
	_, _ = w.Write([]byte("<html><body> handler.Post() </body></html>"))
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	log.Print("handler.NotFound()")
	_, _ = w.Write([]byte("<html><body> handler.NotFound() </body></html>"))
}
