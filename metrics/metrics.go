// Teal.Finance/Garcon is an opinionated boilerplate API and website server.
// Copyright (C) 2021 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon, licensed under LGPL-3.0-or-later.
//
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 of the License, or (at your option) any later version.
//
// Teal.Finance/Garcon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.

package metrics

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/teal-finance/garcon/chain"
)

type Metrics struct {
	conn     int64 // gauge   - Current number of HTTP connections
	active   int64 // counter - Accumulate HTTP connections that have been in StateActive
	idle     int64 // counter - Accumulate HTTP connections that have been in StateIdle
	hijacked int64 // counter - Accumulate HTTP connections that have been in StateHijacked
}

// MetricsServer creates and starts the Prometheus export server.
func (m *Metrics) StartServer(port int, devMode bool) (chain.Chain, func(net.Conn, http.ConnState)) {
	if port <= 0 {
		log.Print("Disable Prometheus, export port=", port)

		return nil, nil
	}

	addr := ":" + strconv.Itoa(port)

	go func() {
		err := http.ListenAndServe(addr, handler())
		log.Fatal(err)
	}()

	log.Print("Prometheus export http://localhost" + addr)

	// connState counts the HTTP client connections.
	// In prod mode, we do not care about minor counting errors, we use the unsafe-thread version.
	// In dev mode we use the atomic version to avoid warnings from "go build -race".
	var connState func(net.Conn, http.ConnState)
	if devMode {
		connState = m.updateConnCountersAtomic()
	} else {
		connState = m.updateConnCounters()
	}

	return chain.New(m.count), connState
}

// handler returns the endpoints "/metrics/xxx".
func handler() http.Handler {
	sink, err := prometheus.NewPrometheusSink()
	if err != nil {
		log.Print("ERR: NewPrometheusSink cannot register sink because ", err)

		return nil
	}

	if _, err := metrics.NewGlobal(metrics.DefaultConfig("Rainbow"), sink); err != nil {
		log.Print("ERR: Prometheus export is not able to provide metrics because ", err)

		return nil
	}

	handler := chi.NewRouter()
	handler.Handle("/metrics", promhttp.Handler())

	return handler
}

// count increments/decrements web traffic metrics depending on incoming requests and outgoing responses.
func (m *Metrics) count(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		record := &statusRecorder{ResponseWriter: w, Status: "success"}

		next.ServeHTTP(record, r)

		labels := []metrics.Label{
			{Name: "method", Value: r.Method},
			{Name: "route", Value: r.RequestURI},
			{Name: "status", Value: record.Status},
		}

		duration := time.Since(start)
		metrics.AddSampleWithLabels([]string{"request_duration"}, float32(duration.Milliseconds()), labels)

		log.Print("out ", r.RemoteAddr, " ", r.Method, " ", r.URL, " ", duration,
			" c=", m.conn, " a=", m.active, " i=", m.idle, " h=", m.hijacked)
	})
}

// updateConnCounters increments/decrements the number of connections.
func (m *Metrics) updateConnCounters() (connState func(net.Conn, http.ConnState)) {
	return func(_ net.Conn, cs http.ConnState) {
		switch cs {
		// StateNew: the client just connects to TealAPI expecting a request.
		// Transition to either StateActive or StateClosed.
		case http.StateNew:
			m.conn++
		// StateActive when 1 or more bytes of a request has been read.
		// After the request is handled, transitions to StateClosed, StateHijacked, or StateIdle.
		// HTTP/2: StateActive only transitions away once all active requests are complete.
		case http.StateActive:
			m.active++
		// StateIdle when handling a request is finished and is in the keep-alive state,
		// waiting for a new request, then transitions to either StateActive or StateClosed.
		case http.StateIdle:
			m.idle++
		// StateHijacked is a terminal state: does not transition to StateClosed.
		case http.StateHijacked:
			m.hijacked++
			m.conn--
		// StateClosed is a terminal state.
		case http.StateClosed:
			m.conn--
		}
		metrics.SetGauge([]string{"conn_count"}, float32(m.conn))
	}
}

func (m *Metrics) updateConnCountersAtomic() (connState func(net.Conn, http.ConnState)) {
	return func(_ net.Conn, cs http.ConnState) {
		switch cs {
		case http.StateNew:
			atomic.AddInt64(&m.conn, 1)
		case http.StateActive:
			atomic.AddInt64(&m.active, 1)
		case http.StateIdle:
			atomic.AddInt64(&m.idle, 1)
		case http.StateHijacked:
			atomic.AddInt64(&m.hijacked, 1)
			atomic.AddInt64(&m.conn, -1)
		case http.StateClosed:
			atomic.AddInt64(&m.conn, -1)
		}
	}
}

type statusRecorder struct {
	http.ResponseWriter
	Status string
}

func (r *statusRecorder) WriteHeader(status int) {
	if status != http.StatusOK {
		r.Status = "error"
	}

	r.ResponseWriter.WriteHeader(status)
}
