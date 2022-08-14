// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerName is used in multiple parts in Garcon:
// - the HTTP Server header,
// - the Prometheus namespace.
type ServerName string

// ExtractName extracts the wider "[a-zA-Z0-9_]+" string from the end of str.
// If str is a path or an URL, keep the last basename.
// Example: keep "myapp" from "https://example.com/path/myapp/"
// ExtractName also removes all punctuation characters except "_".
func ExtractName(str string) ServerName {
	str = strings.Trim(str, "/")
	if i := strings.LastIndex(str, "/"); i >= 0 {
		str = str[i+1:]
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	str = re.ReplaceAllLiteralString(str, "")
	return ServerName(str)
}

// SetPromNamingRule verifies Prom naming rules for namespace and fixes it if necessary.
// valid namespace = [a-zA-Z][a-zA-Z0-9_]*
// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
func (ns *ServerName) SetPromNamingRule() {
	str := ns.String()
	if !unicode.IsLetter(rune(str[0])) {
		*ns = ServerName("a" + str)
	}
}

func (ns ServerName) String() string {
	return string(ns)
}

// StartMetricsServer creates and starts the Prometheus export server.
func (g *Garcon) StartMetricsServer(expPort int) (Chain, func(net.Conn, http.ConnState)) {
	return StartMetricsServer(expPort, g.ServerName)
}

// StartMetricsServer creates and starts the Prometheus export server.
func StartMetricsServer(port int, namespace ServerName) (Chain, func(net.Conn, http.ConnState)) {
	if port <= 0 {
		log.Print("INF Disable Prometheus, export port=", port)
		return nil, nil
	}

	addr := ":" + strconv.Itoa(port)

	go func() {
		err := http.ListenAndServe(addr, metricsHandler())
		log.Fatal(err)
	}()

	log.Print("INF Prometheus export http://localhost" + addr + " namespace=" + namespace.String())

	// Add build info
	prometheus.MustRegister(collectors.NewBuildInfoCollector())

	namespace.SetPromNamingRule()
	chain := NewChain(namespace.MiddlewareMeasureDuration)
	counter := namespace.updateHTTPMetrics()
	return chain, counter
}

// metricsHandler exports the metrics by processing
// the Prometheus requests on the "/metrics" endpoint.
func metricsHandler() http.Handler {
	handler := chi.NewRouter()
	handler.Handle("/metrics", promhttp.Handler())
	return handler
}

// MiddlewareMeasureDuration measures the time to handle a request.
func (ns ServerName) MiddlewareMeasureDuration(next http.Handler) http.Handler {
	summary := ns.newSummaryVec(
		"request_duration_seconds",
		"Time to handle a client request",
		"code",
		"route")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		record := &statusRecorder{ResponseWriter: w, Status: "success"}
		next.ServeHTTP(record, r)
		duration := time.Since(start)
		summary.WithLabelValues(record.Status, r.RequestURI).Observe(duration.Seconds())
		log.Print(safeIPMethodURLDuration(r, duration))
	})
}

// updateHTTPMetrics counts the connections and update web traffic metrics
// depending on incoming requests and outgoing responses.
func (ns ServerName) updateHTTPMetrics() func(net.Conn, http.ConnState) {
	connGauge := ns.newGauge("in_flight_connections", "Number of current active connections")
	iniCounter := ns.newCounter("conn_new_total", "Total initiated connections since startup")
	reqCounter := ns.newCounter("conn_req_total", "Total requested connections since startup")
	resCounter := ns.newCounter("conn_res_total", "Total responded connections since startup")
	hijCounter := ns.newCounter("conn_hij_total", "Total hijacked connections since startup")

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

func (ns ServerName) newSummaryVec(name, help string, labels ...string) *prometheus.SummaryVec {
	return promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:   string(ns),
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

func (ns ServerName) newGauge(name, help string) prometheus.Gauge {
	return promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   string(ns),
		Subsystem:   "http",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	})
}

func (ns ServerName) newCounter(name, help string) prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   string(ns),
		Subsystem:   "http",
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	})
}
