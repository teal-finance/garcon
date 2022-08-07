// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

// Package garcon is a server for API and static website
// including middlewares to manage rate-limit, Cookies, JWT,
// CORS, OPA, web traffic, Prometheus export and PProf.
package garcon

import (
	"encoding/hex"
	"log"
)

type Option func(*Garcon)

func WithNamespace(namespace string) Option {
	return func(g *Garcon) {
		oldNS := g.Namespace
		newNS := NewNamespace(namespace)
		if oldNS != "" && g.Namespace != newNS {
			log.Panicf("WithProm(namespace=%q) overrides namespace=%q", newNS, oldNS)
		}
		g.Namespace = newNS
	}
}

func WithDocURL(docURL string) Option {
	return func(g *Garcon) {
		g.docURL = docURL
	}
}

func WithDev(enable ...bool) Option {
	devMode := true
	if len(enable) > 0 {
		devMode = enable[0]

		if len(enable) >= 2 {
			log.Panic("garcon.WithDev() must be called with zero or one argument")
		}
	}

	return func(g *Garcon) {
		g.devMode = devMode
	}
}

func WithPProf(port int) Option {
	return func(g *Garcon) {
		g.pprofPort = port
	}
}

func WithProm(port int, namespace string) Option {
	setNamespace := WithNamespace(namespace)

	return func(g *Garcon) {
		g.expPort = port
		setNamespace(g)
	}
}

func WithReqLogs(verbosity ...int) Option {
	v := 1
	if len(verbosity) > 0 {
		if len(verbosity) >= 2 {
			log.Panic("garcon.WithReqLogs() must be called with zero or one argument")
		}
		v = verbosity[0]
		if v < 0 || v > 2 {
			log.Panicf("garcon.WithReqLogs(verbosity=%v) currently accepts values [0, 1, 2] only", v)
		}
	}

	return func(g *Garcon) { g.reqLogVerbosity = v }
}

func (g *Garcon) RequestLogger() Middleware {
	switch g.reqLogVerbosity {
	case 1:
		return LogRequest
	case 2:
		return LogRequestFingerprint
	}
	return nil // do not log incoming HTTP requests
}

func WithLimiter(values ...int) Option {
	var burst, perMinute int

	switch len(values) {
	case 0:
		burst = 20
		perMinute = 4 * burst
	case 1:
		burst = values[0]
		perMinute = 4 * burst
	case 2:
		burst = values[0]
		perMinute = values[1]
	default:
		log.Panic("garcon.WithLimiter() must be called with less than three arguments")
	}

	return func(g *Garcon) {
		g.reqBurst = burst
		g.reqMinute = perMinute
	}
}

func (g *Garcon) RateLimiter() Middleware {
	if g.reqMinute <= 0 {
		return nil
	}
	reqLimiter := NewReqLimiter(g.reqBurst, g.reqMinute, g.devMode, g.Writer)
	return reqLimiter.LimitRate
}

func WithServerHeader(program string) Option {
	return func(g *Garcon) {
		g.version = Version(program)
	}
}

func (g *Garcon) ServerSetter() Middleware {
	if g.version == "" {
		return nil
	}
	return ServerHeader(g.version)
}

func WithURLs(addresses ...string) Option {
	return func(g *Garcon) {
		g.urls = ParseURLs(addresses)
	}
}

func (g *Garcon) CORSHandler() Middleware {
	if len(g.origins) == 0 {
		return nil
	}
	return CORSHandler(g.origins, g.devMode)
}

func WithOPA(opaFilenames ...string) Option {
	return func(g *Garcon) {
		g.opaFilenames = opaFilenames
	}
}

// OPAHandler creates the middleware for Authentication rules (Open Policy Agent)
func (g *Garcon) OPAHandler() Middleware {
	if len(g.opaFilenames) == 0 {
		return nil
	}
	policy, err := NewPolicy(g.opaFilenames, g.Writer)
	if err != nil {
		log.Panic("WithOPA: cannot create OPA Policy: ", err)
	}
	return policy.AuthOPA
}

// WithJWT requires WithURLs() to set the Cookie name, secure, domain and path.
// WithJWT is not compatible with WithTkn: use only one of them.
func WithJWT(secretKeyHex string, planPerm ...any) Option {
	key, err := hex.DecodeString(secretKeyHex)
	if err != nil {
		log.Panic("WithJWT: cannot decode the HMAC-SHA256 key, please provide hexadecimal format (64 characters) ", err)
	}
	if len(key) != 32 {
		log.Panic("WithJWT: want HMAC-SHA256 key containing 32 bytes, but got ", len(key))
	}

	return func(g *Garcon) {
		g.secretKey = key
		g.checkerCfg = planPerm
	}
}

// WithIncorruptible uses cookies based on fast and tiny token.
// WithIncorruptible requires WithURLs() to set the Cookie secure, domain and path.
// WithIncorruptible is not compatible with WithJWT: use only one of the two.
func WithIncorruptible(secretKeyHex string, maxAge int, setIP bool) Option {
	key, err := hex.DecodeString(secretKeyHex)
	if err != nil {
		log.Panic("WithIncorruptible: cannot decode the 128-bit AES key, please provide hexadecimal format (32 characters)")
	}
	if len(key) < 16 {
		log.Panic("WithIncorruptible: want 128-bit AES key containing 16 bytes, but got ", len(key))
	}

	const cookieName = ""

	return func(g *Garcon) {
		g.secretKey = key
		g.checkerCfg = []any{cookieName, maxAge, setIP}
	}
}
