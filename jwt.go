// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"context"
	"crypto"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"

	"github.com/teal-finance/garcon/timex"
	"github.com/teal-finance/quid/quidlib/tokens"
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
	defaultPlanName   = "DefaultPlan"
	defaultPermValue  = 1
)

var (
	ErrExpiredToken    = errors.New("expired or invalid refresh token")
	ErrJWTSignature    = errors.New("JWT signature mismatch")
	ErrNoAuthorization = errors.New("provide your JWT within the 'Authorization Bearer' HTTP header")
	ErrNoBase64JWT     = errors.New("the token claims (second part of the JWT) is not base64-valid")
	ErrNoBearer        = errors.New("malformed HTTP Authorization, must be Bearer")
)

type Perm struct {
	Value int
}

type Checker struct {
	errWriter  ErrWriter
	secretKey  []byte
	perms      []Perm
	plans      []string
	cookies    []http.Cookie
	devOrigins []string
}

func NewChecker(urls []*url.URL, errWriter ErrWriter, secretKey []byte, permissions ...any) *Checker {
	plans, perms := checkParameters(secretKey, permissions)

	c := &Checker{
		errWriter:  errWriter,
		secretKey:  secretKey,
		plans:      plans,
		perms:      perms,
		cookies:    make([]http.Cookie, len(plans)),
		devOrigins: extractDevOrigins(urls),
	}

	secure, dns, path, name := extractMainDomain(urls)
	for i := range plans {
		c.cookies[i] = c.NewCookie(name, plans[i], secure, dns, path)
	}

	return c
}

func checkParameters(secretKey []byte, permissions ...any) ([]string, []Perm) {
	if len(secretKey) != 32 {
		log.Panic("Want HMAC-SHA256 key containing 32 bytes, but got ", len(secretKey))
	}

	n := len(permissions) / 2
	if n == 0 {
		return []string{defaultPlanName}, []Perm{{Value: defaultPermValue}}
	}

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
				"must alternate string and int: plan1, perm1, plan2, perm2...")
		}
	}

	return plans, perms
}

func extractMainDomain(urls []*url.URL) (secure bool, dns, path, name string) {
	if len(urls) == 0 {
		log.Panic("No urls => Cannot set Cookie domain")
	}

	u := urls[0]
	if u == nil {
		log.Panic("Unexpected nil in URL slide: ", urls)
	}

	switch {
	case u.Scheme == "http":
		secure = false
	case u.Scheme == "https":
		secure = true
	default:
		log.Panic("Unexpected URL scheme in ", u)
	}

	path, name = cleanPath(u.Path)
	return secure, u.Hostname(), path, name
}

// cleanPath returns the sanitized path and
// a nice cookie name deduced from the path basename.
func cleanPath(path string) (_, name string) {
	name = defaultCookieName

	if path != "" {
		// remove trailing slash
		if path[len(path)-1] == '/' {
			path = path[:len(path)-1]
		}

		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == byte('/') {
				name = path[i+1:]
				break
			}
		}
	}

	if path == "" {
		path = "/"
	}

	return path, name
}

func extractDevURLs(urls []*url.URL) []*url.URL {
	if len(urls) == 1 {
		log.Print("JWT required for single domain: ", urls)
		return nil
	}

	for i, u := range urls {
		if u == nil {
			log.Panic("Unexpected nil in URL slide: ", urls)
		}
		if u.Scheme == "http" {
			return urls[i:]
		}
	}

	return nil
}

func extractDevOrigins(urls []*url.URL) []string {
	if len(urls) > 0 && urls[0].Scheme == "http" {
		host, _, _ := net.SplitHostPort(urls[0].Host)
		if host == "localhost" {
			log.Print("JWT not required for http://localhost")
			return []string{"*"}
		}
	}

	devURLS := extractDevURLs(urls)

	if len(devURLS) == 0 {
		return nil
	}

	devOrigins := make([]string, 0, len(urls))
	for _, u := range urls {
		o := u.Scheme + "://" + u.Host
		devOrigins = append(devOrigins, o)
	}

	log.Print("JWT not required for dev. origins: ", devOrigins)
	return devOrigins
}

func (ck *Checker) NewCookie(name, plan string, secure bool, dns, path string) http.Cookie {
	JWT, err := tokens.GenRefreshToken("1y", "1y", plan, "", ck.secretKey)
	if err != nil || JWT == "" {
		log.Panic("Cannot create JWT: ", err)
	}

	log.Print("newCookie plan="+plan+" domain="+dns+
		" path="+path+" secure=", secure, " "+name+"="+JWT)

	return http.Cookie{
		Name:       name,
		Value:      JWT,
		Path:       path,
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

// Cookie returns the internal cookie (for test purpose).
func (ck *Checker) Cookie(i int) *http.Cookie {
	if (i < 0) || (i >= len(ck.cookies)) {
		return nil
	}
	return &ck.cookies[i]
}

// Set puts a HttpOnly cookie in the HTTP response header
// when no valid cookie is present.
// The new cookie conveys the JWT of the first plan.
// Set also puts the permission from the JWT in the request context.
func (ck *Checker) Set(next http.Handler) http.Handler {
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

// Chk only accepts HTTP requests having a valid cookie.
// Then, Chk puts the permission (of the JWT) in the request context.
func (ck *Checker) Chk(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, err := ck.PermFromCookie(r)
		if err != nil {
			if ck.isDevOrigin(r) {
				perm = ck.perms[0]
			} else {
				ck.errWriter.Write(w, r, http.StatusUnauthorized, err...)
				return
			}
		}

		next.ServeHTTP(w, perm.PutInCtx(r))
	})
}

// Vet only accepts the HTTP request having a valid JWT.
// The JWT can be either in the cookie or in the first "Authorization" header.
// Then, Vet puts the permission (of the JWT) in the request context.
func (ck *Checker) Vet(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, err := ck.PermFromBearerOrCookie(r)
		if err != nil {
			if ck.isDevOrigin(r) {
				perm = ck.perms[0]
			} else {
				ck.errWriter.Write(w, r, http.StatusUnauthorized, err...)
				return
			}
		}

		next.ServeHTTP(w, perm.PutInCtx(r))
	})
}

