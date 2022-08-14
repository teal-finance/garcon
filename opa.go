// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

// Policy manages the Open Policy Agent (OPA).
// see https://www.openpolicyagent.org/docs/edge/integration/#integrating-with-the-go-api
type Policy struct {
	gw       Writer
	compiler *ast.Compiler
}

var ErrEmptyOPAFilename = errors.New("OPA: missing filename")

// NewPolicy creates a new Policy by loading rego files.
func NewPolicy(gw Writer, filenames []string) (Policy, error) {
	compiler, err := LoadPolicy(filenames)
	return Policy{gw, compiler}, err
}

// LoadPolicy checks the Rego filenames and loads them to build the OPA compiler.
func LoadPolicy(filenames []string) (*ast.Compiler, error) {
	if len(filenames) == 0 {
		return nil, ErrEmptyOPAFilename
	}

	modules := map[string]string{}

	for _, fn := range filenames {
		log.Printf("INF OPA: load %q", fn)

		if fn == "" {
			return nil, ErrEmptyOPAFilename
		}

		content, err := os.ReadFile(fn)
		if err != nil {
			return nil, fmt.Errorf("OPA: ReadFile %w", err)
		}

		modules[path.Base(fn)] = string(content)
	}

	return ast.CompileModules(modules)
}

// MiddlewareOPA creates the middleware for Authentication rules (Open Policy Agent).
func (g *Garcon) MiddlewareOPA(opaFilenames ...string) Middleware {
	if len(opaFilenames) == 0 {
		return nil
	}
	policy, err := NewPolicy(g.Writer, opaFilenames)
	if err != nil {
		log.Panic("WithOPA: cannot create OPA Policy: ", err)
	}
	return policy.MiddlewareOPA
}

// MiddlewareOPA is the HTTP middleware to check endpoint authorization.
func (opa Policy) MiddlewareOPA(next http.Handler) http.Handler {
	log.Print("INF Middleware OPA: ", opa.compiler.Modules)

	compiler := opa.compiler
	gw := opa.gw

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := map[string]any{
			"method": r.Method,
			"path":   strings.Split(strings.Trim(r.URL.Path, "/"), "/"),
			"token":  r.Header.Get("Authorization"),
		}

		// evaluation
		rg := rego.New(
			rego.Query("data.auth.allow"),
			rego.Compiler(compiler),
			rego.Input(input),
		)

		rs, err := rg.Eval(context.Background())
		if err != nil || len(rs) == 0 {
			gw.WriteErr(w, r, http.StatusInternalServerError, "Cannot evaluate autorisation settings")
			log.Print("ERR OPA Eval: ", err)
			return
		}

		allow, ok := rs[0].Expressions[0].Value.(bool)
		if !ok {
			gw.WriteErr(w, r, http.StatusInternalServerError, "Missing autorisation settings")
			log.Print("ERR missing OPA data in ", rs)
			return
		}

		if !allow {
			gw.WriteErr(w, r, http.StatusUnauthorized, "No valid JWT",
				"advice", "Provide your JWT within the 'Authorization Bearer' HTTP header")
			log.Print("INF OPA Missing or invalid Authorization header " + r.RemoteAddr + " " + r.RequestURI)
			return
		}

		next.ServeHTTP(w, r)
	})
}
