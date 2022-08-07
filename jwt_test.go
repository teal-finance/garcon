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

type next struct {
	called bool
	perm   int
}

func (next *next) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	next.called = true
	next.perm = garcon.PermFromCtx(r).Value
}

var cases = []struct {
	name        string
	addresses   []string
	gw          garcon.Writer
	secretHex   string
	permissions []any
	plan        string
	perm        int
	shouldPanic bool
}{{
	name:        "0plans",
	addresses:   []string{"http://my-dns.co"},
	gw:          garcon.NewWriter("http://my-dns.co/doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: nil,
	plan:        garcon.DefaultPlan,
	perm:        garcon.DefaultPerm,
	shouldPanic: false,
}, {
	name:        "1plans",
	addresses:   []string{"http://my-dns.co/"},
	gw:          garcon.NewWriter("http://my-dns.co/doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6},
	plan:        "Anonymous",
	perm:        6,
	shouldPanic: false,
}, {
	name:        "bad-plans",
	addresses:   []string{"http://my-dns.co/g"},
	gw:          garcon.NewWriter("doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6, "Personal"}, // len(permissions) is not even => panic
	plan:        "error",
	perm:        666,
	shouldPanic: true,
}, {
	name:        "3plans",
	addresses:   []string{"http://my-dns.co//./sss/..///g///"},
	gw:          garcon.NewWriter("/doc"),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6, "Personal", 48, "Enterprise", 0},
	plan:        "Personal",
	perm:        48,
	shouldPanic: false,
}, {
	name:        "localhost",
	addresses:   []string{"http://localhost:8080/"},
	gw:          garcon.NewWriter(""),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6, "Personal", 48, "Enterprise", -1},
	plan:        "Enterprise",
	perm:        -1,
	shouldPanic: false,
}, {
	name:        "customPlan",
	addresses:   []string{"http://localhost:8080/"},
	gw:          garcon.NewWriter(""),
	secretHex:   "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	permissions: []any{"Anonymous", 6, "Personal", 48, "Enterprise", -1},
	plan:        "55",
	perm:        55,
	shouldPanic: false,
}}

func TestNewJWTChecker(t *testing.T) {
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
				defer func() { _ = recover() }()
			}

			ck := garcon.NewJWTChecker(urls, c.gw, secretKey, c.permissions...)

			if c.shouldPanic {
				t.Errorf("NewChecker() did not panic")
			}

			cookie := ck.NewCookie("g", c.plan, "Jonh Doe", false, c.addresses[0][6:], "/")

			r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatal("NewRequestWithContext err:", err)
			}
			r.AddCookie(&cookie)

			w := httptest.NewRecorder()
			next := &next{
				called: false,
				perm:   0,
			}
			handler := ck.Chk(next)
			handler.ServeHTTP(w, r)

			body := w.Body.String()
			if body != "" {
				t.Fatal("checker.Chk()", body)
			}
			if !next.called {
				t.Errorf("checker.Chk() has not called next.ServeHTTP()")
			}
			if next.perm != c.perm {
				t.Errorf("checker.Chk() request ctx perm got=%d want=%d", next.perm, c.perm)
			}

			r, err = http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatal("NewRequestWithContext err:", err)
			}
			r.AddCookie(&cookie)

			w = httptest.NewRecorder()
			next.called = false
			next.perm = 0
			handler = ck.Vet(next)
			handler.ServeHTTP(w, r)

			body = w.Body.String()
			if body != "" {
				t.Fatal("checker.Vet()", body)
			}
			if !next.called {
				t.Errorf("checker.Vet() has not called next.ServeHTTP()")
			}
			if next.perm != c.perm {
				t.Errorf("checker.Vet() request ctx perm got=%d want=%d", next.perm, c.perm)
			}

			r, err = http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatal("NewRequestWithContext err:", err)
			}

			w = httptest.NewRecorder()
			next.called = false
			next.perm = 0
			handler = ck.Set(next)
			handler.ServeHTTP(w, r)

			response := w.Result()
			cookies := response.Cookies()
			if len(cookies) != 1 {
				t.Error("checker.Set() has not set only one cookie, but ", len(cookies))
			} else if cookies[0].Value != ck.Cookie(0).Value {
				t.Error("checker.Set() has not used the first cookie")
			}
			if !next.called {
				t.Errorf("checker.Vet() has not called next.ServeHTTP()")
			}
			if len(c.permissions) >= 2 {
				var ok bool
				c.perm, ok = c.permissions[1].(int)
				if !ok {
					t.Errorf("c.permissions[1] is not int")
				}
			}
			if next.perm != c.perm {
				t.Errorf("checker.Set() request ctx perm got=%d want=%d "+
					"len(permissions)=%d", next.perm, c.perm, len(c.permissions))
			}
			response.Body.Close()
		})
	}
}
