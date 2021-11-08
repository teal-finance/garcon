# Teal.Finance/Garcon

![logo](logo.jpg) | <big>Opinionated boilerplate all-in-one HTTP server with rate-limiter, CORS, OPA, web traffic, Prometheus export, PProf… for API and static website.</big>
-|-

This library is used by
[Rainbow](https://github.com/teal-finance/rainbow)
and other internal projects at Teal.Finance.

Please propose a PR to add here your project that also uses Garcon.

## Features

Garcon includes the following middlewares:

* Logging of incoming requests ;
* Rate limiter to prevent requests flooding ;
* Authentication rules based on Datalog/Rego files using [Open Policy Agent](https://www.openpolicyagent.org) ;
* Cross-Origin Resource Sharing (CORS) ;
* Web traffic metrics.

Garcon also provides the following features:

* HTTP/REST server for API endpoints (compatible with any Go-standard HTTP handlers) ;
* File server intended for static web files supporting Brotli and AVIF data ;
* Metrics server exporting data to Prometheus (or other compatible monitoring tool) ;
* PProf server for debugging purpose ;
* Error response in JSON format ;
* Chained middlewares (fork of [github.com/justinas/alice](https://github.com/justinas/alice)).

## CPU profiling

Moreover, Garcon provides a helper feature `defer ProbeCPU.Stop()`
to investigate CPU consumption issues
thanks to <https://github.com/pkg/profile>.

In you code, add `defer ProbeCPU.Stop()` that will write the `cpu.pprof` file.

```go
import "github.com/teal-finance/garcon/pprof"

func myFunctionConsummingLotsOfCPU() {
    defer pprof.ProbeCPU.Stop()

    // ... lots of stuff
}
```

Install `pprof` and browse your `cpu.pprof` file:

```
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
    s := garcon.Garcon{
        Version:        "MyApp-1.2.0",
        Resp:           "https://my-dns.co/doc",
        AllowedOrigins: []string{"https://my-dns.co"},
        OPAFilenames:   []string{"my-auth.rego"},
    }

    h := myHandler()

    // main port 8080, export port 9093, rate limiter 10 20, dev mode 
    log.Fatal(s.RunServer(h, 8080, 9093, 10, 20, true))
}
```

### 1. Run the [high-level example](examples/high-level/main.go)

```
$ go build ./examples/high-level && ./high-level
2021/10/26 16:55:37 Prometheus export http://localhost:9093
2021/10/26 16:55:37 CORS: Set origin prefixes: [https://my-dns.co http://localhost: http://192.168.1.]
2021/10/26 16:55:37 Middleware CORS: {AllowedOrigins:[] AllowOriginFunc:0x6e6ee0 AllowOriginRequestFunc:<nil> AllowedMethods:[GET] AllowedHeaders:[Origin Accept Content-Type Authorization Cookie] ExposedHeaders:[] MaxAge:60 AllowCredentials:true OptionsPassthrough:false Debug:true}
2021/10/26 16:55:37 Enable PProf endpoints: http://localhost:8093/debug/pprof
2021/10/26 16:55:37 Middleware response HTTP header: Set Server MyBackendName-1.2.0
2021/10/26 16:55:37 Middleware RateLimiter: burst=100 rate=5/s
2021/10/26 16:55:37 Server listening on http://localhost:8080
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

```
cd ~
go get -u github.com/google/pprof

curl http://localhost:6063/debug/pprof/allocs > allocs.pprof
pprof -http=: allocs.pprof

wget http://localhost:31415/debug/pprof/heap
pprof -http=: heap

wget http://localhost:31415/debug/pprof/trace
pprof -http=: trace

wget http://localhost:31415/debug/pprof/goroutine
pprof -http=: goroutine
```

See the [blog post](https://go.dev/blog/pprof) (2013) for more accurate explanation.

### 3. Embedded metrics server

The export port <http://localhost:9093/metrics> (test it) is for monitoring tools like Prometheus.

### 4. Static website server

The [high-level example](examples/high-level/main.go)
is running without authentication.
Open <http://localhost:8080> with your browser,
and play with the API endpoints.

### 5. Enable Authentication

Then restart again the [high-level example](examples/high-level/main.go),
but with authentication enabled:

```
$ go build ./examples/high-level && ./high-level -auth
2021/10/26 16:51:30 Prometheus export http://localhost:9093
2021/10/26 16:51:30 CORS: Set origin prefixes: [https://my-dns.co http://localhost: http://192.168.1.]
2021/10/26 16:51:30 Middleware CORS: {AllowedOrigins:[] AllowOriginFunc:0x6e6ee0 AllowOriginRequestFunc:<nil> AllowedMethods:[GET] AllowedHeaders:[Origin Accept Content-Type Authorization Cookie] ExposedHeaders:[] MaxAge:60 AllowCredentials:true OptionsPassthrough:false Debug:true}
2021/10/26 16:51:30 OPA: load "examples/sample-auth.rego"
2021/10/26 16:51:30 Enable PProf endpoints: http://localhost:8093/debug/pprof
2021/10/26 16:51:30 Middleware OPA: map[sample-auth.rego:package auth

default allow = false
tokens := {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJsb2dnZWRJbkFzIjoiYWRtaW4iLCJpYXQiOjE0MjI3Nzk2Mzh9.gzSraSYS8EXBxLN_oWnFSRgCzcmJmMjLiuyu5CSpyHI"} { true }
allow = true { __local0__ = input.token; data.auth.tokens[__local0__] }]
2021/10/26 16:51:30 Middleware response HTTP header: Set Server MyBackendName-1.2.0
2021/10/26 16:51:30 Middleware RateLimiter: burst=100 rate=5/s
2021/10/26 16:51:30 Server listening on http://localhost:8080
```

### 6. Default HTTP request headers

Test the API with `curl`:

```
curl -D - http://localhost:8080/api/v1/items
```

```yaml
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Server: MyBackendName-1.2.0
Vary: Origin
X-Content-Type-Options: nosniff
Date: Tue, 26 Oct 2021 15:01:58 GMT
Content-Length: 80

{"error":"Unauthorized",
"path":"/api/v1/items",
"doc":"https://my-dns.co/doc"}
```

The corresponding garcon logs:

```
2021/10/26 17:01:58 in  GET [::1]:53336 /api/v1/items
[cors] 2021/10/26 17:01:58 Handler: Actual request
[cors] 2021/10/26 17:01:58   Actual request no headers added: missing origin
2021/10/26 17:01:58 OPA unauthorize [::1]:53336 /api/v1/items
2021/10/26 17:01:58 out GET [::1]:53336 /api/v1/items 342.221µs c=1 a=1 i=0 h=0
```

The CORS logs can be disabled by passing `debug=false` in `cors.Handler(origins, false)`.

The values `c=1 a=1 i=0 h=0` measure the web traffic:

* `c` for the current number of HTTP connections (gauge)
* `a` for the accumulated HTTP connections that have been in StateActive (counter)
* `i` for the accumulated HTTP connections that have been in StateIdle (counter)
* `h` for the accumulated HTTP connections that have been in StateHijacked (counter)

### 7. With Authorization header

```
curl -D - http://localhost:8080/api/v1/items -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJsb2dnZWRJbkFzIjoiYWRtaW4iLCJpYXQiOjE0MjI3Nzk2Mzh9.gzSraSYS8EXBxLN_oWnFSRgCzcmJmMjLiuyu5CSpyHI'
```

```yaml
HTTP/1.1 200 OK
Content-Type: application/json
Server: MyBackendName-1.2.0
Vary: Origin
Date: Tue, 26 Oct 2021 15:10:10 GMT
Content-Length: 25

["item1","item2","item3"]
```

The corresponding garcon logs:

```
2021/10/26 17:10:10 in  GET [::1]:53338 /api/v1/items
[cors] 2021/10/26 17:10:10 Handler: Actual request
[cors] 2021/10/26 17:10:10   Actual request no headers added: missing origin
2021/10/26 17:10:10 out GET [::1]:53338 /api/v1/items 333.351µs c=2 a=2 i=1 h=0
```

### 8. With Authorization and Origin headers

```
curl -D - http://localhost:8080/api/v1/items -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJsb2dnZWRJbkFzIjoiYWRtaW4iLCJpYXQiOjE0MjI3Nzk2Mzh9.gzSraSYS8EXBxLN_oWnFSRgCzcmJmMjLiuyu5CSpyHI' -H 'Origin: https://my-dns.co'
```

```yaml
HTTP/1.1 200 OK
Access-Control-Allow-Credentials: true
Access-Control-Allow-Origin: https://my-dns.co
Content-Type: application/json
Server: MyBackendName-1.2.0
Vary: Origin
Date: Tue, 26 Oct 2021 15:12:50 GMT
Content-Length: 25

["item1","item2","item3"]
```

The corresponding garcon logs:

```
2021/10/26 17:12:50 in  GET [::1]:53340 /api/v1/items
[cors] 2021/10/26 17:12:50 Handler: Actual request
2021/10/26 17:12:50 CORS: Accept https://my-dns.co because starts with prefix https://my-dns.co
[cors] 2021/10/26 17:12:50   Actual response added headers: map[Access-Control-Allow-Credentials:[true] Access-Control-Allow-Origin:[https://my-dns.co] Server:[MyBackendName-1.2.0] Vary:[Origin]]
2021/10/26 17:12:50 out GET [::1]:53340 /api/v1/items 385.422µs c=3 a=3 i=2 h=0
```

## Low-level

See the [low-level example](examples/low-level/main.go).

The following code is similar to the stuff done by the high-level function `Garcon.RunServer()` presented in the previous chapter. The following code is intended to show that Garcon can be customized to meet specific requirements.

```go
package main

import (
    "log"
    "net"
    "net/http"
    "time"

    "github.com/teal-finance/garcon"
    "github.com/teal-finance/garcon/chain"
    "github.com/teal-finance/garcon/cors"
    "github.com/teal-finance/garcon/export"
    "github.com/teal-finance/garcon/limiter"
    "github.com/teal-finance/garcon/opa"
    "github.com/teal-finance/garcon/reserr"
)

func main() {
    middlewares, connState := setMiddlewares()

    h := myHandler()
    h = middlewares.Then(h)

    runServer(h, connState)
}

func setMiddlewares() (middlewares chain.Chain, connState func(net.Conn, http.ConnState)) {
    // Uniformize error responses with API doc
    resErr := reserr.New("https://my-dns.co/doc")

    // Start a metrics server in background if export port > 0.
    // The metrics server is for use with Prometheus or another compatible monitoring tool.
    metrics := export.Metrics{}
    middlewares, connState = metrics.StartServer(9093, true)

    // Limit the input request rate per IP
    reqLimiter := limiter.New(10, 20, true, resErr)
    middlewares = middlewares.Append()

    // Endpoint authentication rules (Open Policy Agent)
    policy, err := opa.New(resErr, []string{"examples/sample-auth.rego"})
    if err != nil {
        log.Fatal(err)
    }

    // CORS
    allowedOrigins := []string{"https://my-dns.co"}

    middlewares = middlewares.Append(
        garcon.LogRequests,
        reqLimiter.Limit,
        garcon.ServerHeader("MyBackendName-1.2.0"),
        policy.Auth,
        cors.Handle(allowedOrigins, true),
    )

    return middlewares, connState
}

// runServer runs in foreground the main server.
func runServer(h http.Handler, connState func(net.Conn, http.ConnState)) {
    server := http.Server{
        Addr:              ":8080",
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
