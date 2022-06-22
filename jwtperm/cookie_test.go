// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

// Package jwtperm delivers and checks the JWT permissions
package jwtperm_test

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/jwtperm"
	"github.com/teal-finance/garcon/reserr"
)

type next struct{ called bool }

func (n *next) ServeHTTP(http.ResponseWriter, *http.Request) { n.called = true }

func TestNew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		addresses   []string
		resErr      reserr.ResErr
		secretHex   string
		permissions []any
		// want     *Checker
	}{{
		name:        "TAPI",
		addresses:   []string{"http://my-dns.co"},
		resErr:      reserr.New("http://my-dns.co/doc"),
		secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions: []any{"Anonymous", 6, "Personal", 48, "Enterprise", 0},
	}}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			urls := garcon.ParseURLs(c.addresses)
			secretKey, err := hex.DecodeString(c.secretHex)
			if err != nil {
				log.Panic(err)
			}

			checker := jwtperm.New(urls, c.resErr, secretKey, c.permissions...)
			cookie := checker.Cookie(0)

			req, _ := http.NewRequestWithContext(context.Background(), "GET", c.addresses[0], http.NoBody)
			req.AddCookie(cookie)

			w := httptest.NewRecorder()

			next := &next{called: false}
			handler := checker.Chk(next)
			handler.ServeHTTP(w, req)

			if !next.called {
				t.Errorf("checker.Chk() has not called next.ServeHTTP()")
			}
		})
	}
}
