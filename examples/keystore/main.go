// SPDX-License-Identifier: CC0-1.0+
// Creative Commons Zero v1.0 Universal - No Rights Reserved
//
// To the extent possible under law, the Teal.Finance contributors
// have waived all copyright and related/neighboring rights to this
// file "keystore/main.go" to be freely used without any restriction.
// See <https://creativecommons.org/publicdomain/zero/1.0>

package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/webserver"
)

// Garcon settings
const (
	mainPort, pprofPort, expPort = 8080, 8093, 9093
	burst, perMinute             = 2, 5
)

func main() {
	prod := flag.Bool("prod", false, "Use settings for production")
	flag.Parse()

	addr := "http://localhost:" + strconv.Itoa(mainPort)

	g, err := garcon.New(
		garcon.WithURLs(addr),
		garcon.WithDocURL("/doc"),
		garcon.WithServerHeader("KeyStore-v0"),
		garcon.WithReqLogs(),
		garcon.WithLimiter(burst, perMinute),
		garcon.WithPProf(pprofPort),
		garcon.WithProm(expPort, "keystore"),
		garcon.WithDev(!*prod),
	)
	if err != nil {
		log.Fatal(err)
	}

	// handles both REST API and static web files
	h := handler(g)

	err = g.Run(h, mainPort)
	log.Fatal(err)
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(g *garcon.Garcon) http.Handler {
	r := chi.NewRouter()

	// Static website files
	ws := webserver.WebServer{Dir: "examples/www", ResErr: g.ResErr}
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
	r.NotFound(g.ResErr.InvalidPath)

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
		db.g.ResErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #3")
		log.Print("WARN ", err)
		return
	}

	if ip != "127.0.0.1" {
		db.g.ResErr.Write(w, r, http.StatusForbidden, "GET is forbidden for IP="+r.RemoteAddr+" (only 127.0.0.1)")
		return
	}

	keyNames := map[string][]string{}
	for h, keys := range db.KeysByIP {
		keyNames[h] = make([]string, len(keys))
		for name := range keys {
			keyNames[h] = append(keyNames[h], name)
		}
	}

	bytes, err := json.Marshal(keyNames)
	if err != nil {
		db.g.ResErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #4")
		log.Print("WARN ", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(bytes)
}

// post writes to DB the keys but also responds keys values from DB
// when the key value is missing in the request body.
func (db *db) post(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		db.g.ResErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #3")
		log.Print("WARN ", err)
	}

	values, err := parseForm(r)
	if err != nil {
		db.g.ResErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #5")
		log.Print("WARN ", err)
	}

	if values == nil {
		db.g.ResErr.Write(w, r, http.StatusBadRequest, "Bad request missing form body")
	}

	keys, ok := db.KeysByIP[ip]
	if !ok {
		keys = Keys{}
	}

	result := Keys{}
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

	bytes, err := json.Marshal(result)
	if err != nil {
		db.g.ResErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #4")
		log.Print("WARN ", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(bytes)
}

func (db *db) delete(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		db.g.ResErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #3")
		log.Print("WARN ", err)
	}

	values, err := parseForm(r)
	if err != nil {
		db.g.ResErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #5")
		log.Print("WARN ", err)
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
