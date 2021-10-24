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

package limiter

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/teal-finance/garcon/reserr"
	"golang.org/x/time/rate"
)

type ReqLimiter struct {
	visitors    map[string]*visitor
	initLimiter *rate.Limiter
	mu          sync.Mutex
	resErr      reserr.ResErr
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func New(maxReqBurst, maxReqPerMinute int, devMode bool, resErr reserr.ResErr) ReqLimiter {
	if devMode {
		maxReqBurst *= 10
		maxReqPerMinute *= 10
	}

	ratePerSecond := float64(maxReqPerMinute) / 60

	return ReqLimiter{
		visitors:    make(map[string]*visitor),
		initLimiter: rate.NewLimiter(rate.Limit(ratePerSecond), maxReqBurst),
		mu:          sync.Mutex{},
		resErr:      resErr,
	}
}

// LogRequests logs the incoming HTTP requests.
func LogRequests(next http.Handler) http.Handler {
	log.Print("Middleware logger: log requested URLs and remote addresses")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Printf("in  %v %v %v", r.Method, r.RemoteAddr, r.RequestURI)
			next.ServeHTTP(w, r)
		})
}

func (rl *ReqLimiter) Limit(next http.Handler) http.Handler {
	log.Printf("Middleware RateLimiter: burst=%v rate=%v/s",
		rl.initLimiter.Burst(), rl.initLimiter.Limit())

	go rl.removeOldVisitors()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			rl.resErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #3")
			log.Printf("in  %v %v %v - Error SplitHostPort %v", r.Method, r.RemoteAddr, r.RequestURI, err)

			return
		}

		limiter := rl.getVisitor(ip)

		log.Printf("in  %v %v %v B=%v %v/m", r.Method, r.RemoteAddr, r.RequestURI,
			limiter.Burst(), int(60*limiter.Limit()))

		if err := limiter.Wait(r.Context()); err != nil {
			if r.Context().Err() == nil {
				rl.resErr.Write(w, r, http.StatusTooManyRequests, "Too Many Requests")
				log.Printf("rej %v %v %v TooManyRequests %v", r.Method, r.RemoteAddr, r.RequestURI, err)
			} else {
				log.Printf("XXX %v %v %v %v", r.Method, r.RemoteAddr, r.RequestURI, err)
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
