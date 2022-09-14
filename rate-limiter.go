// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/teal-finance/garcon/gg"
)

type ReqLimiter struct {
	gw          Writer
	visitors    map[string]*visitor
	initLimiter *rate.Limiter
	mu          sync.Mutex
}

type visitor struct {
	lastSeen time.Time
	limiter  *rate.Limiter
}

func (g *Garcon) MiddlewareRateLimiter(settings ...int) gg.Middleware {
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
		log.Panic("garcon.MiddlewareRateLimiter() accepts up to two arguments, got", len(settings))
	}

	reqLimiter := NewRateLimiter(g.Writer, maxReqBurst, maxReqPerMinute, g.devMode)
	return reqLimiter.MiddlewareRateLimiter
}

func NewRateLimiter(gw Writer, maxReqBurst, maxReqPerMinute int, devMode bool) ReqLimiter {
	if devMode {
		maxReqBurst *= 2
		maxReqPerMinute *= 2
	}

	ratePerSecond := float64(maxReqPerMinute) / 60

	return ReqLimiter{
		gw:          gw,
		visitors:    make(map[string]*visitor),
		initLimiter: rate.NewLimiter(rate.Limit(ratePerSecond), maxReqBurst),
		mu:          sync.Mutex{},
	}
}

func (rl *ReqLimiter) MiddlewareRateLimiter(next http.Handler) http.Handler {
	log.Infof("MiddlewareRateLimiter burst=%v rate=%.2f/s",
		rl.initLimiter.Burst(), rl.initLimiter.Limit())

	go rl.removeOldVisitors()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			rl.gw.WriteErr(w, r, http.StatusInternalServerError,
				"Cannot split remote_addr=host:port", "remote_addr", r.RemoteAddr)
			log.Out("500", r.RemoteAddr, r.Method, r.RequestURI, "Split host:port ERROR:", err)
			return
		}

		limiter := rl.getVisitor(ip)

		if err := limiter.Wait(r.Context()); err != nil {
			if r.Context().Err() == nil {
				rl.gw.WriteErr(w, r, http.StatusTooManyRequests, "Too Many Requests",
					"advice", "Please contact the team support is this is annoying")
				log.Out("429", r.RemoteAddr, r.Method, r.RequestURI, "ERROR:", err)
			} else {
				log.In("-->", r.RemoteAddr, r.Method, r.RequestURI, "ERROR:", err)
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

// AdaptiveRate continuously adjusts the timing between requests
// to prevent the API responds "429 Too Many Requests".
// AdaptiveRate increases/decreases the rate
// depending on absence/presence of the 429 status code.
type AdaptiveRate struct {
	Name      string
	NextSleep time.Duration
	MinSleep  time.Duration
}

func NewAdaptiveRate(name string, d time.Duration) AdaptiveRate {
	ar := AdaptiveRate{
		Name:      name,
		NextSleep: d * factorInitialNextSleep,
		MinSleep:  d,
	}

	ar.LogStats()

	return ar
}

const (
	factorInitialNextSleep  = 2
	factorIncreaseMinSleep  = 32  // higher, the change is slower
	factorDecreaseMinSleep  = 512 // higher, the change is slower
	factorIncreaseNextSleep = 2   // higher, the change is faster
	factorDecreaseNextSleep = 8   // higher, the change is slower
	maxAlpha                = 16
	printDebug              = false
)

func (ar *AdaptiveRate) adjust(d time.Duration) {
	const fim = factorIncreaseMinSleep - 1
	const fin = factorIncreaseNextSleep - 1
	const fdn = factorDecreaseNextSleep - 1

	if d > ar.NextSleep {
		prevNext := ar.NextSleep
		prevMin := ar.MinSleep
		ar.NextSleep = (ar.NextSleep + fin*d) / factorIncreaseNextSleep
		ar.MinSleep = (d + fim*ar.MinSleep) / factorIncreaseMinSleep
		ar.logIncrease(prevMin, prevNext)
		return
	}

	// gap is used to detect stabilized sleep duration
	gap := ar.NextSleep - ar.MinSleep

	ar.NextSleep = (ar.MinSleep + fdn*ar.NextSleep) / factorDecreaseNextSleep

	// try to reduce slowly the "min sleep time"
	if reduce := ar.MinSleep / factorDecreaseMinSleep; gap < reduce {
		ar.MinSleep -= reduce
		ar.logDecrease(reduce)
	}
}

func (ar *AdaptiveRate) Get(symbol, url string, msg any, maxBytes ...int) error {
	var err error
	d := ar.NextSleep
	for try, status := 1, http.StatusTooManyRequests; (try < 88) && (status == http.StatusTooManyRequests); try++ {
		if try > 1 {
			previous := d
			alpha := int64(maxAlpha * ar.MinSleep / d)
			d *= time.Duration(try)
			d += time.Duration(alpha) * ar.MinSleep
			log.Infof("%s Get %s #%d sleep=%s (+%s) alpha=%d n=%s min=%s",
				ar.Name, symbol, try, d, d-previous, alpha, ar.NextSleep, ar.MinSleep)
		}
		time.Sleep(d)
		status, err = ar.get(symbol, url, msg, maxBytes...)
	}

	ar.adjust(d)

	return err
}

func (ar *AdaptiveRate) get(symbol, url string, msg any, maxBytes ...int) (int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("GET %s %s: %w", ar.Name, symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return resp.StatusCode, errors.New("Too Many Requests " + symbol)
	}

	if err = gg.DecodeJSONResponse(resp, msg, maxBytes...); err != nil {
		return resp.StatusCode, fmt.Errorf("decode book %s: %w", symbol, err)
	}

	return resp.StatusCode, nil
}

func (ar *AdaptiveRate) LogStats() {
	log.Infof("%s Adjusted sleep durations: min=%s next=%s",
		ar.Name, ar.MinSleep, ar.NextSleep)
}

func (ar *AdaptiveRate) logIncrease(prevMin, prevNext time.Duration) {
	if printDebug {
		log.Debugf("%s Increase MinSleep=%s (+%s) next=%s (+%s)",
			ar.Name, ar.MinSleep, ar.MinSleep-prevMin, ar.NextSleep, ar.NextSleep-prevNext)
	}
}

func (ar *AdaptiveRate) logDecrease(reduce time.Duration) {
	if printDebug {
		log.Debugf("%s Decrease MinSleep=%s (-%s) next=%s",
			ar.Name, ar.MinSleep, reduce, ar.NextSleep)
	}
}
