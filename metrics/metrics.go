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

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/teal-finance/garcon/chain"
	"github.com/teal-finance/garcon/security"
)

type Metrics struct {
	reqDuration *prometheus.SummaryVec
	connGauge   prometheus.Gauge
	iniCounter  prometheus.Counter
	reqCounter  prometheus.Counter
	resCounter  prometheus.Counter
	hijCounter  prometheus.Counter
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

	m.reqDuration = newSummaryVec("request_duration_seconds", "Time to handle a client request", "code", "route")
	m.connGauge = newGauge("in_flight_connections", "Number of current active connections")
	m.iniCounter = newCounter("conn_new_total", "Total initiated connections since startup")
	m.reqCounter = newCounter("conn_req_total", "Total requested connections since startup")
	m.resCounter = newCounter("conn_res_total", "Total responded connections since startup")
	m.hijCounter = newCounter("conn_hij_total", "Total hijacked connections since startup")

	// Add build info.
	prometheus.MustRegister(collectors.NewBuildInfoCollector())

	return chain.New(m.measureDuration), m.updateHTTPMetrics()
}

// handler returns the endpoint "/metrics".
func handler() http.Handler {
	handler := chi.NewRouter()
	handler.Handle("/metrics", promhttp.Handler())

	return handler
}

// measureDuration measures the time to handle a request.
func (m *Metrics) measureDuration(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		record := &statusRecorder{ResponseWriter: w, Status: "success"}

		next.ServeHTTP(record, r)

		duration := time.Since(start)

		m.reqDuration.WithLabelValues(record.Status, r.RequestURI).
			Observe(duration.Seconds())

		uri := security.Sanitize(r.RequestURI)
		log.Print("out ", r.RemoteAddr, " ", r.Method, " ", uri, " ", duration)
	})
}

// updateHTTPMetrics counts the connections and update web traffic metrics depending on incoming requests and outgoing responses.
func (m *Metrics) updateHTTPMetrics() (connState func(net.Conn, http.ConnState)) {
	return func(_ net.Conn, cs http.ConnState) {
		switch cs {
		// StateNew: the client just connects, the server expects its request.
		// Transition to either StateActive or StateClosed.
		case http.StateNew:
			m.connGauge.Inc()
			m.iniCounter.Inc()

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
			m.connGauge.Dec()
			m.hijCounter.Inc()

		// StateClosed: terminal state.
		case http.StateClosed:
			m.connGauge.Dec()
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

func newSummaryVec(name, help string, labels ...string) *prometheus.SummaryVec {
	return promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:   "http",
		Subsystem:   "",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
		Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		MaxAge:      24 * time.Hour,
		AgeBuckets:  0,
		BufCap:      0,
	}, labels)
}

func newGauge(name, help string) prometheus.Gauge {
	return promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   "http",
		Subsystem:   "",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	})
}

func newCounter(name, help string) prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   "http",
		Subsystem:   "",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	})
}
