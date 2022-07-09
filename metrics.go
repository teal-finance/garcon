// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

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
)

// space holds the Prometheus namespace.
type space struct {
	namespace string
}

// StartMetricsServer creates and starts the Prometheus export server.
func StartMetricsServer(port int, namespace string) (Chain, func(net.Conn, http.ConnState)) {
	if port <= 0 {
		log.Print("Disable Prometheus, export port=", port)
		return nil, nil
	}

	addr := ":" + strconv.Itoa(port)

	go func() {
		err := http.ListenAndServe(addr, metricsHandler())
		log.Fatal(err)
	}()

	log.Print("Prometheus export http://localhost" + addr + " namespace=" + namespace)

	// Add build info.
	prometheus.MustRegister(collectors.NewBuildInfoCollector())

	s := space{namespace: namespace}

	return NewChain(s.measureDuration), s.updateHTTPMetrics()
}

// metricsHandler exports the metrics by processing
// the Prometheus requests on the "/metrics" endpoint.
func metricsHandler() http.Handler {
	handler := chi.NewRouter()
	handler.Handle("/metrics", promhttp.Handler())
	return handler
}

// measureDuration measures the time to handle a request.
func (s *space) measureDuration(next http.Handler) http.Handler {
	summary := s.newSummaryVec(
		"request_duration_seconds",
		"Time to handle a client request",
		"code",
		"route")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		record := &statusRecorder{ResponseWriter: w, Status: "success"}

		next.ServeHTTP(record, r)

		duration := time.Since(start)

		summary.WithLabelValues(record.Status, r.RequestURI).
			Observe(duration.Seconds())

		uri := Sanitize(r.RequestURI)
		log.Print("out ", r.RemoteAddr, " ", r.Method, " ", uri, " ", duration)
	})
}

// updateHTTPMetrics counts the connections and update web traffic metrics
// depending on incoming requests and outgoing responses.
func (s *space) updateHTTPMetrics() func(net.Conn, http.ConnState) {
	connGauge := s.newGauge("in_flight_connections", "Number of current active connections")
	iniCounter := s.newCounter("conn_new_total", "Total initiated connections since startup")
	reqCounter := s.newCounter("conn_req_total", "Total requested connections since startup")
	resCounter := s.newCounter("conn_res_total", "Total responded connections since startup")
	hijCounter := s.newCounter("conn_hij_total", "Total hijacked connections since startup")

	return func(_ net.Conn, cs http.ConnState) {
		switch cs {
		// StateNew: the client just connects, the server expects its request.
		// Transition to either StateActive or StateClosed.
		case http.StateNew:
			connGauge.Inc()
			iniCounter.Inc()

		// StateActive: a request is being received.
		// Transition to StateClosed, StateHijacked or StateIdle, after the request is handled.
		// HTTP/2: StateActive only transitions away once all active requests are complete.
		case http.StateActive:
			reqCounter.Inc()

		// StateIdle: the server has handled the request and is in the keep-alive state waiting for a new request.
		// Transitions to either StateActive or StateClosed.
		case http.StateIdle:
			resCounter.Inc()

		// StateHijacked: terminal state.
		case http.StateHijacked:
			connGauge.Dec()
			hijCounter.Inc()

		// StateClosed: terminal state.
		case http.StateClosed:
			connGauge.Dec()
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

func (s *space) newSummaryVec(name, help string, labels ...string) *prometheus.SummaryVec {
	return promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:   s.namespace,
		Subsystem:   "http",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
		Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		MaxAge:      24 * time.Hour,
		AgeBuckets:  0,
		BufCap:      0,
	}, labels)
}

func (s *space) newGauge(name, help string) prometheus.Gauge {
	return promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   s.namespace,
		Subsystem:   "http",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	})
}

func (s *space) newCounter(name, help string) prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   s.namespace,
		Subsystem:   "http",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	})
}
