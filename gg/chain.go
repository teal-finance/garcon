// Copyright 2014 Justinas Stankevicius
// Copyright 2015 Alice contributors
// Copyright 2017 Sangjin Lee (sjlee)
// Copyright 2021 Teal.Finance/Garcon contributors
//
// This file is a modified copy from https://github.com/justinas/alice
// and also from https://github.com/justinas/alice/pull/40
//
// SPDX-License-Identifier: MIT

package gg

import (
	"net/http"
)

// Middleware is a constructor function returning a http.Handler.
type Middleware func(http.Handler) http.Handler

// RTMiddleware for a piece of RoundTrip middleware.
// Some middleware uses this out of the box,
// so in most cases you can just use somepackage.New().
type RTMiddleware func(http.RoundTripper) http.RoundTripper

// RoundTripperFunc is to RoundTripper what HandlerFunc is to Handler.
// It is a higher-order function that enables chaining of RoundTrippers
// with the middleware pattern.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip calls the function itself.
func (f RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Chain acts as a list of http.Handler middleware.
// Chain is effectively immutable:
// once created, it will always hold
// the same set of middleware in the same order.
type Chain []Middleware

// RTChain acts as a list of http.RoundTripper middlewares.
// RTChain is effectively immutable:
// once created, it will always hold
// the same set of middleware in the same order.
type RTChain []RTMiddleware

// NewChain creates a new chain,
// memorizing the given list of middleware.
// NewChain serves no other function,
// middleware is only constructed upon a call to chain.Then().
func NewChain(chain ...Middleware) Chain {
	return chain
}

// NewRTChain creates a new chain of HTTP round trippers,
// memorizing the given list of RoundTrip middlewares.
// NewRTChain serves no other function,
// middlewares are only called upon a call to Then().
func NewRTChain(chain ...RTMiddleware) RTChain {
	return chain
}

// Append extends a chain, adding the provided middleware
// as the last ones in the request flow.
//
//	chain := garcon.NewChain(m1, m2)
//	chain = chain.Append(m3, m4)
//	// requests in chain go m1 -> m2 -> m3 -> m4
func (c Chain) Append(chain ...Middleware) Chain {
	return append(c, chain...)
}

// Append extends a chain, adding the specified middlewares
// as the last ones in the request flow.
//
// Append returns a new chain, leaving the original one untouched.
//
// Example #1
//
//	stdChain := garcon.NewRTChain(m1, m2)
//	extChain := stdChain.Append(m3, m4)
//	// requests in stdChain go m1 -> m2
//	// requests in extChain go m1 -> m2 -> m3 -> m4
//
// Example #2
//
//	stdChain := garcon.NewRTChain(m1, m2)
//	ext1Chain := garcon.NewRTChain(m3, m4)
//	ext2Chain := stdChain.Append(ext1Chain...)
//	// requests in stdChain go  m1 -> m2
//	// requests in ext1Chain go m3 -> m4
//	// requests in ext2Chain go m1 -> m2 -> m3 -> m4
//
// Example #3
//
//	aHtmlAfterNosurf := garcon.NewRTChain(m2)
//	aHtml := garcon.NewRTChain(m1, func(rt http.RoundTripper) http.RoundTripper {
//		csrf := nosurf.New(rt)
//		csrf.SetFailureHandler(aHtmlAfterNosurf.ThenFunc(csrfFail))
//		return csrf
//	}).Append(aHtmlAfterNosurf)
//	// requests to aHtml hitting nosurfs success handler go: m1 -> nosurf -> m2 -> rt
//	// requests to aHtml hitting nosurfs failure handler go: m1 -> nosurf -> m2 -> csrfFail
func (c RTChain) Append(chain ...RTMiddleware) RTChain {
	return append(c, chain...)
}

// Then chains the middlewares and returns the final http.Handler.
//
//	garcon.NewChain(m1, m2, m3).Then(h)
//
// is equivalent to:
//
//	m1(m2(m3(h)))
//
// When the request comes in, it will be passed to m1, then m2, then m3
// and finally, the given handler
// (assuming every middleware calls the following one).
//
// A chain can be safely reused by calling Then() several times.
//
//	chain := garcon.NewChain(rateLimitHandler, csrfHandler)
//	indexPipe = chain.Then(indexHandler)
//	authPipe  = chain.Then(authHandler)
//
// Note: every call to Then() calls all middleware pieces.
// Thus several instances of the same middleware will be created
// when a chain is reused in this previous example.
// For proper middleware, this should cause no problem.
//
// Then() treats nil as http.DefaultServeMux.
func (c Chain) Then(handler http.Handler) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}

	for i := range c {
		middleware := c[len(c)-1-i]
		if middleware != nil {
			handler = middleware(handler)
		}
	}

	return handler
}

// Then chains the middleware and returns the final http.RoundTripper.
//
//	garcon.NewRTChain(m1, m2, m3).Then(rt)
//
// is equivalent to:
//
//	m1(m2(m3(rt)))
//
// When the request goes out, it will be passed to m1, then m2, then m3
// and finally, the given round tripper
// (assuming every middleware calls the following one).
//
// A chain can be safely reused by calling Then() several times.
//
//	stdStack := garcon.NewRTChain(rateLimitHandler, csrfHandler)
//	indexPipe = stdStack.Then(indexHandler)
//	authPipe = stdStack.Then(authHandler)
//
// Note that middlewares are called on every call to Then()
// and thus several instances of the same middleware will be created
// when a chain is reused in this way.
// For proper middleware, this should cause no problems.
//
// Then() treats nil as http.DefaultTransport.
func (c RTChain) Then(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}

	for i := range c {
		middleware := c[len(c)-1-i]
		if middleware != nil {
			rt = middleware(rt)
		}
	}

	return rt
}

// ThenFunc works identically to Then, but takes
// a HandlerFunc instead of a Handler.
//
// The following two statements are equivalent:
//
//	c.Then(http.HandlerFunc(fn))
//	c.ThenFunc(fn)
//
// ThenFunc provides all the guarantees of Then.
func (c Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	// This nil check cannot be removed due to the "nil is not nil"
	// https://stackoverflow.com/q/21460787 https://yourbasic.org/golang/gotcha-why-nil-error-not-equal-nil/0000
	if fn == nil {
		return c.Then(nil)
	}
	return c.Then(fn)
}

// ThenFunc works identically to Then, but takes
// a RoundTripperFunc instead of a RoundTripper.
//
// The following two statements are equivalent:
//
//	c.Then(http.RoundTripperFunc(fn))
//	c.ThenFunc(fn)
//
// ThenFunc provides all the guarantees of Then.
func (c RTChain) ThenFunc(fn RoundTripperFunc) http.RoundTripper {
	// This nil check cannot be removed due to the "nil is not nil"
	// https://stackoverflow.com/q/21460787 https://yourbasic.org/golang/gotcha-why-nil-error-not-equal-nil/0000
	if fn == nil {
		return c.Then(nil)
	}
	return c.Then(fn)
}
