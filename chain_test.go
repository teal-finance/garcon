// Copyright 2014 Justinas Stankevicius
// Copyright 2015 Alice contributors
// Copyright 2017 Sangjin Lee (sjlee)
// Copyright 2021 Teal.Finance/Garcon contributors
//
// This file is a modified copy from https://github.com/justinas/alice
// and also from https://github.com/justinas/alice/pull/40
//
// SPDX-License-Identifier: MIT

package garcon_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/teal-finance/garcon"
)

func TestNewChain(t *testing.T) {
	c1 := func(h http.Handler) http.Handler {
		return nil
	}

	c2 := func(h http.Handler) http.Handler {
		return http.StripPrefix("potato", nil)
	}

	slice := []garcon.Middleware{c1, c2}

	chain := garcon.NewChain(slice...)
	for k := range slice {
		if !funcsEqual(chain[k], slice[k]) {
			t.Error("garcon.NewChain does not add middlewares correctly")
		}
	}
}

func TestNewRTChain(t *testing.T) {
	c1 := func(h http.RoundTripper) http.RoundTripper {
		return nil
	}

	c2 := func(h http.RoundTripper) http.RoundTripper {
		return http.DefaultTransport
	}

	slice := []garcon.RTMiddleware{c1, c2}

	chain := garcon.NewRTChain(slice...)
	for k := range slice {
		if !funcsEqual(chain[k], slice[k]) {
			t.Error("garcon.NewRTChain does not add middlewares correctly")
		}
	}
}

func TestChain_Then_WorksWithNoMiddleware(t *testing.T) {
	if !funcsEqual(garcon.NewChain().Then(testApp), testApp) {
		t.Error("Then does not work with zero middleware")
	}
}

func TestRTChain_Then_WorksWithNoMiddleware(t *testing.T) {
	if !funcsEqual(garcon.NewRTChain().Then(testRoundTripApp), testRoundTripApp) {
		t.Error("Then does not work with zero middleware")
	}
}

func TestChain_Then_TreatsNilAsDefaultServeMux(t *testing.T) {
	if garcon.NewChain().Then(nil) != http.DefaultServeMux {
		t.Error("Then does not treat nil as DefaultServeMux")
	}
}

func TestChain_ThenFunc_TreatsNilAsDefaultServeMux(t *testing.T) {
	if garcon.NewChain().ThenFunc(nil) != http.DefaultServeMux {
		t.Error("ThenFunc does not treat nil as DefaultServeMux")
	}
}

func TestRTChain_Then_TreatsNilAsDefaultTransport(t *testing.T) {
	if garcon.NewRTChain().Then(nil) != http.DefaultTransport {
		t.Error("Then does not treat nil as DefaultTransport")
	}
}

func TestRTChain_ThenFunc_TreatsNilAsDefaultTransport(t *testing.T) {
	if garcon.NewRTChain().ThenFunc(nil) != http.DefaultTransport {
		t.Error("ThenFunc does not treat nil as DefaultTransport")
	}
}

func TestChain_ThenFunc_ConstructsHandlerFunc(t *testing.T) {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	chained := garcon.NewChain().ThenFunc(fn)
	rec := httptest.NewRecorder()

	chained.ServeHTTP(rec, (*http.Request)(nil))

	if reflect.TypeOf(chained) != reflect.TypeOf((http.HandlerFunc)(nil)) {
		t.Error("ThenFunc does not construct HandlerFunc")
	}
}

func TestRTChain_ThenFunc_ConstructsRoundTripperFunc(t *testing.T) {
	fn := garcon.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	})
	chained := garcon.NewRTChain().ThenFunc(fn)

	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	chained.RoundTrip(r)

	if reflect.TypeOf(chained) != reflect.TypeOf((garcon.RoundTripperFunc)(nil)) {
		t.Error("ThenFunc does not construct RoundTripperFunc")
	}
}

func TestChain_Then_OrdersHandlersCorrectly(t *testing.T) {
	t1 := tagMiddleware("t1\n")
	t2 := tagMiddleware("t2\n")
	t3 := tagMiddleware("t3\n")

	chained := garcon.NewChain(t1, t2, t3).Then(testApp)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	chained.ServeHTTP(w, r)

	if w.Body.String() != "t1\nt2\nt3\napp\n" {
		t.Error("Then does not order handlers correctly")
	}
}

