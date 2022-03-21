// #region <editor-fold desc="Preamble">
// Copyright (c) 2021-2022 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon,
// an opinionated boilerplate API and website server,
// licensed under LGPL-3.0-or-later.
// SPDX-License-Identifier: LGPL-3.0-or-later
//
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 or any later version, at the licensee’s option.
//
// Teal.Finance/Garcon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
//
// For more details, see the LICENSE file (alongside the source files)
// or the GNU General Public License: <https://www.gnu.org/licenses/>
// #endregion </editor-fold>

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

// ProbeCPU is used like the following:
//
//     defer pprof.ProbeCPU.Stop()
//
// When the caller reaches its function end,
// the defer executes Stop() that writes the file "cpu.pprof".
// To visualize "cpu.pprof" use the pprof tool:
//
//    cd ~/go
//    go get -u github.com/google/pprof
//    cd -
//    pprof -http=: cpu.pprof
func ProbeCPU() Stoppable {
	log.Print("Probing CPU. To visualize the profile: pprof -http=: cpu.pprof")

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
