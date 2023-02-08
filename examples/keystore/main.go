// SPDX-License-Identifier: CC0-1.0
// Creative Commons Zero v1.0 Universal - No Rights Reserved
// <https://creativecommons.org/publicdomain/zero/1.0>
//
// To the extent possible under law, the Teal.Finance/Garcon contributors
// have waived all copyright and related/neighboring rights to this
// file "keystore/main.go" to be freely used without any restriction.

package main

import (
	"flag"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/emo"
	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/gg"
)

var log = emo.NewZone("app")

// Garcon settings
const (
	burst, perMinute = 2, 5
)

var (
	mainPort  = gg.EnvInt("MAIN_PORT", 8088)
	pprofPort = gg.EnvInt("PPROF_PORT", 8098)
	expPort   = gg.EnvInt("EXP_PORT", 9098)
)

func main() {
	prod := flag.Bool("prod", false, "Use settings for production")
	flag.Parse()

	addr := "http://localhost:" + strconv.Itoa(mainPort)

	g := garcon.New(
		garcon.WithServerName("KeyStore"),
		garcon.WithURLs(addr),
		garcon.WithDocURL("/doc"),
		garcon.WithPProf(pprofPort),
		garcon.WithDev(!*prod),
	)

	middleware, connState := g.StartExporter(expPort,
		garcon.WithLivenessProbes(func() []byte { return nil }),
		garcon.WithLivenessProbes(func() []byte { return nil }),
		garcon.WithLivenessProbes(func() []byte { return nil }),
		garcon.WithReadinessProbes(func() []byte { return nil }),
		garcon.WithReadinessProbes(func() []byte { return []byte("fail") }))

	middleware = middleware.Append(
		g.MiddlewareRejectUnprintableURI(),
		g.MiddlewareLogRequest(),
		g.MiddlewareRateLimiter(burst, perMinute),
		g.MiddlewareServerHeader("KeyStore"),
		g.MiddlewareCORS())

	// handles both REST API and static web files
	r := router(g)
	h := middleware.Then(r)

	server := garcon.Server(h, mainPort, connState)

	log.Init("-------------- Open http://localhost" + server.Addr + " --------------")
	log.Fatal(garcon.ListenAndServe(&server))
}

// router creates the mapping between the endpoints and the router functions.
func router(g *garcon.Garcon) http.Handler {
	r := chi.NewRouter()

	// Static website files
	ws := garcon.StaticWebServer{Dir: "examples/www", Writer: g.Writer}
	r.Get("/", ws.ServeFile("keystore/index.html", "text/html; charset=utf-8"))
	r.Get("/favicon.ico", ws.ServeFile("keystore/favicon.ico", "image/x-icon"))

	// API
	db := &db{
		g:        g,
		KeysByIP: map[string]Keys{},
	}
	r.Get("/keys", db.list)
	r.Post("/keys", db.post)
	r.Delete("/keys", db.delete)

	// Other endpoints
	r.NotFound(g.Writer.InvalidPath)

	return r
}

type db struct {
	g        *garcon.Garcon
	KeysByIP map[string]Keys
}

type Keys map[string]string

func (db *db) list(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		db.g.Writer.WriteErr(w, r, http.StatusInternalServerError,
			"Cannot split addr=host:port", "addr", r.RemoteAddr)
		log.Error("Cannot split addr=host:port", err)
		return
	}

	if ip != "127.0.0.1" {
		db.g.Writer.WriteErr(w, r, http.StatusForbidden,
			"GET is forbidden for IP="+r.RemoteAddr+" (only 127.0.0.1)",
			"IP", r.RemoteAddr)
		return
	}

	keyNames := make(map[string][]string, len(db.KeysByIP))
	for h, keys := range db.KeysByIP {
		keyNames[h] = make([]string, len(keys))
		for name := range keys {
			keyNames[h] = append(keyNames[h], name)
		}
	}

	db.g.Writer.WriteOK(w, keyNames)
}

// post writes to DB the keys but also responds keys values from DB
// when the key value is missing in the request body.
func (db *db) post(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		db.g.Writer.WriteErr(w, r, http.StatusInternalServerError,
			"Cannot split addr=host:port", "addr", r.RemoteAddr)
		log.Error("Cannot split addr=host:port", err)
	}

	values, err := parseForm(r)
	if err != nil {
		db.g.Writer.WriteErr(w, r, http.StatusInternalServerError,
			"Cannot parse the webform (request body)")
		log.Error("Cannot parse the webform", err)
	}

	if values == nil {
		db.g.Writer.WriteErr(w, r, http.StatusBadRequest,
			"Missing webform (request body)")
	}

	keys, ok := db.KeysByIP[ip]
	if !ok {
		keys = Keys{}
	}

	result := make(Keys, len(values))
	for name, vals := range values {
		{
			v, ok := keys[name]
			if ok && vals[0] == "" {
				result[name] = v
			}
		}

		for _, v := range vals {
			if v != "" {
				keys[name] = v
			}
		}
	}

	db.KeysByIP[ip] = keys

	db.g.Writer.WriteOK(w, result)
}

func (db *db) delete(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		db.g.Writer.WriteErr(w, r, http.StatusInternalServerError,
			"Cannot split addr=host:port", "addr", r.RemoteAddr)
		log.Error("Cannot split addr=host:port", err)
	}

	values, err := parseForm(r)
	if err != nil {
		db.g.Writer.WriteErr(w, r, http.StatusInternalServerError,
			"Cannot parse the webform (request body)")
		log.Error("Cannot parse the webform", err)
	}

	if len(values) == 0 {
		delete(db.KeysByIP, ip)
		return
	}

	keys := db.KeysByIP[ip]
	for name := range values {
		delete(keys, name)
	}

	if len(keys) == 0 {
		delete(db.KeysByIP, ip)
	} else {
		db.KeysByIP[ip] = keys
	}
}

func parseForm(r *http.Request) (url.Values, error) {
	if r.Body == nil {
		return nil, nil
	}

	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	return url.ParseQuery(string(bytes))
}
