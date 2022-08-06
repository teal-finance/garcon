// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"log"
	"net/http"
	"net/http/pprof"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/profile"
)

// ProbeCPU is used like the following:
//
//	defer pprof.ProbeCPU.Stop()
//
// When the caller reaches its function end,
// the defer executes Stop() that writes the file "cpu.pprof".
// To visualize "cpu.pprof" use the pprof tool:
//
//	cd ~/go
//	go get -u github.com/google/pprof
//	cd -
//	pprof -http=: cpu.pprof
//
// or using one single command line:
//
//	go run github.com/google/pprof@latest -http=: cpu.pprof
func ProbeCPU() interface{ Stop() } {
	log.Print("Probing CPU. To visualize the profile: pprof -http=: cpu.pprof")
	return profile.Start(profile.ProfilePath("."))
}

// StartPProfServer starts a PProf server in background.
// Endpoints usage example:
//
//	curl http://localhost:6063/debug/pprof/allocs > allocs.pprof
//	pprof -http=: allocs.pprof
//
//	wget http://localhost:31415/debug/pprof/goroutine
//	pprof -http=: goroutine
//
//	wget http://localhost:31415/debug/pprof/heap
//	pprof -http=: heap
//
//	wget http://localhost:31415/debug/pprof/trace
//	pprof -http=: trace
func StartPProfServer(port int) {
	if port == 0 {
		return // Disable PProf endpoints /debug/pprof/*
	}

	addr := "localhost:" + strconv.Itoa(port)
	h := pProfHandler()

	go runPProfServer(addr, h)
}

// pProfHandler serves the /debug/pprof/* endpoints.
func pProfHandler() http.Handler {
	r := chi.NewRouter()
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.NotFound(pprof.Index) // also serves /debug/pprof/{heap,goroutine,block...}
	return r
}

func runPProfServer(addr string, handler http.Handler) {
	log.Print("Enable PProf endpoints: http://" + addr + "/debug/pprof")
	err := http.ListenAndServe(addr, handler)
	log.Panic(err)
}
