# Teal.Finance/Server

![logo](logo.jpg) | Opinionated HTTP server with CORS, OPA, Prometheus, rate-limiter… for API and static website.
-|-

## Origin

This library was originally developed as part of the project
[Rainbow](https://github.com/teal-finance/rainbow) during hackathons,
based on older Teal.Finance products,
and then moved to [its own repository](https://github.com/teal-finance/server).

## Features

Teal.Finance/Server supports:

* Metrics server exporting data to Prometheus or other monitoring services ;
* File server intended for static web files ;
* HTTP/REST server for API endpoints (compatible any Go-standard HTTP handlers) ;
* Chained middlewares (fork of github.com/justinas/alice)
* Auto-completed error response in JSON format ;
* Middleware: authentication rules based on Datalog/Rego files using [Open Policy Agent](https://www.openpolicyagent.org) ;
* Middleware: rate limiter to prevent flooding by incoming requests ;
* Middleware: logging of incoming requests ;
* Middleware: Cross-Origin Resource Sharing (CORS).

## License

[LGPL-3.0-or-later](https://spdx.org/licenses/LGPL-3.0-or-later.html):
GNU Lesser General Public License v3.0 or later
([tl;drLegal](https://tldrlegal.com/license/gnu-lesser-general-public-license-v3-(lgpl-3)),
[Choosealicense.com](https://choosealicense.com/licenses/lgpl-3.0/)).
See the [LICENSE](LICENSE) file.

Except the two example files under CC0-1.0 (Creative Commons Zero v1.0 Universal)
and the file [chain.go](chain/chain.go) (fork) under the MIT License.

## Easy usage

See [easy-example_test.go](easy-example_test.go).

```go
package main

import (
    "log"

    "github.com/teal-finance/server"
)

func main() {
    s := server.Server{
        Version:        "MyApp-1.2.3",
        Resp:           "https://my-dns.com/doc",
        AllowedOrigins: []string{"http://my-dns.com"},
        OPAFilenames:   []string{"rego.json"},
    }

    h := myHandler()

    // main port 8080, export port 9093, rate limiter 10 20, debug mode 
    log.Fatal(s.RunServer(h, 8080, 9093, 10, 20, true))
}
```

## Fined-control usage

See the [cmd/server/main.go](https://github.com/teal-finance/rainbow/blob/main/cmd/server/main.go)
in the repository Rainbow for a complete example.

See also the local file [full-example_test.go](full-example_test.go).

```go
package main

import (
    "log"
    "net"
    "net/http"
    "time"

    "github.com/teal-finance/server"
    "github.com/teal-finance/server/chain"
    "github.com/teal-finance/server/cors"
    "github.com/teal-finance/server/export"
    "github.com/teal-finance/server/limiter"
    "github.com/teal-finance/server/opa"
    "github.com/teal-finance/server/resperr"
)

func main() {
    middlewares, connState := setMiddlewares()

    h := myHandler()
    h = middlewares.Then(h)

    runServer(h, connState)
}

func setMiddlewares() (middlewares chain.Chain, connState func(net.Conn, http.ConnState)) {
    // Uniformize error responses with API doc
    respError := resperr.New("https://my-dns.com/doc")

    // Start a metrics server in background if export port > 0.
    // The metrics server is for use with Prometheus or another compatible monitoring tool.
    metrics := export.Metrics{}
    middlewares, connState = metrics.StartServer(9093, true)

    // Limit the input request rate per IP
    reqLimiter := limiter.New(10, 20, true, respError)
    middlewares = middlewares.Append()

    // Endpoint authentication rules (Open Policy Agent)
    policy, err := opa.New(respError, []string{"rego.json"})
    if err != nil {
        log.Fatal(err)
    }

    // CORS
    allowedOrigins := []string{"http://my-dns.com"}

    middlewares = middlewares.Append(
        server.LogRequests,
        reqLimiter.Limit,
        server.Header("MyServerName-1.2.3"),
        policy.Auth,
        cors.HandleCORS(allowedOrigins),
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
