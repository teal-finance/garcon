# Teal.Finance/Garcon

![logo](examples/www/images/garcon.png) | <big>Opinionated boilerplate all-in-one HTTP server with rate-limiter, Cookies, JWT, CORS, OPA, web traffic, Prometheus export, PProf… for API and static website.</big>
-|-

This library is used by
[Rainbow](https://github.com/teal-finance/rainbow)
and other internal projects at Teal.Finance.

Please propose a PR to add here your project that also uses Garcon.

## Features

Garcon includes the following middlewares:

* Logging of incoming requests ;
* Rate limiter to prevent requests flooding ;
* JWT management using HttpOnly cookie or Authorization header ;
* Cross-Origin Resource Sharing (CORS) ;
* Authentication rules based on Datalog/Rego files using [Open Policy Agent](https://www.openpolicyagent.org) ;
* Web traffic metrics.

Garcon also provides the following features:

* HTTP/REST server for API endpoints (compatible with any Go-standard HTTP handlers) ;
* File server intended for static web files supporting Brotli and AVIF data ;
* Metrics server exporting data to Prometheus (or other compatible monitoring tool) ;
* PProf server for debugging purpose ;
* Error response in JSON format ;
* Chained middlewares (fork of [justinas/alice](https://github.com/justinas/alice)).

## CPU profiling

Moreover, Garcon provides a helper feature `defer ProbeCPU.Stop()`
to investigate CPU consumption issues
thanks to <https://github.com/pkg/profile>.

In you code, add `defer ProbeCPU.Stop()` that will write the `cpu.pprof` file.

```go
import "github.com/teal-finance/garcon/pprof"

func myFunctionConsummingLotsOfCPU() {
    defer pprof.ProbeCPU.Stop()

    // ... lots of sub-functions
}
```

Install `pprof` and browse your `cpu.pprof` file:

```sh
cd ~/go
go get -u github.com/google/pprof
cd -
pprof -http=: cpu.pprof
```

## Examples

See also a complete real example in the repo
[github.com/teal-finance/rainbow](https://github.com/teal-finance/rainbow/blob/main/cmd/server/main.go).

## High-level

The following code uses the high-level function `Garcon.RunServer()`.

```go
package main

import (
    "log"

    "github.com/teal-finance/garcon"
)

func main() {
    g, _ := garcon.New(
        garcon.WithOrigins("localhost:8080"),
        garcon.WithDocURL("/doc"), // ==> URL = localhost:8080/doc
        garcon.WithServerHeader("MyBackendName-1.2.0"),
        garcon.WithJWT(hmacSHA256, "FreePlan", 10, "PremiumPlan", 100),
        garcon.WithOPA("auth.rego"),
        garcon.WithLimiter(10, 30),
        garcon.WithPProf(8093),
        garcon.WithProm(9093),
        garcon.WithDev(),
    )

    h := handler(g.ResErr, g.JWTChecker)

    log.Fatal(g.Run(h, 8080))
}
```

### 1. Run the [high-level example](examples/high-level/main.go)

```sh
go build -race ./examples/high-level && ./high-level
```

```log
2021/11/17 15:37:23 Enable PProf endpoints: http://localhost:8093/debug/pprof
2021/11/17 15:37:23 Prometheus export http://localhost:9093
2021/11/17 15:37:23 CORS: Set origin prefixes: [http://localhost:8080 http://localhost: http://192.168.1.]
2021/11/17 15:37:23 CORS: Methods=[GET] Headers=[Origin Accept Content-Type Authorization Cookie] Credentials=true MaxAge=86400
2021/11/17 15:37:23 JWT not required for dev. origins: [http://localhost:8080 http://localhost: http://192.168.1.]
2021/11/17 15:37:23 Create cookie plan=FreePlan domain=localhost secure=false jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJGcmVlUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk1ODQzfQ.BemzR-4vTAdcV_wmshNkF5-R0-cO4rFN09BfesNhjCc
2021/11/17 15:37:23 Create cookie plan=PremiumPlan domain=localhost secure=false jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJQcmVtaXVtUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk1ODQzfQ.LxGc8xJ03jIvzxiAVi1IsnOwglpPNz2RwxpUtpOYqjk
2021/11/17 15:37:23 Middleware response HTTP header: Set Server MyBackendName-1.2.0
2021/11/17 15:37:23 Middleware RateLimiter + Logger: burst=100 rate=5/s fingerprints:
1. Accept-Language, the language preferred by the user. 
2. User-Agent, name and version of the browser and OS. 
3. R=Referer, the website from which the request originated. 
4. A=Accept, the content types the browser prefers. 
5. E=Accept-Encoding, the compression formats the browser supports. 
6. Connection, can be empty, "keep-alive" or "close". 
7. DNT (Do Not Track) can be used by Firefox (dropped by web standards). 
8. Cache-Control, how the browser is caching data. 
9. Authorization and/or Cookie content.
2021/11/17 15:37:23 Server listening on http://localhost:8080
```

### 2. Embedded PProf server

Visit the PProf server at <http://localhost:8093/debug/pprof> providing the following endpoints:

* <http://localhost:8093/debug/pprof/cmdline> - Command line arguments
* <http://localhost:8093/debug/pprof/profile> - CPU profile
* <http://localhost:8093/debug/pprof/allocs> - Memory allocations from start
* <http://localhost:8093/debug/pprof/heap> - Current memory allocations
* <http://localhost:8093/debug/pprof/trace> - Current program trace
* <http://localhost:8093/debug/pprof/goroutine> - Traces of all current threads (goroutines)
* <http://localhost:8093/debug/pprof/block> - Traces of blocking threads
* <http://localhost:8093/debug/pprof/mutex> - Traces of threads with contended mutex
* <http://localhost:8093/debug/pprof/threadcreate> - Traces of threads creating a new thread

PProf is easy to use with `curl` or `wget`:

```sh
( cd ~ ; go get -u github.com/google/pprof )

curl http://localhost:8093/debug/pprof/allocs > allocs.pprof
pprof -http=: allocs.pprof

wget http://localhost:8093/debug/pprof/heap
pprof -http=: heap

wget http://localhost:8093/debug/pprof/goroutine
pprof -http=: goroutine
```

See the [PProf post](https://go.dev/blog/pprof) (2013) for further explanations.

### 3. Embedded metrics server

The export port <http://localhost:9093/metrics> is for the monitoring tools like Prometheus.

### 4. Static website server

The [high-level example](examples/high-level/main.go) is running.

Open <http://localhost:8080> with your browser, and play with the API endpoints.

The resources and API endpoints are protected with a HttpOnly cookie.
The [high-level example](examples/high-level/main.go) sets the cookie to browsers visiting the `index.html`.

```go
func handler(resErr reserr.ResErr, jc *jwtperm.Checker) http.Handler {
    r := chi.NewRouter()

    // Static website files
    ws := webserver.WebServer{Dir: "examples/www", ResErr: resErr}
    r.With(jc.SetCookie).Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
    r.With(jc.SetCookie).Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
    r.With(jc.ChkCookie).Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    r.With(jc.ChkCookie).Get("/css/*", ws.ServeDir("text/css; charset=utf-8"))
    r.With(jc.ChkCookie).Get("/images/*", ws.ServeImages())

    // API
    r.With(jc.ChkJWT).Get("/api/v1/items", items)
    r.With(jc.ChkJWT).Get("/api/v1/ducks", resErr.NotImplemented)

    // Other endpoints
    r.NotFound(resErr.InvalidPath)

    return r
}
```

### 5. Enable Authentication

Restart again the [high-level example](examples/high-level/main.go) with authentication enabled.

Attention, in this example we use two redundant middlewares: `jwtperm` + `opa` using the same JWT.
This is just an example, do not be confused.

```sh
go build -race ./examples/high-level && ./high-level -auth
```

```log
2021/11/17 15:47:08 Prometheus export http://localhost:9093
2021/11/17 15:47:08 CORS: Set origin prefixes: [http://localhost:8080 http://localhost: http://192.168.1.]
2021/11/17 15:47:08 CORS: Methods=[GET] Headers=[Origin Accept Content-Type Authorization Cookie] Credentials=true MaxAge=86400
2021/11/17 15:47:08 JWT not required for dev. origins: [http://localhost:8080 http://localhost: http://192.168.1.]
2021/11/17 15:47:08 Enable PProf endpoints: http://localhost:8093/debug/pprof
2021/11/17 15:47:08 Create cookie plan=FreePlan domain=localhost secure=false jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJGcmVlUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk2NDI4fQ.MyWlTaCTOeAJ-fiAPCqhvYHXpH7Lj6GzbHEK-4CZY5I
2021/11/17 15:47:08 Create cookie plan=PremiumPlan domain=localhost secure=false jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJQcmVtaXVtUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk2NDI4fQ.KjHERRLe8lrMQoZXt_TqwxZnL4LgELIECmuIlXqHiEk
2021/11/17 15:47:08 OPA: load "examples/sample-auth.rego"
2021/11/17 15:47:08 Middleware OPA: map[sample-auth.rego:package auth

default allow = false
tokens := {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJGcmVlUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk1OTU2fQ.45Ku3S7ljXKtrbxwg_sAJam12RMHenC2GYlAa-nXcgo"} { true }
allow = true { __local0__ = input.token; data.auth.tokens[__local0__] }]
2021/11/17 15:47:08 Middleware response HTTP header: Set Server MyBackendName-1.2.0
2021/11/17 15:47:08 Middleware RateLimiter + Logger: burst=100 rate=5/s fingerprints:
1. Accept-Language, the language preferred by the user. 
2. User-Agent, name and version of the browser and OS. 
3. R=Referer, the website from which the request originated. 
4. A=Accept, the content types the browser prefers. 
5. E=Accept-Encoding, the compression formats the browser supports. 
6. Connection, can be empty, "keep-alive" or "close". 
7. DNT (Do Not Track) can be used by Firefox (dropped by web standards). 
8. Cache-Control, how the browser is caching data. 
9. Authorization and/or Cookie content.
2021/11/17 15:47:08 Server listening on http://localhost:8080
```

### 6. Default HTTP request headers

Test the API with `curl`:

```sh
curl -D - http://localhost:8080/api/v1/items
```

```yaml
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Server: MyBackendName-1.2.0
Vary: Origin
X-Content-Type-Options: nosniff
Date: Wed, 17 Nov 2021 14:47:44 GMT
Content-Length: 84

{"error":"Unauthorized",
"path":"/api/v1/items",
"doc":"http://localhost:8080/doc"}
```

The corresponding garcon logs:

```log
2021/11/17 15:47:44 in  127.0.0.1:35246 GET /api/v1/items  curl/7.79.1 A=*/*
[cors] 2021/11/17 15:47:44 Handler: Actual request
[cors] 2021/11/17 15:47:44   Actual request no headers added: missing origin
2021/11/17 15:47:44 OPA unauthorize 127.0.0.1:35246 /api/v1/items
2021/11/17 15:47:44 out 127.0.0.1:35246 GET /api/v1/items 1.576138ms c=1 a=1 i=0 h=0
```

The CORS logs can be disabled by passing `debug=false` in `cors.Handler(origins, false)`.

The values `c=1 a=1 i=0 h=0` measure the web traffic:

* `c` for the current number of HTTP connections (gauge)
* `a` for the accumulated HTTP connections that have been in StateActive (counter)
* `i` for the accumulated HTTP connections that have been in StateIdle (counter)
* `h` for the accumulated HTTP connections that have been in StateHijacked (counter)

### 7. With Authorization header

```sh
curl -D - http://localhost:8080/api/v1/items -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJGcmVlUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk1OTU2fQ.45Ku3S7ljXKtrbxwg_sAJam12RMHenC2GYlAa-nXcgo'
```

```yaml
HTTP/1.1 200 OK
Content-Type: application/json
Server: MyBackendName-1.2.0
Vary: Origin
Date: Wed, 17 Nov 2021 14:48:51 GMT
Content-Length: 25

["item1","item2","item3"]
```

The corresponding garcon logs:

```log
2021/11/17 15:48:51 in  127.0.0.1:35250 GET /api/v1/items  curl/7.79.1 A=*/* Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJGcmVlUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk1OTU2fQ.45Ku3S7ljXKtrbxwg_sAJam12RMHenC2GYlAa-nXcgo
[cors] 2021/11/17 15:48:51 Handler: Actual request
[cors] 2021/11/17 15:48:51   Actual request no headers added: missing origin
2021/11/17 15:48:51 Authorization header has JWT: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJGcmVlUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk1OTU2fQ.45Ku3S7ljXKtrbxwg_sAJam12RMHenC2GYlAa-nXcgo
2021/11/17 15:48:51 JWT Claims: &{FreePlan  { 1668695956 invalid cookie 0  0 }}
2021/11/17 15:48:51 JWT has the FreePlan Namespace
2021/11/17 15:48:51 JWT Permission: {10}
2021/11/17 15:48:51 out 127.0.0.1:35250 GET /api/v1/items 1.625788ms c=2 a=2 i=1 h=0
```

## Low-level

**WARNING: This chapter is outdated!**

See the [low-level example](examples/low-level/main.go).

The following code is a bit different to the stuff done
by the high-level function `Garcon.Run()` presented in the previous chapter.
The following code is intended to show
Garcon can be customized to meet your specific requirements.

```go
package main

import (
    "log"
    "net"
    "net/http"
    "time"

    "github.com/go-chi/chi"
    "github.com/teal-finance/garcon"
    "github.com/teal-finance/garcon/chain"
    "github.com/teal-finance/garcon/cors"
    "github.com/teal-finance/garcon/limiter"
    "github.com/teal-finance/garcon/metrics"
    "github.com/teal-finance/garcon/opa"
    "github.com/teal-finance/garcon/pprof"
    "github.com/teal-finance/garcon/reserr"
    "github.com/teal-finance/garcon/webserver"
)

// Garcon settings
const apiDoc = "https://my-dns.co/doc"
const allowedProdOrigin = "https://my-dns.co"
const allowedDevOrigins = "http://localhost:  http://192.168.1."
const serverHeader = "MyBackendName-1.2.0"
const authCfg = "examples/sample-auth.rego"
const pprofPort = 8093
const expPort = 9093
const burst, reqMinute = 10, 30
const devMode = true

func main() {
    if devMode {
        // the following line collects the CPU-profile and writes it in the file "cpu.pprof"
        defer pprof.ProbeCPU().Stop()
    }

    pprof.StartServer(pprofPort)

    // Uniformize error responses with API doc
    resErr := reserr.New(apiDoc)

    middlewares, connState := setMiddlewares(resErr)

    // Handles both REST API and static web files
    h := handler(resErr)
    h = middlewares.Then(h)

    runServer(h, connState)
}

func setMiddlewares(resErr reserr.ResErr) (middlewares chain.Chain, connState func(net.Conn, http.ConnState)) {
    // Start a metrics server in background if export port > 0.
    // The metrics server is for use with Prometheus or another compatible monitoring tool.
    metrics := metrics.Metrics{}
    middlewares, connState = metrics.StartServer(expPort, devMode)

    // Limit the input request rate per IP
    reqLimiter := limiter.New(burst, reqMinute, devMode, resErr)

    corsConfig := allowedProdOrigin
    if devMode {
        corsConfig += " " + allowedDevOrigins
    }

    allowedOrigins := garcon.SplitClean(corsConfig)

    middlewares = middlewares.Append(
        reqLimiter.Limit,
        garcon.ServerHeader(serverHeader),
        cors.Handler(allowedOrigins, devMode),
    )

    // Endpoint authentication rules (Open Policy Agent)
    files := garcon.SplitClean(authCfg)
    policy, err := opa.New(files, resErr)
    if err != nil {
        log.Fatal(err)
    }

    if policy.Ready() {
        middlewares = middlewares.Append(policy.Auth)
    }

    return middlewares, connState
}

// runServer runs in foreground the main server.
func runServer(h http.Handler, connState func(net.Conn, http.ConnState)) {
    const mainPort = "8080"

    server := http.Server{
        Addr:              ":" + mainPort,
        Handler:           h,
        TLSConfig:         nil,
        ReadTimeout:       1 * time.Second,
        ReadHeaderTimeout: 1 * time.Second,
        WriteTimeout:      1 * time.Second,
        IdleTimeout:       1 * time.Second,
        MaxHeaderBytes:    222,
        TLSNextProto:      nil,
        ConnState:         connState,
        ErrorLog:          log.Default(),
        BaseContext:       nil,
        ConnContext:       nil,
    }

    log.Print("Server listening on http://localhost", server.Addr)

    log.Fatal(server.ListenAndServe())
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(resErr reserr.ResErr) http.Handler {
    r := chi.NewRouter()

    // Static website files
    ws := webserver.WebServer{Dir: "examples/www", ResErr: resErr}
    r.Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
    r.Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    r.Get("/css/*", ws.ServeDir("text/css; charset=utf-8"))
    r.Get("/images/*", ws.ServeImages())

    // API
    r.Get("/api/v1/items", items)
    r.Get("/api/v1/ducks", resErr.NotImplemented)

    // Other endpoints
    r.NotFound(resErr.InvalidPath)

    return r
}

func items(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write([]byte(`["item1","item2","item3"]`))
}
```

## License

[LGPL-3.0-or-later](https://spdx.org/licenses/LGPL-3.0-or-later.html):
GNU Lesser General Public License v3.0 or later
([tl;drLegal](https://tldrlegal.com/license/gnu-lesser-general-public-license-v3-(lgpl-3)),
[Choosealicense.com](https://choosealicense.com/licenses/lgpl-3.0/)).
See the [LICENSE](LICENSE) file.

Except:

* the example files under CC0-1.0 (Creative Commons Zero v1.0 Universal) ;
* the file [chain.go](chain/chain.go) (fork) under the MIT License.
