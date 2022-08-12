// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ReqLimiter struct {
	visitors    map[string]*visitor
	initLimiter *rate.Limiter
	mu          sync.Mutex
	gw          Writer
}

type visitor struct {
	lastSeen time.Time
	limiter  *rate.Limiter
}

func (g *Garcon) RateLimiter(settings ...int) Middleware {
	var maxReqBurst, maxReqPerMinute int

	switch len(settings) {
	case 0: // default settings
		maxReqBurst = 20
		maxReqPerMinute = 4 * maxReqBurst
	case 1:
		maxReqBurst = settings[0]
		maxReqPerMinute = 4 * maxReqBurst
	case 2:
		maxReqBurst = settings[0]
		maxReqPerMinute = settings[1]
	default:
		log.Panic("garcon.RateLimiter() accept up to two arguments, got ", len(settings))
	}

	reqLimiter := NewRateLimiter(maxReqBurst, maxReqPerMinute, g.devMode, g.Writer)
	return reqLimiter.LimitRate
}

func NewRateLimiter(maxReqBurst, maxReqPerMinute int, devMode bool, gw Writer) ReqLimiter {
	if devMode {
		maxReqBurst *= 2
		maxReqPerMinute *= 2
	}

	ratePerSecond := float64(maxReqPerMinute) / 60

	return ReqLimiter{
		visitors:    make(map[string]*visitor),
		initLimiter: rate.NewLimiter(rate.Limit(ratePerSecond), maxReqBurst),
		mu:          sync.Mutex{},
		gw:          gw,
	}
}

func (rl *ReqLimiter) LimitRate(next http.Handler) http.Handler {
	log.Printf("INF Middleware RateLimiter: burst=%v rate=%v/s",
		rl.initLimiter.Burst(), rl.initLimiter.Limit())

	go rl.removeOldVisitors()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			rl.gw.WriteErr(w, r, http.StatusInternalServerError,
				"Cannot split addr=host:port", "addr", r.RemoteAddr)
			log.Print("ERR in  ", r.RemoteAddr, " ", r.Method, " ", r.RequestURI, " SplitHostPort ", err)
			return
		}

		limiter := rl.getVisitor(ip)

		if err := limiter.Wait(r.Context()); err != nil {
			if r.Context().Err() == nil {
				rl.gw.WriteErr(w, r, http.StatusTooManyRequests, "Too Many Requests",
					"advice", "Please contact the team support is this is annoying")
				log.Print("WRN ", r.RemoteAddr, " ", r.Method, " ", r.RequestURI, "TooManyRequests ", err)
			} else {
				log.Print("WRM ", r.RemoteAddr, " ", r.Method, " ", r.RequestURI, " ", err)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *ReqLimiter) removeOldVisitors() {
	for ; true; <-time.NewTicker(1 * time.Minute).C {
		rl.mu.Lock()

		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}

		rl.mu.Unlock()
	}
}

func (rl *ReqLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.visitors[ip]
	if !ok {
		v = &visitor{
			limiter:  rl.initLimiter,
			lastSeen: time.Time{},
		}
		rl.visitors[ip] = v
	}

	v.lastSeen = time.Now()

	return v.limiter
}
