# Teal.Finance/Garcon

| ![logo](examples/www/myapp/images/garcon.png) | Garcon works with all HTTP routers ans middleware respecting the Go HTTP standards. Garcon provides the batteries: static website server, contact-form backend, API helpers, debugging helpers (PProf), Git version, metrics server (Prometheus), URI sanitization and middleware: rate-limiter, JWT cookies, CORS, traffic logs, OPA…<br>[![Go Reference](examples/www/myapp/images/go-ref.svg "Go documentation for Garcon")](https://pkg.go.dev/github.com/teal-finance/garcon) [![Go Report Card](https://goreportcard.com/badge/github.com/teal-finance/garcon)](https://goreportcard.com/report/github.com/teal-finance/garcon) |
| --------------------------------------------- |:--------- |

## Motivation

Many projects often start with one of the many nice HTTP routers and middleware already available and develop their own middleware, debugging server, API helpers... At Teal.Finance, we decided to share in one place (here) all our stuff in the idea to let other projects go faster.

## Middleware

Our Middleware are very easy to setup. They respect the Go standards. Thus you can easily use them with the HTTP router of your choice and chained them with other middleware:

- `MiddlewareLogRequest` Log incoming requests (with or without browser fingerprint)
- `MiddlewareLogDuration` Log processing time
- `MiddlewareExportTrafficMetrics` Export web traffic metrics
- `MiddlewareRejectUnprintableURI` Reject request with unwanted characters
- `MiddlewareRateLimiter` Limit incoming request to prevent flooding
- `MiddlewareServerHeader` Add the "Server" HTTP header in the response
- `JWTChecker` JWT management using HttpOnly cookie or Authorization header
- `IncorruptibleChecker` Session cookie with [Incorruptible](https://github.com/teal-finance/incorruptible) token
- `MiddlewareCORS` Cross-Origin Resource Sharing (CORS)
- `MiddlewareOPA` Authenticate from Datalog/Rego files using [Open Policy Agent](https://www.openpolicyagent.org)
- `MiddlewareSecureHTTPHeader` Set some HTTP header to increase the web security

```go
g := garcon.New()
middleware = garcon.NewChain(
    g.MiddlewareRejectUnprintableURI(),
    g.MiddlewareLogRequest(),
    g.MiddlewareRateLimiter())
router  := ... you choose
handler := middleware.Then(router)
server  := http.Server{Addr: ":8080", Handler: handler}
server.ListenAndServe()
```

## Other features

- Static web files server supporting Brotli and AVIF
- Metrics server exporting data to Prometheus (or other compatible monitoring tool)
- PProf server for debugging purpose
- Serialize JSON responses, including the error messages
- Chained middleware (fork of [justinas/alice](https://github.com/justinas/alice))
- Chained round trip handlers
- Retrieve Git version, branch and commit from build flags and Go module information

## Basic example

```go
g := garcon.New()

// chain some middleware
middleware = garcon.NewChain(
    g.MiddlewareRejectUnprintableURI(),
    g.MiddlewareLogRequests(),
    g.MiddlewareRateLimiter())

// use the HTTP router library of your choice, here we use Chi 
router := chi.NewRouter()

// static website, automatically sends the Brotli-compressed file if present and supported by the browser
ws := g.NewStaticWebServer("/var/www")
router.Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
router.Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
router.Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
router.Get("/css/*", ws.ServeDir("text/css; charset=utf-8"))
router.Get("/images/*", ws.ServeImages()) // automatically sends AVIF if present and supported by the browser

// receive contact-forms on your chat channel on the fly
cf := g.NewContactForm("/")
router.Post("/", cf.Notify("https://mattermost.com/hooks/qite178czotd5"))

// Git version and last commit date (HTML or JSON depending on the "Accept" header)
router.Get("/version", garcon.ServeVersion())

// return a JSON message
router.Get("/reserved", g.Writer.NotImplemented)
router.NotFound(g.Writer.InvalidPath)

handler := middleware.Then(router)
server := http.Server{Addr: ":8080", Handler: handler}
server.ListenAndServe()
```

## Incorruptible middleware

Garcon uses the [Incorruptible](https://github.com/teal-finance/incorruptible) package
to create/verify session cookie.

```go
package main

import "github.com/teal-finance/garcon"

func main() {
    g, _ := garcon.New(
        garcon.WithURLs("https://my-company.com"),
        garcon.WithDev())

    aes128Key = "00112233445566778899aabbccddeeff"
    maxAge := 3600
    setIP := true
    ck := g.IncorruptibleChecker(aes128Key, maxAge, setIP)

    router := chi.NewRouter()

    // website with static files directory
    ws := g.NewStaticWebServer("/var/www")

    // ck.Set => set the cookie when visiting /
    router.With(ck.Set).Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))

    // ck.Chk => reject request with invalid JWT cookie
    router.With(ck.Chk).Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    router.With(ck.Chk).Get("/assets/*", ws.ServeAssets())

    // ck.Vet => accepts valid JWT in either the cookie or the Authorization header
    router.With(ck.Vet).Post("/api/items", myFunctionHandler)

    server := http.Server{Addr: ":8080", Handler: router}
    server.ListenAndServe()
}
```

## JWT middleware

The JWT and Incorruptible checkers share a common interface,
`TokenChecker`, providing the same middleware: `Set()`, `Chk()` and `Vet()`.

```go
package main

import "github.com/teal-finance/garcon"

func main() {
    g, _ := garcon.New(
        garcon.WithURLs("https://my-company.com"),
        garcon.WithDev())

    hmacSHA256Key := "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
    ck := g.JWTChecker(hmacSHA256Key, "FreePlan", 10, "PremiumPlan", 100)

    router := chi.NewRouter()

    // website with static files directory
    ws := g.NewStaticWebServer("/var/www")

    // ck.Set => set the cookie when visiting /
    router.With(ck.Set).Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))

    // ck.Chk => reject request with invalid JWT cookie
    router.With(ck.Chk).Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    router.With(ck.Chk).Get("/assets/*", ws.ServeAssets())

    // ck.Vet => accepts valid JWT in either the cookie or the Authorization header
    router.With(ck.Vet).Post("/api/items", myFunctionHandler)

    server := http.Server{Addr: ":8080", Handler: router}
    server.ListenAndServe()
}
```

## Who use Garcon

In production, this library is used by
[Rainbow](https://github.com/teal-finance/rainbow),
[Quid](https://github.com/teal-finance/quid)
and other internal projects at Teal.Finance.

Please propose a [Pull Request](https://github.com/teal-finance/garcon/pulls) to add here your project that also uses Garcon.

See a complete real example in the repo
[github.com/teal-finance/rainbow](https://github.com/teal-finance/rainbow/blob/main/cmd/server/main.go).

## CPU profiling

Moreover, Garcon simplifies investigation on CPU and memory consumption issues
thanks to <https://github.com/pkg/profile>.

In your code, add `defer garcon.ProbeCPU.Stop()` that will write the `cpu.pprof` file.

```go
import "github.com/teal-finance/garcon"

func myFunctionConsumingLotsOfCPU() {
    defer garcon.ProbeCPU.Stop()

    // ... lots of sub-functions
}
```

Run `pprof` and browse your `cpu.pprof` file:

```sh
go run github.com/google/pprof@latest -http=: cpu.pprof
```

## Complete example

See the [complete example](examples/complete)
enabling almost of the Garcon features.
Below is a simplified extract:

```go
package main

import "github.com/teal-finance/garcon"

func main() {
    defer garcon.ProbeCPU().Stop() // collects the CPU-profile and writes it in the file "cpu.pprof"

    garcon.LogVersion()     // log the Git version
    garcon.SetVersionFlag() // the -version flag prints the Git version
    jwt := flag.Bool("jwt", false, "Use JWT in lieu of the Incorruptible token")
    flag.Parse()

    g := garcon.New(
        garcon.WithURLs("https://my-company.co"),
        garcon.WithDocURL("/doc"),
        garcon.WithPProf(8093))

    ic := g.IncorruptibleChecker(aes128Key, 60, true)
    jc := g.JWTChecker(hmacSHA256Key, "FreePlan", 10, "PremiumPlan", 100)

    middleware, connState := g.StartMetricsServer(9093)
    middleware = middleware.Append(
        g.MiddlewareRejectUnprintableURI(),
        g.MiddlewareLogRequests("fingerprint"),
        g.MiddlewareRateLimiter(10, 30),
        g.MiddlewareServerHeader("MyApp"),
        g.MiddlewareCORS(),
        g.MiddlewareOPA("auth.rego"),
        g.MiddlewareLogDuration(true()))

    router := chi.NewRouter()

    // website with static files directory
    ws := g.NewStaticWebServer("/var/www")
    router.With(ic.Set).With(jc.Set).Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
    router.With(ic.Chk).Get("/assets/*", ws.ServeAssets())

    // Contact-form
    cf := g.NewContactForm("/")
    router.Post("/", cf.Notify("https://mattermost.com/hooks/qite178czotd5"))

    // API
    router.With(jc.Vet).Post("/api/items", myFunctionHandler)

    handler := middleware.Then(router)
    server := garcon.Server(handler, 8080, connState)
    garcon.ListenAndServe(&server)
}
```

### 1. Run the [complete example](examples/complete/main.go)

```sh
cd garcon
go run -race ./examples/complete
```

```log
garcon ℹ️  Probing CPU. To visualize the profile: pprof -http=: cpu.pprof
2022/09/14 18:43:05 profile: cpu profiling enabled, cpu.pprof
garcon 🎬  Version: devel
garcon ℹ️  Enable PProf endpoints: http://localhost:8093/debug/pprof
incorr 🔒  DevMode accepts missing/invalid token from http://localhost:8080/myapp
incorr 🔒  cookie myapp Domain=localhost Path=/myapp Max-Age=60 Secure=false SameSite=3 HttpOnly=true Value=0 bytes
garcon ℹ️  Prometheus export http://localhost:9093 namespace=myapp
garcon 🔒  CORS Allow origin prefixes: [http://localhost:8080 http://localhost: http://192.168.1.]
garcon 🔒  CORS Methods: [GET POST DELETE]
garcon 🔒  CORS Headers: [Origin Content-Type Authorization]
garcon 🔒  CORS Credentials=true MaxAge=86400
incorr 🔒  Middleware Incorruptible.Set cookie "myapp" MaxAge=60 setIP=true
incorr 🔒  Middleware Incorruptible.Set cookie "myapp" MaxAge=60 setIP=true
incorr 🔒  Middleware Incorruptible.Chk cookie DevMode= true
incorr 🔒  Middleware Incorruptible.Chk cookie DevMode= true
incorr 🔒  Middleware Incorruptible.Chk cookie DevMode= true
incorr 🔒  Middleware Incorruptible.Chk cookie DevMode= true
garcon ℹ️  Middleware WebForm redirects to http://localhost:8080/myapp
garcon ℹ️  empty URL => use the LogNotifier
incorr 🔒  Middleware Incorruptible.Set cookie "myapp" MaxAge=60 setIP=true
incorr 🔒  Middleware Incorruptible.Vet cookie/bearer DevMode= true
incorr 🔒  Middleware Incorruptible.Vet cookie/bearer DevMode= true
incorr 🔒  Middleware Incorruptible.Vet cookie/bearer DevMode= true
garcon ℹ️  MiddlewareLogDurationSafe: logs requester IP, sanitized URL and duration
garcon ℹ️  MiddlewareServerHeader sets the HTTP header Server=MyApp-devel in the responses
garcon ℹ️  MiddlewareRateLimiter burst=40 rate=2.67/s
garcon ℹ️  MiddlewareLogFingerprint: 
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
11. DNT (Do Not Track) is being dropped by web browsers.
garcon ℹ️  MiddlewareRejectUnprintableURI rejects URI having line breaks or unprintable characters
app    🎬  -------------- Open http://localhost:8080/myapp --------------
garcon 📰  Server listening on http://localhost:8080
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

:warning: **WARNING: This section is outdated!** :warning:

The [complete example](examples/complete/main.go) is running.

Open <http://localhost:8080/myapp> with your browser, and play with the API endpoints.

The resources and API endpoints are protected with a HttpOnly cookie.
The [complete example](examples/complete/main.go) sets the cookie to browsers visiting the `index.html`.

```go
func handler(gw garcon.Writer, jc *jwtperm.Checker) http.Handler {
    r := chi.NewRouter()

    // Static website files
    ws := garcon.WebServer{Dir: "examples/www", Writer: gw}
    r.With(jc.SetCookie).Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
    r.With(jc.SetCookie).Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
    r.With(jc.ChkCookie).Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    r.With(jc.ChkCookie).Get("/css/*", ws.ServeDir("text/css; charset=utf-8"))
    r.With(jc.ChkCookie).Get("/images/*", ws.ServeImages())

    // API
    r.With(jc.ChkJWT).Get("/api/v1/items", items)
    r.With(jc.ChkJWT).Get("/api/v1/ducks", gw.NotImplemented)

    // Other endpoints
    r.NotFound(gw.InvalidPath)

    return r
}
```

### 5. Enable Authentication

:warning: **WARNING: This section is outdated!** :warning:

Restart again the [complete example](examples/complete/main.go) with authentication enabled.

Attention, in this example we use two redundant middleware pieces using the same JWT: `jwtperm` and `opa`.
This is just an example, don't be confused.

```sh
go run -race ./examples/complete -auth
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
2021/12/02 08:09:47 MiddlewareRateLimiter burst=100 rate=5/s
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

:warning: **WARNING: This section is outdated!** :warning:

See the [low-level example](examples/low-level/main.go).

The following code is a bit different to the stuff done
by the complete function `Garcon.Run()` presented in the previous chapter.
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
    gw := garcon.NewWriter(apiDoc)

    middleware, connState := setMiddlewares(gw)

    // Handles both REST API and static web files
    h := handler(gw)
    h = middleware.Then(h)

    runServer(h, connState)
}

func setMiddlewares(gw garcon.Writer) (middleware garcon.Chain, connState func(net.Conn, http.ConnState)) {
    // Start a metrics server in background if export port > 0.
    // The metrics server is for use with Prometheus or another compatible monitoring tool.
    middleware, connState = garcon.StartMetricsServer(expPort, devMode)

    // Limit the input request rate per IP
    reqLimiter := garcon.NewReqLimiter(gw, burst, reqMinute, devMode)

    corsConfig := allowedProdOrigin
    if devMode {
        corsConfig += " " + allowedDevOrigins
    }

    allowedOrigins := garcon.SplitClean(corsConfig)

    middleware = middleware.Append(
        reqLimiter.Limit,
        garcon.ServerHeader(serverHeader),
        cors.Handler(allowedOrigins, devMode),
    )

    // Endpoint authentication rules (Open Policy Agent)
    files := garcon.SplitClean(authCfg)
    policy, err := garcon.NewPolicy(gw, files)
    if err != nil {
        log.Fatal(err)
    }

    if policy.Ready() {
        middleware = middleware.Append(policy.Auth)
    }

    return middleware, connState
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
func handler(gw garcon.Writer) http.Handler {
    r := chi.NewRouter()

    // Website with static files
    ws := g.NewStaticWebServer("/var/www")
    r.Get("/", ws.ServeFile("index.html", "text/html; charset=utf-8"))
    r.Get("/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
    r.Get("/css/*", ws.ServeDir("text/css; charset=utf-8"))
    r.Get("/images/*", ws.ServeImages())

    // API
    r.Get("/api/v1/items", items)
    r.Get("/api/v1/ducks", gw.NotImplemented)

    // Other endpoints
    r.NotFound(gw.InvalidPath)

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
cd garcon
go run -race ./examples/keystore
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

Copyright (c) 2021 Teal.Finance/Garcon contributors

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
- <https://github.com/kataras/iris>