func (ck *Checker) isDevOrigin(r *http.Request) bool {
	if len(ck.devOrigins) == 0 {
		return false
	}

	if len(ck.devOrigins) > 0 {
		origin := r.Header.Get("Origin")
		for _, prefix := range ck.devOrigins {
			if prefix == "*" {
				return true
			}
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}
	}

	return false
}

func (ck *Checker) PermFromBearerOrCookie(r *http.Request) (Perm, []any) {
	JWT, errBearer := ck.jwtFromBearer(r)
	if errBearer != nil {
		c, errCookie := r.Cookie(ck.cookies[0].Name)
		if errCookie != nil {
			return Perm{}, []any{
				fmt.Errorf("cannot find a valid JWT in either "+
					"the first 'Authorization' HTTP header or "+
					"the cookie '%s'", ck.cookies[0].Name),
				"error_bearer", errBearer,
				"error_cookie", errCookie,
			}
		}
		JWT = c.Value
	}
	return ck.PermFromJWT(JWT)
}

func (ck *Checker) jwtFromBearer(r *http.Request) (string, error) {
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

func (ck *Checker) PermFromCookie(r *http.Request) (Perm, []any) {
	c, err := r.Cookie(ck.cookies[0].Name)
	if err != nil {
		return Perm{}, []any{err}
	}
	return ck.PermFromJWT(c.Value)
}

func (ck *Checker) PermFromJWT(JWT string) (Perm, []any) {
	for i := range ck.cookies {
		if JWT == ck.cookies[i].Value {
			return ck.perms[i], nil
		}
	}

	claimsJSON, err := ck.claimsFromJWT(JWT)
	if err != nil {
		return Perm{}, []any{err}
	}

	perm, err := ck.permFromRefreshBytes(claimsJSON)
	if err != nil {
		return perm, []any{ErrExpiredToken}
	}

	return perm, nil
}

func (ck *Checker) permFromRefreshClaims(claims *tokens.RefreshClaims) (Perm, error) {
	for i := range ck.plans {
		if claims.Namespace == ck.plans[i] {
			return ck.perms[i], nil
		}
	}

	// Try to convert the Namespace into permission
	v, err := strconv.Atoi(claims.Namespace)
	if err != nil {
		return Perm{}, fmt.Errorf("the JWT claims has plan '%s' but should be in %v",
			Sanitize(claims.Namespace), ck.plans)
	}

	return Perm{Value: v}, nil
}

func (ck *Checker) claimsFromJWT(JWT string) ([]byte, error) {
	// decompose JWT in three parts
	parts := strings.Split(JWT, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("the JWT consists in %d parts, "+
			"must be 3 parts separated by dots", len(parts))
	}

	if err := ck.verifySignature(parts); err != nil {
		return nil, err
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrNoBase64JWT
	}

	return claimsJSON, nil
}

// verifySignature of HS256 tokens.
func (ck *Checker) verifySignature(parts []string) error {
	signingTxt := strings.Join(parts[0:2], ".")
	signature := ck.sign(signingTxt)
	if signature != parts[2] { // parts[2] = JWT signature
		return ErrJWTSignature
	}
	return nil
}

func (ck *Checker) permFromRefreshBytes(claimsJSON []byte) (Perm, error) {
	claims := tokens.RefreshClaims{
		Namespace: "",
		UserName:  "",
		StandardClaims: jwt.StandardClaims{
			Audience:  "",
			ExpiresAt: 0,
			Id:        "",
			IssuedAt:  0,
			Issuer:    "",
			NotBefore: 0,
			Subject:   "",
		},
	}

	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return Perm{}, fmt.Errorf("%w while unmarshaling RefreshClaims: "+
			Sanitize(string(claimsJSON)), err)
	}

	if err := claims.Valid(); err != nil {
		return Perm{}, fmt.Errorf("%w in RefreshClaims: "+
			Sanitize(string(claimsJSON)), err)
	}

	return ck.permFromRefreshClaims(&claims)
}

// sign return the signature of the signingString.
// It allocates hmac.New() each time to avoid race condition.
func (ck *Checker) sign(signingString string) string {
	h := hmac.New(crypto.SHA256.New, ck.secretKey)
	_, _ = h.Write([]byte(signingString))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// --------------------------------------
// Read/write permissions to/from context

//nolint:gochecknoglobals // permKey is a Context key and need to be global
var permKey struct{}

// PermFromCtx gets the permission information from the request context.
func PermFromCtx(r *http.Request) Perm {
	perm, ok := r.Context().Value(permKey).(Perm)
	if !ok {
		log.Print("WRN JWT: no permission in context ", r.URL.Path)
	}
	return perm
}

// PutInCtx stores the permission info within the request context.
func (perm Perm) PutInCtx(r *http.Request) *http.Request {
	parent := r.Context()
	child := context.WithValue(parent, permKey, perm)
	return r.WithContext(child)
}
