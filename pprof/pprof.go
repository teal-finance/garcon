// Copyright (C) 2020-2021 TealTicks contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package pprof serves the /debug/pprof endpoint
package pprof

import (
	"log"
	"net/http"
	"net/http/pprof"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/profile"
)

type Stoppable interface {
	Stop()
}

// WriteCPUProfile should be called in a high level function like the following:
//
//     defer WriteCPUProfile.Stop()
//
// When the caller reaches its function end,
// the defer executes Stop() that writes the file "cpu.pprof".
// To visualize that "cpu.pprof" file use the pprof tool:
//
//    cd ~/go
//    go get -u github.com/google/pprof
//    cd -
//    pprof -http=: cpu.pprof
func WriteCPUProfile() Stoppable {
	return profile.Start(profile.ProfilePath("."))
}

// StartServer starts a PProf server in background.
// Endpoints usage example:
//
//     curl http://localhost:6063/debug/pprof/allocs > allocs.pprof
//     pprof -http=: allocs.pprof
//
//     wget http://localhost:31415/debug/pprof/goroutine
//     pprof -http=: goroutine
//
//     wget http://localhost:31415/debug/pprof/heap
//     pprof -http=: heap
//
//     wget http://localhost:31415/debug/pprof/trace
//     pprof -http=: trace
func StartServer(port int) {
	if port == 0 {
		return // Disable PProf endpoints /debug/pprof/*
	}

	addr := "localhost:" + strconv.Itoa(port)
	h := handler()

	go runServer(addr, h)
}

func handler() http.Handler {
	r := chi.NewRouter()

	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.NotFound(pprof.Index) // also serves /debug/pprof/{heap,goroutine,block…}

	return r
}

func runServer(addr string, handler http.Handler) {
	log.Print("Enable PProf endpoints: http://" + addr + "/debug/pprof")

	err := http.ListenAndServe(addr, handler)
	log.Fatal(err)
}
