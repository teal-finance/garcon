// Copyright (c) 2020-2022 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon, an API and website server.
// Teal.Finance/Garcon is free software under the GNU LGPL
// either version 3 or any later version, at the licensee's option.
// SPDX-License-Identifier: LGPL-3.0-or-later
//
// Teal.Finance/Garcon is distributed WITHOUT ANY WARRANTY.
// See the LICENSE and COPYING.LESSER files alongside the source files.

// package opa manages the Open Policy Agent.
// see https://www.openpolicyagent.org/docs/edge/integration/#integrating-with-the-go-api
package opa

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/teal-finance/garcon/reserr"
)

type Policy struct {
	compiler *ast.Compiler
	resErr   reserr.ResErr
}

var ErrEmptyFilename = errors.New("OPA: missing filename")

// New creates a new Policy by loading rego files.
func New(filenames []string, reserr reserr.ResErr) (Policy, error) {
	compiler, err := Load(filenames)
	return Policy{compiler, reserr}, err
}

// Load check the Rego filenames and loads them to build the OPA compiler.
func Load(filenames []string) (*ast.Compiler, error) {
	if len(filenames) == 0 {
		return nil, ErrEmptyFilename
	}

	modules := map[string]string{}

	for _, f := range filenames {
		log.Printf("OPA: load %q", f)

		if f == "" {
			return nil, ErrEmptyFilename
		}

		content, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("OPA: ReadFile %w", err)
		}

		modules[path.Base(f)] = string(content)
	}

	return ast.CompileModules(modules)
}

// Auth is the HTTP middleware to check endpoint authorization.
func (opa Policy) Auth(next http.Handler) http.Handler {
	log.Print("Middleware OPA: ", opa.compiler.Modules)

	compiler := opa.compiler
	resErr := opa.resErr

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := map[string]interface{}{
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
			resErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #1")
			log.Print("ERR OPA Eval: ", err)
			return
		}

		allow, ok := rs[0].Expressions[0].Value.(bool)
		if !ok {
			resErr.Write(w, r, http.StatusInternalServerError, "Internal Server Error #2")
			log.Print("ERR missing OPA data in ", rs)
			return
		}

		if !allow {
			resErr.Write(w, r, http.StatusUnauthorized,
				"Provide your JWT within the 'Authorization Bearer' HTTP header")
			log.Print("OPA: Missing or invalid Authorization header " + r.RemoteAddr + " " + r.RequestURI)
			return
		}

		next.ServeHTTP(w, r)
	})
}
