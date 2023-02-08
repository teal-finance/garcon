// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/teal-finance/garcon/gg"
)

// ServerName is used in multiple parts in Garcon:
// - the HTTP Server header,
// - the Prometheus namespace.
type ServerName string

func (ns ServerName) String() string {
	return string(ns)
}

// RespectPromNamingRule verifies Prom naming rules for namespace and fixes it if necessary.
// valid namespace = [a-zA-Z][a-zA-Z0-9_]*
// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
func (ns ServerName) RespectPromNamingRule() ServerName {
	str := ns.String()
	str = strings.ReplaceAll(str, "-", "_")
	if !unicode.IsLetter(rune(str[0])) {
		ns = ServerName("a" + str)
	}
	return ns
}

// ConnState counts the HTTP connections and update web traffic metrics
// depending on incoming requests and outgoing responses.
func (ns ServerName) ConnState() func(net.Conn, http.ConnState) {
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

type statusRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.ResponseWriter.WriteHeader(status)
	r.StatusCode = status
}

// MiddlewareExportTrafficMetrics measures the duration to process a request.
func (ns ServerName) MiddlewareExportTrafficMetrics(next http.Handler) http.Handler {
	summary := ns.newSummaryVec(
		"request_duration_seconds",
		"Time to handle a client request",
		"code",
		"route")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := &statusRecorder{ResponseWriter: w, StatusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(record, r)
		duration := time.Since(start)

		code := StatusCodeStr(record.StatusCode)
		summary.WithLabelValues(code, r.RequestURI).Observe(duration.Seconds())
		log.Out(ipMethodURLDurationSafe(r, code, duration))
	})
}

// MiddlewareLogDuration logs the requested URL along with the time to handle it.
func MiddlewareLogDuration(next http.Handler) http.Handler {
	log.Info("MiddlewareLogDuration logs requester IP, request URL and duration")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := &statusRecorder{ResponseWriter: w, StatusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(record, r)
		d := time.Since(start)

		code := StatusCodeStr(record.StatusCode)
		log.Out(ipMethodURLDuration(r, code, d))
	})
}

// MiddlewareLogDurationSafe is similar to MiddlewareLogDurations but also sanitizes the URL.
func MiddlewareLogDurationSafe(next http.Handler) http.Handler {
	log.Info("MiddlewareLogDurationSafe: logs requester IP, sanitized URL and duration")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := &statusRecorder{ResponseWriter: w, StatusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(w, r)
		d := time.Since(start)

		code := StatusCodeStr(record.StatusCode)
		log.Out(ipMethodURLDurationSafe(r, code, d))
	})
}

// MiddlewareLogRequest logs the incoming request URL.
// If one of its optional parameter is "fingerprint", this middleware also logs the browser fingerprint.
// If the other optional parameter is "safe", this middleware sanitizes the URL before printing it.
func (g *Garcon) MiddlewareLogRequest(settings ...string) gg.Middleware {
	logFingerprint := false
	logSafe := false

	for _, s := range settings {
		switch s {
		case "fingerprint":
			logFingerprint = true
		case "safe":
			logSafe = true
		default:
			log.Panicf(`g.MiddlewareLogRequests() accepts only "fingerprint" and "safe" but got: %q`, s)
		}
	}

	if logFingerprint {
		if logSafe {
			return MiddlewareLogFingerprintSafe
		}
		return MiddlewareLogFingerprint
	}

	if logSafe {
		return MiddlewareLogRequestSafe
	}
	return MiddlewareLogRequest
}

// MiddlewareLogDuration logs the requested URL along with its handling time.
// When the optional parameter safe is true, this middleware sanitizes the URL before printing it.
func (g *Garcon) MiddlewareLogDuration(safe ...bool) gg.Middleware {
	if len(safe) > 0 && safe[0] {
		return MiddlewareLogDurationSafe
	}
	return MiddlewareLogDuration
}

// MiddlewareLogRequest is the middleware to log the requester IP and the requested URL.
func MiddlewareLogRequest(next http.Handler) http.Handler {
	log.Info("MiddlewareLogRequest logs requester IP and request URL")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.In(ipMethodURL(r))
			next.ServeHTTP(w, r)
		})
}

// MiddlewareLogRequestSafe is similar to LogRequest but sanitize the URL.
func MiddlewareLogRequestSafe(next http.Handler) http.Handler {
	log.Info("MiddlewareLogRequestSafe logs requester IP and sanitized URL")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.In(ipMethodURLSafe(r))
			next.ServeHTTP(w, r)
		})
}

