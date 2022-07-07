# Teal.Finance/Garcon

| ![logo](examples/www/myapp/images/garcon.png) | Opinionated boilerplate all-in-one HTTP server with rate-limiter, Cookies, JWT, CORS, OPA, web traffic, Prometheus export, PProf… for API and static website.<br><br><br>[![Go Reference](examples/www/myapp/images/go-ref.svg "Go documentation for Garcon")](https://pkg.go.dev/github.com/teal-finance/garcon) [![Go Report Card](https://goreportcard.com/badge/github.com/teal-finance/garcon)](https://goreportcard.com/report/github.com/teal-finance/garcon) |
| --------------------------------------------- | --------- |

This library is used by
[Rainbow](https://github.com/teal-finance/rainbow)
and other internal projects at Teal.Finance.

Please propose a [Pull Request](https://github.com/teal-finance/garcon/pulls) to add here your project that also uses Garcon.

## Features

Garcon includes the following middleware pieces:

- Logging of incoming requests ;
- Rate-limiter to prevent requests flooding ;
- JWT management using HttpOnly cookie or Authorization header ;
- Cross-Origin Resource Sharing (CORS) ;
- Authentication rules based on Datalog/Rego files using [Open Policy Agent](https://www.openpolicyagent.org) ;
- Web traffic metrics.

Garcon also provides the following features:

- HTTP/REST server for API endpoints (compatible with any Go-standard HTTP handlers) ;
- File server intended for static web files supporting Brotli and AVIF data ;
- Metrics server exporting data to Prometheus (or other compatible monitoring tool) ;
- PProf server for debugging purpose ;
- Error response in JSON format ;
- Chained middleware (fork of [justinas/alice](https://github.com/justinas/alice)).

## CPU profiling

Moreover, Garcon provides a helper feature `defer ProbeCPU.Stop()`
to investigate CPU consumption issues
thanks to <https://github.com/pkg/profile>.

In you code, add `defer ProbeCPU.Stop()` that will write the `cpu.pprof` file.

```go
import "github.com/teal-finance/garcon/pprof"

func myFunctionConsumingLotsOfCPU() {
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

## High-level example

The following code uses the high-level function `Garcon.RunServer()`.

```go
package main

import "github.com/teal-finance/garcon"

func main() {
    g, _ := garcon.New(
        garcon.WithURLs("http://localhost:8080/myapp"),
        garcon.WithDocURL("/doc"), // URL --> http://localhost:8080/myapp/doc
        garcon.WithServerHeader("MyBackendName-1.2.0"),
        garcon.WithJWT(hmacSHA256, "FreePlan", 10, "PremiumPlan", 100),
        garcon.WithOPA("auth.rego"),
        garcon.WithLimiter(10, 30),
        garcon.WithPProf(8093),
        garcon.WithProm(9093),
        garcon.WithDev(),
    )

    h := handler(g.ErrWriter, g.Checker)

    g.Run(h, 8080)
}
```

### 1. Run the [high-level example](examples/high-level/main.go)

```sh
go build -race ./examples/high-level && ./high-level
```

```log
2022/01/29 17:31:26 Prometheus export http://localhost:9093
2022/01/29 17:31:26 CORS: Set origin prefixes: [http://localhost:8080 http://localhost: http://192.168.1.]
2022/01/29 17:31:26 CORS: Methods=[GET POST] Headers=[Origin Accept Content-Type Authorization Cookie] Credentials=true MaxAge=86400
2022/01/29 17:31:26 Enable PProf endpoints: http://localhost:8093/debug/pprof
2022/01/29 17:31:26 Create cookie plan=FreePlan domain=localhost secure=false myapp=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiRnJlZVBsYW4iLCJleHAiOjE2NzUwMDk4ODZ9.hiQQuFxNghrrCvvzEsXzN1lWTavL09Plx0dhFynrBxc
2022/01/29 17:31:26 Create cookie plan=PremiumPlan domain=localhost secure=false myapp=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiUHJlbWl1bVBsYW4iLCJleHAiOjE2NzUwMDk4ODZ9.iP587iHjhLmX_8yMhuQfKu9q7qLbLE7UX-UgkL_VYhE
2022/01/29 17:31:26 JWT not required for dev. origins: [http://localhost:8080 http://localhost: http://192.168.1.]
2022/01/29 17:31:26 Middleware response HTTP header: Set Server MyApp-1.2.0
2022/01/29 17:31:26 Middleware RateLimiter: burst=100 rate=5/s
2022/01/29 17:31:26 Middleware logger: requester IP and requested URL
2022/01/29 17:31:26 Server listening on http://localhost:8080
```

### 2. Embedded PProf server

Visit the PProf server at <http://localhost:8093/debug/pprof> providing the following endpoints:

- <http://localhost:8093/debug/pprof/cmdline> - Command line arguments
- <http://localhost:8093/debug/pprof/profile> - CPU profile
- <http://localhost:8093/debug/pprof/allocs> - Memory allocations from start
- <http://localhost:8093/debug/pprof/heap> - Current memory allocations
- <http://localhost:8093/debug/pprof/trace> - Current program trace
- <http://localhost:8093/debug/pprof/goroutine> - Traces of all current threads (goroutines)
- <http://localhost:8093/debug/pprof/block> - Traces of blocking threads
- <http://localhost:8093/debug/pprof/mutex> - Traces of threads with contended mutex
- <http://localhost:8093/debug/pprof/threadcreate> - Traces of threads creating a new thread

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

Open <http://localhost:8080/myapp> with your browser, and play with the API endpoints.

The resources and API endpoints are protected with a HttpOnly cookie.
The [high-level example](examples/high-level/main.go) sets the cookie to browsers visiting the `index.html`.

```go
func handler(errWriter garcon.ErrWriter, jc *jwtperm.Checker) http.Handler {
    r := chi.NewRouter()

    // Static website files
    ws := webserver.WebServer{Dir: "examples/www", ErrWriter: errWriter}
    r.With(jc.SetCookie).Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
    r.With(jc.SetCookie).Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
    r.With(jc.ChkCookie).Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    r.With(jc.ChkCookie).Get("/css/*", ws.ServeDir("text/css; charset=utf-8"))
    r.With(jc.ChkCookie).Get("/images/*", ws.ServeImages())

    // API
    r.With(jc.ChkJWT).Get("/api/v1/items", items)
    r.With(jc.ChkJWT).Get("/api/v1/ducks", errWriter.NotImplemented)

    // Other endpoints
    r.NotFound(errWriter.InvalidPath)

    return r
}
```

### 5. Enable Authentication

Restart again the [high-level example](examples/high-level/main.go) with authentication enabled.

Attention, in this example we use two redundant middleware pieces using the same JWT: `jwtperm` and `opa`.
This is just an example, don't be confused.

```sh
go build -race ./examples/high-level && ./high-level -auth
```

```log
2021/12/02 08:09:47 Prometheus export http://localhost:9093
2021/12/02 08:09:47 CORS: Set origin prefixes: [http://localhost:8080 http://localhost: http://192.168.1.]
2021/12/02 08:09:47 CORS: Methods=[GET] Headers=[Origin Accept Content-Type Authorization Cookie] Credentials=true MaxAge=86400
2021/12/02 08:09:47 JWT not required for dev. origins: [http://localhost:8080 http://localhost: http://192.168.1.]
2021/12/02 08:09:47 Enable PProf endpoints: http://localhost:8093/debug/pprof
2021/12/02 08:09:47 Create cookie plan=FreePlan domain=localhost secure=false jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiRnJlZVBsYW4iLCJleHAiOjE2Njk5NjQ5ODd9.5tJk2NoHxkG0o_owtMleBcUaR8z1vRx4rxRRqtZUc_Q
2021/12/02 08:09:47 Create cookie plan=PremiumPlan domain=localhost secure=false jwt=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiUHJlbWl1bVBsYW4iLCJleHAiOjE2Njk5NjQ5ODd9.ifKhbmxQQ64NweL5aQDb_42tvKHwqiEKD-vxHO3KzsM
2021/12/02 08:09:47 OPA: load "examples/sample-auth.rego"
2021/12/02 08:09:47 Middleware OPA: map[sample-auth.rego:package auth

default allow = false
tokens := {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiRnJlZVBsYW4iLCJleHAiOjE2Njk5NjQ0ODh9.elDm_t4vezVgEmS8UFFo_spLJTts7JWybzbyO_aYV3Y"} { true }
allow = true { __local0__ = input.token; data.auth.tokens[__local0__] }]
2021/12/02 08:09:47 Middleware response HTTP header: Set Server MyBackendName-1.2.0
2021/12/02 08:09:47 Middleware RateLimiter: burst=100 rate=5/s
2021/12/02 08:09:47 Middleware logger: requester IP and requested URL
2021/12/02 08:09:47 Server listening on http://localhost:8080
```

### 6. Default HTTP request headers

Test the API with `curl`:

```sh
curl -D - http://localhost:8080/myapp/api/v1/items
```

```yaml
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Server: MyBackendName-1.2.0
Vary: Origin
X-Content-Type-Options: nosniff
Date: Thu, 02 Dec 2021 07:06:20 GMT
Content-Length: 84

{"error":"Unauthorized",
"path":"/api/v1/items",
"doc":"http://localhost:8080/myapp/doc"}
```

The corresponding garcon logs:

```log
2021/12/02 08:06:20 in  127.0.0.1:42888 GET /api/v1/items
[cors] 2021/12/02 08:06:20 Handler: Actual request
[cors] 2021/12/02 08:06:20   Actual request no headers added: missing origin
2021/12/02 08:06:20 OPA unauthorize 127.0.0.1:42888 /api/v1/items
2021/12/02 08:06:20 out 127.0.0.1:42888 GET /api/v1/items 1.426916ms c=1
```

The CORS logs can be disabled by passing `debug=false` in `cors.Handler(origins, false)`.

The value `c=1` measures the web traffic (current active HTTP connections).

### 7. With Authorization header

```sh
curl -D - http://localhost:8080/myapp/api/v1/items -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiRnJlZVBsYW4iLCJleHAiOjE2Njk5NjQ0ODh9.elDm_t4vezVgEmS8UFFo_spLJTts7JWybzbyO_aYV3Y'
```

```yaml
HTTP/1.1 200 OK
Content-Type: application/json
Server: MyBackendName-1.2.0
Vary: Origin
Date: Thu, 02 Dec 2021 07:10:37 GMT
Content-Length: 25

["item1","item2","item3"]
```

The corresponding garcon logs:

```log
2021/12/02 08:10:37 in  127.0.0.1:42892 GET /api/v1/items
[cors] 2021/12/02 08:10:37 Handler: Actual request
[cors] 2021/12/02 08:10:37   Actual request no headers added: missing origin
2021/12/02 08:10:37 Authorization header has JWT: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuIjoiRnJlZVBsYW4iLCJleHAiOjE2Njk5NjQ0ODh9.elDm_t4vezVgEmS8UFFo_spLJTts7JWybzbyO_aYV3Y
2021/12/02 08:10:37 JWT Claims: {FreePlan  {  [] 2022-12-02 08:01:28 +0100 CET <nil> <nil> invalid cookie}}
2021/12/02 08:10:37 JWT has the FreePlan Namespace
2021/12/02 08:10:37 JWT Permission: {10}
2021/12/02 08:10:37 out 127.0.0.1:42892 GET /api/v1/items 1.984568ms c=1 a=1 i=0 h=0
```

## Low-level example

:warning: **WARNING: This chapter is outdated!** :warning:

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

    "github.com/go-chi/chi/v5"
    "github.com/teal-finance/garcon"
)

// Garcon settings
const apiDoc = "https://my-dns.co/doc"
const allowedProdOrigin = "https://my-dns.co"
const allowedDevOrigins = "http://localhost: http://192.168.1."
const serverHeader = "MyBackendName-1.2.0"
const authCfg = "examples/sample-auth.rego"
const pprofPort = 8093
const expPort = 9093
const burst, reqMinute = 10, 30
const devMode = true

func main() {
    if devMode {
        // the following line collects the CPU-profile and writes it in the file "cpu.pprof"
        defer garcon.ProbeCPU().Stop()
    }

    garcon.StartPProfServer(pprofPort)

    // Uniformize error responses with API doc
    errWriter := garcon.NewErrWriter(apiDoc)

    chain, connState := setMiddlewares(errWriter)

    // Handles both REST API and static web files
    h := handler(errWriter)
    h = chain.Then(h)

    runServer(h, connState)
}

func setMiddlewares(errWriter garcon.ErrWriter) (chain garcon.Chain, connState func(net.Conn, http.ConnState)) {
    // Start a metrics server in background if export port > 0.
    // The metrics server is for use with Prometheus or another compatible monitoring tool.
    metrics := garcon.Metrics{}
    chain, connState = garcon.StartMetricsServer(expPort, devMode)

    // Limit the input request rate per IP
    reqLimiter := garcon.NewReqLimiter(burst, reqMinute, devMode, errWriter)

    corsConfig := allowedProdOrigin
    if devMode {
        corsConfig += " " + allowedDevOrigins
    }

    allowedOrigins := garcon.SplitClean(corsConfig)

    chain = chain.Append(
        reqLimiter.Limit,
        garcon.ServerHeader(serverHeader),
        cors.Handler(allowedOrigins, devMode),
    )

    // Endpoint authentication rules (Open Policy Agent)
    files := garcon.SplitClean(authCfg)
    policy, err := garcon.NewPolicy(files, errWriter)
    if err != nil {
        log.Fatal(err)
    }

    if policy.Ready() {
        chain = chain.Append(policy.Auth)
    }

    return chain, connState
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
func handler(errWriter garcon.ErrWriter) http.Handler {
    r := chi.NewRouter()

    // Website with static files
    ws := garcon.NewStaticWebServer("examples/www", errWriter)
    r.Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
    r.Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    r.Get("/css/*", ws.ServeDir("text/css; charset=utf-8"))
    r.Get("/images/*", ws.ServeImages())

    // API
    r.Get("/api/v1/items", items)
    r.Get("/api/v1/ducks", errWriter.NotImplemented)

    // Other endpoints
    r.NotFound(errWriter.InvalidPath)

    return r
}

func items(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write([]byte(`["item1","item2","item3"]`))
}
```

## KeyStore example

The example KeyStore implements a key/value datastore
providing private storage for each client identified by its unique IP.

```sh
go build ./examples/keystore
./keystore
```

Then open <http://localhost:8080> to learn more about the implemented features.

## ✨ Contributions Welcome

This project needs your help to become better.
Please propose your enhancements,
or even a further refactoring.

We welcome contributions in many forms,
and there's always plenty to do!

## 🗣️ Feedback

If you have some suggestions, or need a new feature,
please contact us, using the
[issues](https://github.com/teal-finance/garcon/issues),
or at Teal.Finance@pm.me or
[@TealFinance](https://twitter.com/TealFinance).

Feel free to propose a
[Pull Request](https://github.com/teal-finance/garcon/pulls),
your contributions are welcome. :wink:

## 🗽 Copyright and license

Copyright (c) 2022 Teal.Finance contributors

Teal.Finance/Garcon is free software, and can be redistributed
and/or modified under the terms of the MIT License.
SPDX-License-Identifier: MIT

Teal.Finance/Garcon is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty
of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

See the [LICENSE](LICENSE) file (alongside the source files)
or <https://opensource.org/licenses/MIT>.

## See also

- <https://github.com/kambahr/go-webstandard>
- <https://github.com/go-aah/aah>
- <https://github.com/xyproto/algernon>
