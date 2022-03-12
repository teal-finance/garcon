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
	"time"

	"github.com/teal-finance/garcon/chain"
	"github.com/teal-finance/garcon/security"

	"github.com/armon/go-metrics"
	metricsProm "github.com/armon/go-metrics/prometheus"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	conn float64 // Number of current active HTTP connections

	connGauge  prometheus.Gauge
	iniCounter prometheus.Counter
	reqCounter prometheus.Counter
	resCounter prometheus.Counter
	hijCounter prometheus.Counter
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

	m.connGauge = prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "http", Name: "conn", Help: "Number of current active HTTP connections"})
	m.iniCounter = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "http", Name: "new", Help: "Total initiated HTTP connections since startup"})
	m.reqCounter = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "http", Name: "req", Help: "Total requested HTTP connections since startup"})
	m.resCounter = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "http", Name: "res", Help: "Total responded HTTP connections since startup"})
	m.hijCounter = prometheus.NewCounter(prometheus.CounterOpts{Namespace: "http", Name: "hij", Help: "Total hijacked HTTP connections since startup"})

	prometheus.MustRegister(m.connGauge, m.iniCounter, m.reqCounter, m.resCounter, m.hijCounter)

	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())

	return chain.New(m.count), m.updateConnCounters()
}

// handler returns the endpoint "/metrics".
func handler() http.Handler {
	sink, err := metricsProm.NewPrometheusSink()
	if err != nil {
		log.Fatal("ERR: NewPrometheusSink cannot register sink because ", err)
	}

	if _, err := metrics.NewGlobal(metrics.DefaultConfig("rainbow"), sink); err != nil {
		log.Fatal("ERR: Prometheus export is not able to provide metrics because ", err)
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

		uri := security.Sanitize(r.RequestURI)
		log.Print("out ", r.RemoteAddr, " ", r.Method, " ", uri, " ", duration, " c=", int(m.conn))
	})
}

// updateConnCounters counts the number of HTTP client connections.
func (m *Metrics) updateConnCounters() (connState func(net.Conn, http.ConnState)) {
	return func(_ net.Conn, cs http.ConnState) {
		switch cs {
		// StateNew: the client just connects, the server expects its request.
		// Transition to either StateActive or StateClosed.
		case http.StateNew:
			m.iniCounter.Inc()
			m.conn++
			m.connGauge.Set(m.conn)

		// StateActive: a request is being received.
		// Transition to StateClosed, StateHijacked or StateIdle, after the request is handled.
		// HTTP/2: StateActive only transitions away once all active requests are complete.
		case http.StateActive:
			m.reqCounter.Inc()

		// StateIdle: the server has handled the request and is in the keep-alive state waiting for a new request.
		// Transitions to either StateActive or StateClosed.
		case http.StateIdle:
			m.resCounter.Inc()

		// StateHijacked: terminal state.
		case http.StateHijacked:
			m.hijCounter.Inc()
			m.conn--
			m.connGauge.Set(m.conn)

		// StateClosed: terminal state.
		case http.StateClosed:
			m.conn--
			m.connGauge.Set(m.conn)
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