// MiddlewareLogFingerprint is the middleware to log
// incoming HTTP request and browser fingerprint.
func MiddlewareLogFingerprint(next http.Handler) http.Handler {
	log.Info("MiddlewareLogFingerprint: " + FingerprintExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.In(ipMethodURL(r) + fingerprint(r))
			next.ServeHTTP(w, r)
		})
}

// MiddlewareLogFingerprintSafe is similar to MiddlewareLogFingerprints but sanitize the URL.
func MiddlewareLogFingerprintSafe(next http.Handler) http.Handler {
	log.Info("MiddlewareLogFingerprintSafe: " + FingerprintExplanation)

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.In(ipMethodURLSafe(r) + fingerprint(r))
			next.ServeHTTP(w, r)
		})
}

// StartExporter creates and starts the exporter health server
// (Kubernetes health endpoints and Prometheus export server).
func (g *Garcon) StartExporter(expPort int, options ...ProbeOption) (gg.Chain, func(net.Conn, http.ConnState)) {
	return StartExporter(expPort, g.ServerName, options...)
}

// StartExporter creates and starts the exporter health server for Prometheus metrics and liveness/readiness endpoints.
func StartExporter(port int, namespace ServerName, options ...ProbeOption) (gg.Chain, func(net.Conn, http.ConnState)) {
	if port <= 0 {
		log.Info("Disable Prometheus and health endpoints, export port=", port)
		return nil, nil
	}

	prometheus.MustRegister(collectors.NewBuildInfoCollector())
	namespace = namespace.RespectPromNamingRule()
	connState := namespace.ConnState()
	middleware := namespace.MiddlewareExportTrafficMetrics
	chain := gg.NewChain(middleware)

	addr := ":" + strconv.Itoa(port)
	go serveEndpoints(addr, options...)
	log.Info("Prometheus export http://localhost"+addr+
		" namespace="+namespace.String()+" probes=", len(options))

	return chain, connState
}

// WithLivenessProbes adds given liveness probes to the set of probes.
func WithLivenessProbes(probes ...ProbeFunction) ProbeOption {
	return func(h *exporterHandler) {
		h.livenessProbes = append(h.livenessProbes, probes...)
	}
}

// ProbeFunction returns a JSON text explaining the health issue.
// If the health status is OK, returns nothing (an empty text).
type ProbeFunction func() []byte

// WithReadinessProbes adds given readiness probes to the set of probes.
func WithReadinessProbes(probes ...ProbeFunction) ProbeOption {
	return func(h *exporterHandler) {
		h.readinessProbes = append(h.readinessProbes, probes...)
	}
}

type ProbeOption func(*exporterHandler)

func serveEndpoints(addr string, options ...ProbeOption) {
	server := http.Server{
		Addr:              addr,
		Handler:           newExporterHandler(options...),
		TLSConfig:         nil,
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Minute,
		IdleTimeout:       time.Second,
		MaxHeaderBytes:    444, // 444 bytes should be enough
		TLSNextProto:      nil,
		ConnState:         nil,
		ErrorLog:          nil,
		BaseContext:       nil,
		ConnContext:       nil,
	}
	err := server.ListenAndServe()
	log.Panic(err)
}

// newExporterHandler exports the metrics by processing
// the Prometheus requests on the "/metrics" endpoint.
func newExporterHandler(options ...ProbeOption) http.Handler {
	h := &exporterHandler{
		livenessProbes:  []ProbeFunction{},
		readinessProbes: []ProbeFunction{},
	}

	for _, option := range options {
		option(h)
	}

	return h
}

type exporterHandler struct {
	livenessProbes  []ProbeFunction
	readinessProbes []ProbeFunction
}

