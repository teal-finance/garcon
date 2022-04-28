// #region <editor-fold desc="Preamble">
// Copyright (c) 2020-2022 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon, an API and website server.
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 or any later version, at the licensee’s option.
// SPDX-License-Identifier: LGPL-3.0-or-later
//
// Teal.Finance/Garcon is distributed WITHOUT ANY WARRANTY.
// For more details, see the LICENSE file (alongside the source files)
// or online at <https://www.gnu.org/licenses/lgpl-3.0.html>
// #endregion </editor-fold>

// package opa manages the Open Policy Agent.
// see https://www.openpolicyagent.org/docs/edge/integration/#integrating-with-the-go-api
package opa

import (
	"context"
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
	Compiler *ast.Compiler
	ResErr   reserr.ResErr
}

// New creates a new Policy by loading rego files.
func New(filenames []string, reserr reserr.ResErr) (Policy, error) {
	compiler, err := Load(filenames)
	return Policy{compiler, reserr}, err
}

// Ready returns true if the Policy compiler contains rules.
func (opa Policy) Ready() bool {
	return opa.Compiler != nil
}

// Load check the Rego filenames and loads them to build the OPA compiler.
func Load(filenames []string) (*ast.Compiler, error) {
	if len(filenames) == 0 {
		return nil, nil
	}

	modules := map[string]string{}

	for _, f := range filenames {
		log.Printf("OPA: load %q", f)

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
	log.Print("Middleware OPA: ", opa.Compiler.Modules)

	compiler := opa.Compiler
	resErr := opa.ResErr

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
