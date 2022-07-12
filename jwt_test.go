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

var cases = []struct {
	name        string
	addresses   []string
	errWriter   garcon.ErrWriter
	secretHex   string
	permissions []any
	shouldPanic bool
}{{
	name:        "0plans",
	addresses:   []string{"http://my-dns.co"},
	errWriter:   garcon.NewErrWriter("http://my-dns.co/doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: nil,
	shouldPanic: false,
}, {
	name:        "1plans",
	addresses:   []string{"http://my-dns.co"},
	errWriter:   garcon.NewErrWriter("http://my-dns.co/doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6},
	shouldPanic: false,
}, {
	name:        "bad-plans",
	addresses:   []string{"http://my-dns.co"},
	errWriter:   garcon.NewErrWriter("http://my-dns.co/doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6, "Personal"}, // len(permissions) is not even => panic
	shouldPanic: true,
}, {
	name:        "3plans",
	addresses:   []string{"http://my-dns.co"},
	errWriter:   garcon.NewErrWriter("http://my-dns.co/doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6, "Personal", 48, "Enterprise", 0},
	shouldPanic: false,
}}

func TestNewChecker(t *testing.T) {
	t.Parallel()

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			urls := garcon.ParseURLs(c.addresses)
			secretKey, err := hex.DecodeString(c.secretHex)
			if err != nil {
				log.Panic(err)
			}

			if c.shouldPanic {
				defer func() { recover() }()
			}

			ck := garcon.NewChecker(urls, c.errWriter, secretKey, c.permissions...)

			if c.shouldPanic {
				t.Errorf("NewChecker() did not panic")
			}

			plan := garcon.DefaultPlanName
			if len(c.permissions) > 0 {
				n := (len(c.permissions) - 1) / 2
				plan = c.permissions[2*n].(string)
			}
			cookie := ck.NewCookie("g", plan, false, c.addresses[0][6:], "/")

			r, err := http.NewRequestWithContext(context.Background(), "GET", c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatal("NewRequestWithContext err:", err)
			}
			r.AddCookie(&cookie)

			w := httptest.NewRecorder()
			next := &next{called: false}
			handler := ck.Chk(next)
			handler.ServeHTTP(w, r)

			body := w.Body.String()
			if body != "" {
				t.Fatal("checker.Chk()", body)
			}
			if !next.called {
				t.Errorf("checker.Chk() has not called next.ServeHTTP()")
			}

			w = httptest.NewRecorder()
			next.called = false
			handler = ck.Vet(next)
			handler.ServeHTTP(w, r)

			body = w.Body.String()
			if body != "" {
				t.Fatal("checker.Vet()", body)
			}

			if !next.called {
				t.Errorf("checker.Vet() has not called next.ServeHTTP()")
			}

			r, err = http.NewRequestWithContext(context.Background(), "GET", c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatal("NewRequestWithContext err:", err)
			}
			w = httptest.NewRecorder()
			handler = ck.Set(next)
			handler.ServeHTTP(w, r)

			response := w.Result()
			cookies := response.Cookies()
			if len(cookies) != 1 {
				t.Error("checker.Set() has not set only one cookie, but ", len(cookies))
			} else if cookies[0].Value != ck.Cookie(0).Value {
				t.Error("checker.Set() has not used the first cookie")
			}
			response.Body.Close()
		})
	}
}