func TestChain_Then_OrdersRoundTrippersCorrectly(t *testing.T) {
	t1 := tagRTMiddleware("t1\n")
	t2 := tagRTMiddleware("t2\n")
	t3 := tagRTMiddleware("t3\n")

	chained := garcon.NewRTChain(t1, t2, t3).Then(testRoundTripApp)

	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	chained.RoundTrip(r)

	body, err := bodyAsString(r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "t1\nt2\nt3\napp\n" {
		t.Error("Then does not order round trippers correctly")
	}
}

func TestChain_Append_AddsHandlersCorrectly1(t *testing.T) {
	chain := garcon.NewChain(tagMiddleware("t1\n"), tagMiddleware("t2\n"))
	newChain := chain.Append(tagMiddleware("t3\n"), tagMiddleware("t4\n"))

	if len(chain) != 2 {
		t.Error("chain should have 2 middlewares")
	}
	if len(newChain) != 4 {
		t.Error("newChain should have 4 middlewares")
	}

	chained := newChain.Then(testApp)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	chained.ServeHTTP(w, r)

	if w.Body.String() != "t1\nt2\nt3\nt4\napp\n" {
		t.Error("Append does not add handlers correctly")
	}
}

func TestRTChain_Append_AddsRoundTrippersCorrectly1(t *testing.T) {
	chain := garcon.NewRTChain(tagRTMiddleware("t1\n"), tagRTMiddleware("t2\n"))
	newChain := chain.Append(tagRTMiddleware("t3\n"), tagRTMiddleware("t4\n"))

	if len(chain) != 2 {
		t.Error("chain should have 2 middlewares")
	}
	if len(newChain) != 4 {
		t.Error("newChain should have 4 middlewares")
	}

	chained := newChain.Then(testRoundTripApp)

	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	chained.RoundTrip(r)

	body, err := bodyAsString(r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "t1\nt2\nt3\nt4\napp\n" {
		t.Error("Append does not add round trippers correctly")
	}
}

func TestChain_Append_AddsHandlersCorrectly2(t *testing.T) {
	chain1 := garcon.NewChain(tagMiddleware("t1\n"), tagMiddleware("t2\n"))
	chain2 := garcon.NewChain(tagMiddleware("t3\n"), tagMiddleware("t4\n"))
	newChain := chain1.Append(chain2...)

	if len(chain1) != 2 {
		t.Error("chain1 should contain 2 middlewares")
	}
	if len(chain2) != 2 {
		t.Error("chain2 should contain 2 middlewares")
	}
	if len(newChain) != 4 {
		t.Error("newChain should contain 4 middlewares")
	}

	chained := newChain.Then(testApp)

	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	chained.ServeHTTP(w, r)

	if w.Body.String() != "t1\nt2\nt3\nt4\napp\n" {
		t.Error("Append does not add handlers in correctly")
	}
}

func TestRTChain_Append_AddsRoundTrippersCorrectly2(t *testing.T) {
	chain1 := garcon.NewRTChain(tagRTMiddleware("t1\n"), tagRTMiddleware("t2\n"))
	chain2 := garcon.NewRTChain(tagRTMiddleware("t3\n"), tagRTMiddleware("t4\n"))
	newChain := chain1.Append(chain2...)

	if len(chain1) != 2 {
		t.Error("chain1 should contain 2 middlewares")
	}
	if len(chain2) != 2 {
		t.Error("chain2 should contain 2 middlewares")
	}
	if len(newChain) != 4 {
		t.Error("newChain should contain 4 middlewares")
	}

	chained := newChain.Then(testRoundTripApp)

	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	chained.RoundTrip(r)

	body, err := bodyAsString(r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "t1\nt2\nt3\nt4\napp\n" {
		t.Error("Append does not add round trippers in correctly")
	}
}

func TestChain_Append_RespectsImmutability1(t *testing.T) {
	chain := garcon.NewChain(tagMiddleware(""))
	newChain := chain.Append(tagMiddleware(""))

	if &chain[0] == &newChain[0] {
		t.Error("Append does not respect immutability")
	}
}

func TestRTChain_Append_RespectsImmutability1(t *testing.T) {
	chain := garcon.NewRTChain(tagRTMiddleware(""))
	newChain := chain.Append(tagRTMiddleware(""))

	if &chain[0] == &newChain[0] {
		t.Error("Append does not respect immutability")
	}
}

func TestChain_Append_RespectsImmutability2(t *testing.T) {
	chain := garcon.NewChain(tagMiddleware(""))
	newChain := chain.Append(garcon.NewChain(tagMiddleware(""))...)

	if &chain[0] == &newChain[0] {
		t.Error("Append does not respect immutability")
	}
}

func TestRTChain_Append_RespectsImmutability2(t *testing.T) {
	chain := garcon.NewRTChain(tagRTMiddleware(""))
	newChain := chain.Append(garcon.NewRTChain(tagRTMiddleware(""))...)

	if &chain[0] == &newChain[0] {
		t.Error("Append does not respect immutability")
	}
}

// tagMiddleware and tagRTMiddleware are constructors for middleware
// that writes its own "tag" into the request body and does nothing else.
// Useful in checking if a chain is behaving in the right order.
func tagMiddleware(tag string) garcon.Middleware {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tag))
			h.ServeHTTP(w, r)
		})
	}
}

func tagRTMiddleware(tag string) garcon.RTMiddleware {
	return func(rt http.RoundTripper) http.RoundTripper {
		return garcon.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			err := appendTag(tag, r)
			if err != nil {
				return nil, err
			}
			return rt.RoundTrip(r)
		})
	}
}

// Not recommended (https://golang.org/pkg/reflect/#Value.Pointer),
// but the best we can do.
func funcsEqual(f1, f2 interface{}) bool {
	val1 := reflect.ValueOf(f1)
	val2 := reflect.ValueOf(f2)
	return val1.Pointer() == val2.Pointer()
}

var testApp = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("app\n"))
})

var testRoundTripApp = garcon.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
	appendTag("app\n", r)
	return &http.Response{}, nil
})

func appendTag(tag string, r *http.Request) error {
	var newBody []byte
	if r.Body == nil {
		newBody = []byte(tag)
	} else {
		body, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			return err
		}
		newBody = append(body, []byte(tag)...)
	}
	r.Body = ioutil.NopCloser(bytes.NewBuffer(newBody))
	return nil
}

func bodyAsString(r *http.Request) (string, error) {
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return "", err
	}
	return string(body[:]), nil
}