// ServeHTTP implements http.Handler interface.
func (h *exporterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/metrics":
		promhttp.Handler().ServeHTTP(w, r)
	case "/health":
		handleEnpoint(w, h.livenessProbes)
	case "/ready":
		handleEnpoint(w, append(h.livenessProbes, h.readinessProbes...))
	default:
		log.Warning(ipMethodURLSafe(r) + " on Exporter Server")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"This is the Exporter/Health Server"}`))
	}
}

func handleEnpoint(w http.ResponseWriter, probes []ProbeFunction) {
	for _, p := range probes {
		txt := p()
		if len(txt) != 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write(txt)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func ipMethodURL(r *http.Request) string {
	// double space after "in" is for padding with "out" logs
	return "--> " + r.RemoteAddr + " " + r.Method + " " + r.RequestURI
}

func ipMethodURLSafe(r *http.Request) string {
	return "--> " + r.RemoteAddr + " " + r.Method + " " + gg.Sanitize(r.RequestURI)
}

func ipMethodURLDuration(r *http.Request, statusCode string, d time.Duration) string {
	return statusCode + " " + r.RemoteAddr + " " + r.Method + " " +
		r.RequestURI + " " + d.String()
}

func ipMethodURLDurationSafe(r *http.Request, statusCode string, d time.Duration) string {
	return statusCode + " " + r.RemoteAddr + " " + r.Method + " " +
		gg.Sanitize(r.RequestURI) + " " + d.String()
}

// FingerprintExplanation provides a description of the logged HTTP headers.
const FingerprintExplanation = `
1. Accept-Language, the language preferred by the user. 
2. User-Agent, name and version of the browser and OS. 
3. R=Referer, the website from which the request originated. 
4. A=Accept, the content types the browser prefers. 
5. E=Accept-Encoding, the compression formats the browser supports. 
6. Connection, can be empty, "keep-alive" or "close". 
7. Cache-Control, how the browser is caching data. 
8. URI=Upgrade-Insecure-Requests, the browser can upgrade from HTTP to HTTPS. 
9. Via avoids request loops and identifies protocol capabilities. 
10. Authorization or Cookie (both should not be present at the same time). 
11. DNT (Do Not Track) is being dropped by web browsers.`

// fingerprint logs like logIPMethodURL and also logs the browser fingerprint.
// Attention! fingerprint provides personal data that may identify users.
// To comply with GDPR, the website data owner must have a legitimate reason to do so.
// Before enabling the fingerprinting, the user must understand it
// and give their freely-given informed consent such as the settings change from "no" to "yes".
func fingerprint(r *http.Request) string {
	// double space after "in" is for padding with "out" logs
	line := " " +
		// 1. Accept-Language, the language preferred by the user.
		gg.SafeHeader(r, "Accept-Language") + " " +
		// 2. User-Agent, name and version of the browser and OS.
		gg.SafeHeader(r, "User-Agent") +
		// 3. R=Referer, the website from which the request originated.
		headerTxt(r, "Referer", "R=", "") +
		// 4. A=Accept, the content types the browser prefers.
		headerTxt(r, "Accept", "A=", "") +
		// 5. E=Accept-Encoding, the compression formats the browser supports.
		headerTxt(r, "Accept-Encoding", "E=", "") +
		// 6. Connection, can be empty, "keep-alive" or "close".
		headerTxt(r, "Connection", "", "") +
		// 7. Cache-Control, how the browser is caching data.
		headerTxt(r, "Cache-Control", "", "") +
		// 8. Upgrade-Insecure-Requests, the browser can upgrade from HTTP to HTTPS
		headerTxt(r, "Upgrade-Insecure-Requests", "UIR=", "1") +
		// 9. Via avoids request loops and identifies protocol capabilities
		headerTxt(r, "Via", "Via=", "") +
		// 10. Authorization and Cookie: both should not be present at the same time
		headerTxt(r, "Authorization", "", "") +
		headerTxt(r, "Cookie", "", "")

	// 11, DNT (Do Not Track) is being dropped by web browsers.
	if r.Header.Get("DNT") != "" {
		line += " DNT"
	}

	return line
}

// FingerprintMD provide the browser fingerprint in markdown format.
// Attention: read the .
func FingerprintMD(r *http.Request) string {
	return "\n" + "- **IP**: " + gg.Sanitize(r.RemoteAddr) +
		headerMD(r, "Accept-Language") + // language preferred by the user
		headerMD(r, "User-Agent") + // name and version of browser and OS
		headerMD(r, "Referer") + // URL from which the request originated
		headerMD(r, "Accept") + // content types the browser prefers
		headerMD(r, "Accept-Encoding") + // compression formats the browser supports
		headerMD(r, "Connection") + // can be: empty, "keep-alive" or "close"
		headerMD(r, "Cache-Control") + // how the browser is caching data
		headerMD(r, "DNT") + // "Do Not Track" is being dropped by web standards and browsers
		headerMD(r, "Via") + // avoid request loops and identify protocol capabilities
		headerMD(r, "Authorization") + // Attention: may contain confidential data
		headerMD(r, "Cookie") // Attention: may contain confidential data
}

func headerTxt(r *http.Request, header, key, skip string) string {
	v := gg.SafeHeader(r, header)
	if v == skip {
		return ""
	}
	return " " + key + v
}

func headerMD(r *http.Request, header string) string {
	v := gg.SafeHeader(r, header)
	if v == "" {
		return ""
	}
	return "\n" + "- **" + header + "**: " + v
}

func StatusCodeStr(code int) string {
	// fast path for common codes
	switch code {
	case http.StatusOK:
		return "200" // OK
	case http.StatusNoContent:
		return "204" // No Content

	case http.StatusBadRequest:
		return "400" // Bad Request
	case http.StatusUnauthorized:
		return "401" // Unauthorized
	case http.StatusNotFound:
		return "404" // Not Found

	case http.StatusInternalServerError:
		return "500" // Internal Server Error
	case http.StatusNotImplemented:
		return "501" // Not Implemented
	}

	if false {
		return lessCommonStatusCodes(code)
	}

	return strconv.Itoa(code)
}

func lessCommonStatusCodes(code int) string {
	switch code {
	case http.StatusContinue:
		return "100" // Continue
	case http.StatusSwitchingProtocols:
		return "101" // Switching Protocols
	case http.StatusProcessing:
		return "102" // Processing
	case http.StatusEarlyHints:
		return "103" // Early Hints

	case http.StatusCreated:
		return "201" // Created
	case http.StatusAccepted:
		return "202" // Accepted
	case http.StatusNonAuthoritativeInfo:
		return "203" // Non-Authoritative Information
	case http.StatusResetContent:
		return "205" // Reset Content
	case http.StatusPartialContent:
		return "206" // Partial Content
	case http.StatusMultiStatus:
		return "207" // Multi-Status
	case http.StatusAlreadyReported:
		return "208" // Already Reported
	case http.StatusIMUsed:
		return "226" // IM Used

	case http.StatusMultipleChoices:
		return "300" // Multiple Choices
	case http.StatusMovedPermanently:
		return "301" // Moved Permanently
	case http.StatusFound:
		return "302" // Found
	case http.StatusSeeOther:
		return "303" // See Other
	case http.StatusNotModified:
		return "304" // Not Modified
	case http.StatusUseProxy:
		return "305" // Use Proxy
	case http.StatusTemporaryRedirect:
		return "307" // Temporary Redirect
	case http.StatusPermanentRedirect:
		return "308" // Permanent Redirect

	case http.StatusForbidden:
		return "403" // Forbidden
	case http.StatusPaymentRequired:
		return "402" // Payment Required
	case http.StatusMethodNotAllowed:
		return "405" // Method Not Allowed
	case http.StatusNotAcceptable:
		return "406" // Not Acceptable
	case http.StatusProxyAuthRequired:
		return "407" // Proxy Authentication Required
	case http.StatusRequestTimeout:
		return "408" // Request Timeout
	case http.StatusConflict:
		return "409" // Conflict
	case http.StatusGone:
		return "410" // Gone
	case http.StatusLengthRequired:
		return "411" // Length Required
	case http.StatusPreconditionFailed:
		return "412" // Precondition Failed
	case http.StatusRequestEntityTooLarge:
		return "413" // Request Entity Too Large
	case http.StatusRequestURITooLong:
		return "414" // Request URI Too Long
	case http.StatusUnsupportedMediaType:
		return "415" // Unsupported Media Type
	case http.StatusRequestedRangeNotSatisfiable:
		return "416" // Requested Range Not Satisfiable
	case http.StatusExpectationFailed:
		return "417" // Expectation Failed
	case http.StatusTeapot:
		return "418" // I'm a teapot
	case http.StatusMisdirectedRequest:
		return "421" // Misdirected Request
	case http.StatusUnprocessableEntity:
		return "422" // Unprocessable Entity
	case http.StatusLocked:
		return "423" // Locked
	case http.StatusFailedDependency:
		return "423" // Failed Dependency
	case http.StatusTooEarly:
		return "425" // Too Early
	case http.StatusUpgradeRequired:
		return "426" // Upgrade Required
	case http.StatusPreconditionRequired:
		return "428" // Precondition Required
	case http.StatusTooManyRequests:
		return "429" // Too Many Requests
	case http.StatusRequestHeaderFieldsTooLarge:
		return "431" // Request Header Fields Too Large
	case http.StatusUnavailableForLegalReasons:
		return "451" // Unavailable For Legal Reasons

	case http.StatusBadGateway:
		return "502" // Bad Gateway
	case http.StatusServiceUnavailable:
		return "503" // Service Unavailable
	case http.StatusGatewayTimeout:
		return "504" // Gateway Timeout
	case http.StatusHTTPVersionNotSupported:
		return "505" // HTTP Version Not Supported
	case http.StatusVariantAlsoNegotiates:
		return "506" // Variant Also Negotiates
	case http.StatusInsufficientStorage:
		return "507" // Insufficient Storage
	case http.StatusLoopDetected:
		return "508" // Loop Detected
	case http.StatusNotExtended:
		return "510" // Not Extended
	case http.StatusNetworkAuthenticationRequired:
		return "511" // Network Authentication Required
	}

	return strconv.Itoa(code)
}
