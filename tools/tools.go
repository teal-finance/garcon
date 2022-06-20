//go:build tools

// Package tools is a partial copy from
// https://github.com/teal-finance/rainbow/tree/main/pkg/tools
package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "mvdan.cc/gofumpt"
)

// Lint:
// go run github.com/golangci/golangci-lint/cmd/golangci-lint run --fix

// Format source code:
// go run mvdan.cc/gofumpt -extra -l -w .

// There is nothing more here intentionally.
// See https://github.com/teal-finance/rainbow/tree/main/pkg/tools
