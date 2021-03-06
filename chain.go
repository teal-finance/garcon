// Copyright 2014 Justinas Stankevicius
// Copyright 2015 Alice contributors
// Copyright 2021 Teal.Finance/Garcon contributors
//
// This file is a modified copy from https://github.com/justinas/alice
//
// SPDX-License-Identifier: MIT

package garcon

import (
	"net/http"
)

// Middleware is a constructor function returning a http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain acts as a list of http.Handler middleware.
// Chain is effectively immutable:
// once created, it will always hold
// the same set of middleware in the same order.
type Chain []Middleware

// NewChain creates a new chain,
// memorizing the given list of middleware.
// NewChain serves no other function,
// middleware is only constructed upon a call to Then().
func NewChain(chain ...Middleware) Chain {
	return chain
}

// Append extends a chain, adding the specified middleware
// as the last ones in the request flow.
//
//     chain := chain.New(m1, m2)
//     chain = chain.Append(m3, m4)
//     // requests in chain go m1 -> m2 -> m3 -> m4
func (c Chain) Append(chain ...Middleware) Chain {
	return append(c, chain...)
}

// Then chains the middleware and returns the final http.Handler.
//
//     chain.New(m1, m2, m3).Then(h)
//
// is equivalent to:
//
//     m1(m2(m3(h)))
//
// When the request comes in, it will be passed to m1, then m2, then m3
// and finally, the given handler
// (assuming every middleware calls the following one).
//
// A chain can be safely reused by calling Then() several times.
//
//     chain := chain.New(rateLimitHandler, csrfHandler)
//     indexPipe = chain.Then(indexHandler)
//     authPipe  = chain.Then(authHandler)
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
		handler = c[len(c)-1-i](handler)
	}
	return handler
}

// ThenFunc works identically to Then, but takes
// a HandlerFunc instead of a Handler.
//
// The following two statements are equivalent:
//
//     c.Then(http.HandlerFunc(fn))
//     c.ThenFunc(fn)
//
// ThenFunc provides all the guarantees of Then.
func (c Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	return c.Then(fn)
}
