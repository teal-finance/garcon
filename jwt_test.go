// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/gg"
	"github.com/teal-finance/quid/tokens"
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
	name         string
	addresses    []string
	gw           garcon.Writer
	secretHex    string
	nameAndPerms []any
	cookieName   string
	plan         string
	perm         int
	shouldPanic  bool
}{{
	name:         "0plans",
	addresses:    []string{"http://my-dns.co"},
	gw:           garcon.NewWriter("http://my-dns.co/doc"),
	secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	nameAndPerms: nil,
	cookieName:   "my-dns",
	plan:         garcon.DefaultPlan,
	perm:         garcon.DefaultPerm,
	shouldPanic:  false,
}, {
	name:         "name-only",
	addresses:    []string{"http://my-dns.co"},
	gw:           garcon.NewWriter("http://my-dns.co/doc"),
	secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	nameAndPerms: []any{"my-cookie-name"},
	cookieName:   "my-cookie-name",
	plan:         garcon.DefaultPlan,
	perm:         garcon.DefaultPerm,
	shouldPanic:  false,
}, {
	name:         "1plans",
	addresses:    []string{"https://sub.dns.co/"},
	gw:           garcon.NewWriter("http://my-dns.co/doc"),
	secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	nameAndPerms: []any{"", "Anonymous", 6},
	cookieName:   "__Host-sub-dns",
	plan:         "Anonymous",
	perm:         6,
	shouldPanic:  false,
}, {
	name:         "bad-plans",
	addresses:    []string{"http://my-dns.co/dir"},
	gw:           garcon.NewWriter("doc"),
	secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	nameAndPerms: []any{"", "Anonymous", 6, "Personal"}, // len(permissions) is not even => panic
	cookieName:   "dir",
	plan:         "error",
	perm:         666,
	shouldPanic:  true,
}, {
	name:         "3plans",
	addresses:    []string{"http://sub.dns.co//./sss/..///-_-my.dir_-_.jpg///"},
	gw:           garcon.NewWriter("/doc"),
	secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	nameAndPerms: []any{"", "Anonymous", 6, "Personal", 48, "Enterprise", 0},
	cookieName:   "my-dir",
	plan:         "Personal",
	perm:         48,
	shouldPanic:  false,
}, {
	name:         "localhost",
	addresses:    []string{"http://localhost:8080/"},
	gw:           garcon.NewWriter(""),
	secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	nameAndPerms: []any{"", "Anonymous", 6, "Personal", 48, "Enterprise", -1},
	cookieName:   garcon.DefaultCookieName,
	plan:         "Enterprise",
	perm:         -1,
	shouldPanic:  false,
}, {
	name:         "customPlan",
	addresses:    []string{"https://my-dns.co:8080/my/sub/-_-my.dir_-_.jpg/"},
	gw:           garcon.NewWriter(""),
	secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
	nameAndPerms: []any{"", "Anonymous", 6, "Personal", 48, "Enterprise", -1},
	cookieName:   "__Secure-my-dir",
	plan:         "55",
	perm:         55,
	shouldPanic:  false,
}}

func TestNewJWTChecker(t *testing.T) {
	t.Parallel()

	for i, c := range cases {
		i := i
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			urls := gg.ParseURLs(c.addresses)

			if c.shouldPanic {
				defer func() { _ = recover() }()
			}

			ck := garcon.NewJWTChecker(c.gw, urls, c.secretHex, c.nameAndPerms...)

			if c.shouldPanic {
				t.Errorf("#%d NewChecker() did not panic", i)
			}

			if ck.Cookie(0).Name != c.cookieName {
				t.Errorf("#%d Want cookieName %q but got %q", i, c.cookieName, ck.Cookie(0).Name)
			}

			tokenizer, err := tokens.NewHMAC(c.secretHex, true)
			if err != nil {
				t.Fatalf("#%d tokens.NewHMAC err: %s", i, err)
			}
			cookie := garcon.NewCookie(tokenizer, c.cookieName, c.plan, "Jonh Doe", false, c.addresses[0][6:], "/")

			r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatalf("#%d NewRequestWithContext err: %s", i, err)
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
				t.Fatalf("#%d checker.Chk() %s", i, body)
			}
			if !next.called {
				t.Errorf("#%d checker.Chk() has not called next.ServeHTTP()", i)
			}
			if next.perm != c.perm {
				t.Errorf("#%d checker.Chk() request ctx perm got=%d want=%d", i, next.perm, c.perm)
			}

			r, err = http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatalf("#%d NewRequestWithContext err: %s", i, err)
			}
			r.AddCookie(&cookie)

			w = httptest.NewRecorder()
			next.called = false
			next.perm = 0
			handler = ck.Vet(next)
			handler.ServeHTTP(w, r)

			body = w.Body.String()
			if body != "" {
				t.Fatalf("#%d checker.Vet() %s", i, body)
			}
			if !next.called {
				t.Errorf("#%d checker.Vet() has not called next.ServeHTTP()", i)
			}
			if next.perm != c.perm {
				t.Errorf("#%d checker.Vet() request ctx perm got=%d want=%d", i, next.perm, c.perm)
			}

			r, err = http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatalf("#%d NewRequestWithContext err: %s", i, err)
			}

			w = httptest.NewRecorder()
			next.called = false
			next.perm = 0
			handler = ck.Set(next)
			handler.ServeHTTP(w, r)

			resp := w.Result()
			cookies := resp.Cookies()
			if len(cookies) != 1 {
				t.Errorf("#%d wants only one cookie, but checker.Set() provided %d", i, len(cookies))
			} else if cookies[0].Value != ck.Cookie(0).Value {
				t.Errorf("#%d checker.Set() has not used the first cookie", i)
			}
			if !next.called {
				t.Errorf("#%d checker.Vet() has not called next.ServeHTTP()", i)
			}
			if len(c.nameAndPerms) >= 3 {
				var ok bool
				c.perm, ok = c.nameAndPerms[2].(int)
				if !ok {
					t.Errorf("#%d c.nameAndPerms[2] is not int", i)
				}
			}
			if next.perm != c.perm {
				t.Errorf("#%d checker.Set() request ctx perm got=%d want=%d "+
					"len(permissions)=%d", i, next.perm, c.perm, len(c.nameAndPerms))
			}
			resp.Body.Close()
		})
	}
}
