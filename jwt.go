// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/teal-finance/garcon/timex"
	"github.com/teal-finance/quid/tokens"
)

const (
	// authScheme is part of the HTTP "Authorization" header
	// conveying the "Bearer Token" defined by RFC 6750 as
	// a security token with the property that any party in possession of
	// the token (a "bearer") can use the token in any way that any other
	// party in possession of it can.  Using a bearer token does not
	// require a bearer to prove possession of cryptographic key material
	// (proof-of-possession).
	authScheme        = "Bearer "
	defaultCookieName = "g" // g as in garcon
	// DefaultPlan is the plan name in absence of "permissions" parametric parameters.
	DefaultPlan = "VIP"
	// DefaultPerm is the perm value in absence of "permissions" parametric parameters.
	DefaultPerm = 1
)

var (
	ErrExpiredToken    = errors.New("expired or invalid access token")
	ErrJWTSignature    = errors.New("JWT signature mismatch")
	ErrNoAuthorization = errors.New("provide your JWT within the 'Authorization Bearer' HTTP header")
	ErrNoBase64JWT     = errors.New("the token claims (second part of the JWT) is not base64-valid")
	ErrNoBearer        = errors.New("malformed HTTP Authorization, must be Bearer")
	ErrNoValidJWT      = errors.New("cannot find a valid JWT in either the cookie or the first 'Authorization' HTTP header")
)

type Perm struct {
	Value int
}

type JWTChecker struct {
	gw        Writer
	tokenizer tokens.Tokenizer
	perms     []Perm
	plans     []string
	cookies   []http.Cookie
}

func NewJWTChecker(gw Writer, urls []*url.URL, secretKey string, permissions ...any) *JWTChecker {
	plans, perms := checkParameters(permissions...)

	tokenizer, err := tokens.NewHMAC(secretKey)
	if err != nil {
		log.Panic(err)
	}

	ck := &JWTChecker{
		gw:        gw,
		tokenizer: tokenizer,
		plans:     plans,
		perms:     perms,
		cookies:   make([]http.Cookie, len(plans)),
	}

	secure, dns, dir := extractCookieAttributes(urls)
	dns, name := forgeCookieName(secure, dns, dir)
	for i := range plans {
		ck.cookies[i] = ck.NewCookie(name, plans[i], "", secure, dns, dir)
	}

	return ck
}

func NewAccessToken(maxTTL, user string, groups, orgs []string, hexKey string) string {
	if len(hexKey) != 64 {
		log.Panic("Want HMAC-SHA256 key composed by 64 hexadecimal digits, but got", len(hexKey))
	}

	binKey, err := hex.DecodeString(hexKey)
	if err != nil {
		log.Panic("Cannot decode the HMAC-SHA256 key, please provide 64 hexadecimal digits:", err)
	}

	token, err := tokens.GenAccessToken(maxTTL, maxTTL, user, groups, orgs, binKey)
	if err != nil || token == "" {
		log.Panic("Cannot create JWT:", err)
	}

	return token
}

func (ck *JWTChecker) NewCookie(name, plan, user string, secure bool, dns, dir string) http.Cookie {
	JWT, err := ck.tokenizer.GenAccessToken("1y", "1y", user, []string{plan}, nil)
	if err != nil || JWT == "" {
		log.Panic("Cannot create JWT:", err)
	}

	log.Info("JWT newCookie plan="+plan+" domain="+dns+
		" path="+dir+" secure=", secure, " "+name+"="+JWT)

	return http.Cookie{
		Name:       name,
		Value:      JWT,
		Path:       dir,
		Domain:     dns,
		Expires:    time.Time{},
		RawExpires: "",
		MaxAge:     timex.YearSec,
		Secure:     secure,
		HttpOnly:   true,
		SameSite:   http.SameSiteStrictMode,
		Raw:        "",
		Unparsed:   nil,
	}
}

// Cookie returns a default cookie to facilitate testing.
func (ck *JWTChecker) Cookie(i int) *http.Cookie {
	if (i < 0) || (i >= len(ck.cookies)) {
		return nil
	}
	return &ck.cookies[i]
}

// Set is a middleware putting a HttpOnly cookie in the HTTP response header
// when no valid cookie is present.
// The new cookie conveys the JWT of the first plan.
// Set also puts the permission from the JWT in the request context.
func (ck *JWTChecker) Set(next http.Handler) http.Handler {
	if len(ck.cookies) == 0 {
		log.Panic("Middleware JWT requires at least one cookie")
	}
	log.Infof("Middleware JWT.Set cookie %s=%s MaxAge=%d",
		ck.cookies[0].Name, ck.cookies[0].Value, ck.cookies[0].MaxAge)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, a := ck.PermFromCookie(r)
		if a != nil {
			perm = ck.perms[0]
			ck.cookies[0].Expires = time.Now().Add(timex.YearNs)
			http.SetCookie(w, &ck.cookies[0])
		}

		next.ServeHTTP(w, perm.PutInCtx(r))
	})
}

// Chk is a middleware to accept only HTTP requests having a valid cookie.
// Then, Chk puts the permission (of the JWT) in the request context.
func (ck *JWTChecker) Chk(next http.Handler) http.Handler {
	log.Info("Middleware JWT.Chk cookie")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, a := ck.PermFromCookie(r)
		if a != nil {
			ck.gw.WriteErr(w, r, http.StatusUnauthorized, a...)
			return
		}

		next.ServeHTTP(w, perm.PutInCtx(r))
	})
}

