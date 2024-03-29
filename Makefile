help:
	# make all    Do the following commands
	# make up     Go: Upgrade the patch version of the dependencies
	# make up+    Go: Upgrade the minor version of the dependencies
	# make fmt    Update generated code and Format code
	# make test   Check build and Test
	# make vet    Lint and Run examples
	# make cov    Browse test coverage

.PHONY: all
all: up+ fmt test vet cov

go.sum: go.mod
	go mod tidy

.PHONY: up
up: go.sum
	GOPROXY=direct go get -t -u=patch all
	go mod tidy

.PHONY: up+
up+: go.sum
	go get -u -t all
	go mod tidy

.PHONY: fmt
fmt:
	go generate ./...
	go run mvdan.cc/gofumpt@latest -w -extra -l -lang 1.22 .

.PHONY: test
test:
	go build ./...
	go test -race -vet all -tags=garcon -coverprofile=code-coverage.out ./...

code-coverage.out: go.sum */*.go
	go test -race -vet all -tags=garcon -coverprofile=code-coverage.out ./...

.PHONY: cov
cov: code-coverage.out
	go tool cover -html code-coverage.out

# Allow using a different timeout value, examples:
#    T=30s make vet
#    make vet T=1m
T ?= 10s

.PHONY: vet
vet:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fix || true
	pkill -fe [/]exe/complete   || true
	pkill -fe [/]exe/low-level  || true
	pkill -fe [/]exe/keystore   || true
	pkill -fe [/]exe/httprouter || true
	pkill -fe [/]exe/chi        || true
	timeout $T go run -race ./examples/complete   ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/low-level  ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/keystore   ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/httprouter ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
	timeout $T go run -race ./examples/chi        ; [ 124 -le $$? ] && echo Terminated by timeout $T ; true
