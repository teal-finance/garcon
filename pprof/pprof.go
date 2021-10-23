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
)

func StartServer(port int) {
	if port == 0 {
		return // Disable PProf endpoints /debug/pprof
	}

	handler := pprofHandler()

	go runPProfServer("localhost:"+strconv.Itoa(port), handler)
}

func pprofHandler() http.Handler {
	handler := chi.NewRouter()

	handler.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	handler.HandleFunc("/debug/pprof/profile", pprof.Profile)
	handler.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	handler.HandleFunc("/debug/pprof/trace", pprof.Trace)
	handler.NotFound(pprof.Index) // also serves /debug/pprof/{heap,goroutine,block…}

	return handler
}

func runPProfServer(addr string, handler http.Handler) {
	log.Print("Enable PProf endpoints: http://" + addr + "/debug/pprof")

	err := http.ListenAndServe(addr, handler)
	log.Fatal(err)
}
