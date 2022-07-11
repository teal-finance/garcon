// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon_test

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teal-finance/garcon"
)

type next struct{ called bool }

func (n *next) ServeHTTP(http.ResponseWriter, *http.Request) { n.called = true }

func TestNew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		addresses   []string
		errWriter   garcon.ErrWriter
		secretHex   string
		permissions []any
		// want     *Checker
	}{{
		name:        "TAPI",
		addresses:   []string{"http://my-dns.co"},
		errWriter:   garcon.NewErrWriter("http://my-dns.co/doc"),
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

			checker := garcon.NewChecker(urls, c.errWriter, secretKey, c.permissions...)
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
