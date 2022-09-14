help:
	# make all    Do the following commands
	# make up     Go: Upgrade the patch version of the dependencies
	# make up+    Go: Upgrade the minor version of the dependencies
	# make fmt    Update generated code and Format code
	# make test   Check build and Test
	# make vet    Lint and Run examples
	# make cov    Browse test coverage

.PHONY: all
all: up fmt test vet cov

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
	go run mvdan.cc/gofumpt@latest -w -extra -l -lang 1.19 .

.PHONY: test
test:
	go build ./...
	go test -race -vet all -tags=garcon -coverprofile=code-coverage.out ./...

code-coverage.out: go.sum */*.go
	go test -race -vet all -tags=garcon -coverprofile=code-coverage.out ./...

.PHONY: cov
cov: code-coverage.out
	go tool cover -html code-coverage.out

.PHONY: vet
vet:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fix || true
	go run -race ./examples/complete || true
	go run -race ./examples/low-level || true
	go run -race ./examples/keystore || true
	go run -race ./examples/httprouten || true
	go run -race ./examples/chi || true