// Vet is a middleware to accept only the HTTP request having a valid JWT.
// The JWT can be either in the cookie or in the first "Authorization" header.
// Then, Vet puts the permission (of the JWT) in the request context.
func (ck *JWTChecker) Vet(next http.Handler) http.Handler {
	log.Info("Middleware JWT.Vet cookie/bearer")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, a := ck.PermFromBearerOrCookie(r)
		if a != nil {
			ck.gw.WriteErr(w, r, http.StatusUnauthorized, a...)
			return
		}

		next.ServeHTTP(w, perm.PutInCtx(r))
	})
}

func checkParameters(permissions ...any) ([]string, []Perm) {
	n := len(permissions)
	if n == 0 {
		return []string{DefaultPlan}, []Perm{{Value: DefaultPerm}}
	}
	if n%2 != 0 {
		log.Panicf("The number %d of parametric arguments in NewChecker() must be even, "+
			"permissions must alternate string and int: plan1, perm1, plan2, perm2...", n)
	}

	n /= 2
	plans := make([]string, n)
	perms := make([]Perm, n)
	for i, p := range permissions {
		var ok bool
		if i%2 == 0 {
			plans[i/2], ok = p.(string)
		} else {
			var v int
			v, ok = p.(int)
			perms[i/2] = Perm{Value: v}
		}
		if !ok {
			log.Panic("Wrong type for the parametric arguments in NewChecker(), " +
				"permissions must alternate string and int: plan1, perm1, plan2, perm2...")
		}
	}

	return plans, perms
}

func extractCookieAttributes(urls []*url.URL) (secure bool, dns, dir string) {
	if len(urls) == 0 {
		log.Panic("No urls => Cannot set Cookie domain")
	}

	u := urls[0]
	if u == nil {
		log.Panic("Unexpected nil in URL slide:", urls)
	}

	switch {
	case u.Scheme == "http":
		secure = false
	case u.Scheme == "https":
		secure = true
	default:
		log.Panic("Unexpected URL scheme in", u)
	}

	dns = u.Hostname()

	dir = path.Clean(dir)
	if dir == "." {
		dir = "/"
	}

	return secure, dns, dir
}

// forgeCookieName returns the sanitized path and
// a nice cookie name deduced from the path basename.
func forgeCookieName(secure bool, dns, dir string) (domain, name string) {
	name = defaultCookieName
	for i := len(dir) - 2; i >= 0; i-- {
		if dir[i] == byte('/') {
			name = dir[i+1:]
			break
		}
	}

	if secure {
		if dir == "/" {
			// "__Host-" is when cookie has "Secure" flag, has no "Domain", has "Path=/" and is sent from a secure origin.
			name = "__Host-" + name
			dns = ""
		} else {
			// "__Secure-" is when cookie has "Secure" flag and is sent from a secure origin
			// "__Host-" is safer than the "__Secure-" prefix.
			name = "__Secure-" + name
		}
	}

	return dns, name
}

func (ck *JWTChecker) PermFromBearerOrCookie(r *http.Request) (Perm, []any) {
	JWT, errBearer := ck.jwtFromBearer(r)
	if errBearer != nil {
		c, errCookie := r.Cookie(ck.cookies[0].Name)
		if errCookie != nil {
			return Perm{}, []any{
				ErrNoValidJWT,
				"expected_cookie_name", ck.cookies[0].Name,
				"error_bearer", errBearer,
				"error_cookie", errCookie,
			}
		}
		JWT = c.Value
	}
	return ck.PermFromJWT(JWT)
}

func (ck *JWTChecker) jwtFromBearer(r *http.Request) (string, error) {
	// simple: check only the first header "Authorization"
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", ErrNoAuthorization
	}

	n := len(authScheme)
	if len(auth) > n && auth[:n] == authScheme {
		return auth[n:], nil
	}

	return "", ErrNoBearer
}

func (ck *JWTChecker) PermFromCookie(r *http.Request) (Perm, []any) {
	c, err := r.Cookie(ck.cookies[0].Name)
	if err != nil {
		return Perm{}, []any{err}
	}
	return ck.PermFromJWT(c.Value)
}

func (ck *JWTChecker) PermFromJWT(JWT string) (Perm, []any) {
	for i := range ck.cookies {
		if JWT == ck.cookies[i].Value {
			return ck.perms[i], nil
		}
	}

	claims, err := ck.tokenizer.Claims(JWT)
	if err != nil {
		return Perm{}, []any{err}
	}

	perm, err := ck.permFromAccessClaims(claims)
	if err != nil {
		return perm, []any{err}
	}

	return perm, nil
}

func (ck *JWTChecker) permFromAccessClaims(claims *tokens.AccessClaims) (Perm, error) {
	for i := range claims.Groups {
		for j := range ck.plans {
			if claims.Groups[i] == ck.plans[j] {
				return ck.perms[j], nil
			}
		}
	}

	// fallback: try to convert one of the groups into a permission
	for i := range claims.Groups {
		v, err := strconv.Atoi(claims.Groups[i])
		if err == nil {
			return Perm{Value: v}, nil
		}
	}

	return Perm{}, fmt.Errorf("cannot find any of %v within the JWT claims groups=%s, and cannot convert any group to integer",
		Sanitize(claims.Groups...), ck.plans)
}

// --------------------------------------
// Read/write permissions to/from context

//nolint:gochecknoglobals // permKey is a Context key and need to be global
var permKey struct{}

// PermFromCtx gets the permission information from the request context.
func PermFromCtx(r *http.Request) Perm {
	perm, ok := r.Context().Value(permKey).(Perm)
	if !ok {
		log.Warn("JWT no permission in context", r.URL.Path)
	}
	return perm
}

// PutInCtx stores the permission info within the request context.
func (perm Perm) PutInCtx(r *http.Request) *http.Request {
	parent := r.Context()
	child := context.WithValue(parent, permKey, perm)
	return r.WithContext(child)
}
