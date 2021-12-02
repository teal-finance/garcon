// Copyright (C) 2020-2021 TealTicks contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package jwt delivers and checks the JWT permissions
package jwtperm

import (
	"context"
	"crypto"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/synw/quid/quidlib/tokens"
	"github.com/teal-finance/garcon/reserr"
)

const (
	bearerPrefix = "Bearer "
	cookieName   = "jwt"

	invalidCookie = "invalid cookie"
	noClaimsInJWT = "no Access nor Refresh token in JWT"
	expiredRToken = "Refresh token has expired (or invalid)"

	defaultPlanName = "DefaultPlan"

	defaultPermValue = 3600     // one hour
	oneYearInSeconds = 31556952 // average number of seconds including leap years
	oneYearInNS      = oneYearInSeconds * 1_000_000_000
)

var (
	ErrUnauthorized = errors.New("JWT not authorized")
	ErrNoTokenFound = errors.New("no JWT found")
)

type Perm struct {
	Value int
}

type Checker struct {
	resErr      reserr.ResErr
	b64encoding *base64.Encoding
	secretKey   []byte
	perms       []Perm
	plans       []string
	cookies     []http.Cookie
	devOrigins  []string
}

func New(addresses []string, resErr reserr.ResErr, secretKey string, permissions ...interface{}) *Checker {
	secure, dns, devOrigins := extractDomains(addresses)

	n := len(permissions) / 2
	if n == 0 {
		n = 1
	}

	plans := make([]string, n)
	pVals := make([]int, n)

	plans[0] = defaultPlanName
	pVals[0] = defaultPermValue

	for i, p := range permissions {
		var ok bool
		if i%2 == 0 {
			plans[i/2], ok = p.(string)
		} else {
			pVals[i/2], ok = p.(int)
		}

		if !ok {
			log.Panic("Wrong type for the parametric arguments in jwtperm.New(), " +
				"must alternate string and int: plan1, perm1, plan2, perm2...")
		}
	}

	perms := make([]Perm, n)
	cookies := make([]http.Cookie, n)

	for i, v := range pVals {
		perms[i] = Perm{Value: v}
		cookies[i] = createCookie(plans[i], dns, secretKey, secure)
	}

	return &Checker{
		resErr:      resErr,
		b64encoding: base64.RawURLEncoding,
		secretKey:   []byte(secretKey),
		plans:       plans,
		perms:       perms,
		cookies:     cookies,
		devOrigins:  devOrigins,
	}
}

func extractDomains(addresses []string) (secure bool, dns string, devOrigins []string) {
	if len(addresses) == 0 {
		log.Panic("No addresses => Cannot set Cookie domain")
	}

	if strings.HasPrefix(addresses[0], "http://") {
		secure = false
		dns = addresses[0][len("http://"):]
	} else if strings.HasPrefix(addresses[0], "https://") {
		secure = true
		dns = addresses[0][len("https://"):]
	}

	dns = strings.SplitN(dns, ":", 2)[0]

	if len(addresses) == 1 {
		log.Print("JWT required for single origin: ", addresses)
	} else {
		for i, a := range addresses {
			if strings.HasPrefix(a, "https://") {
				log.Print("JWT required for HTTPS origin: ", a)

				continue
			}

			devOrigins = addresses[i:]
			log.Print("JWT not required for dev. origins: ", devOrigins)

			break
		}
	}

	return secure, dns, devOrigins
}

func createCookie(plan, dns, secretKey string, secure bool) http.Cookie {
	_, jwt, err := tokens.GenRefreshToken(plan, secretKey, "1y", "", "1y")
	if err != nil || jwt == "" {
		log.Panic("Cannot create JWT: ", err)
	}

	log.Print("Create cookie plan=", plan, " domain=", dns, " secure=", secure, " "+cookieName+"=", jwt)

	return http.Cookie{
		Name:       cookieName,
		Value:      jwt,
		Path:       "/",
		Domain:     dns,
		Expires:    time.Time{},
		RawExpires: "",
		MaxAge:     oneYearInSeconds,
		Secure:     secure,
		HttpOnly:   true,
		SameSite:   http.SameSiteStrictMode,
		Raw:        "",
		Unparsed:   []string{},
	}
}

// SetCookie sets a cookie (if not present/valid) within the HTTP response header.
func (ck *Checker) SetCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ck.hasValidCookie(r) {
			log.Print("JWT cookie already present and valid")
		} else {
			ck.cookies[0].Expires = time.Now().Add(oneYearInNS)
			log.Print("JWT: Set cookie ", ck.cookies[0])
			http.SetCookie(w, &ck.cookies[0])
		}

		next.ServeHTTP(w, ck.perms[0].storeInContext(r))
	})
}

// ChkCookie accepts the HTTP request only it contains a valid Cookie.
func (ck *Checker) ChkCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, errMsg := ck.permFromCookie(r)
		if errMsg != "" {
			if ck.isDevOrigin(r) {
				perm = ck.perms[0]
			} else {
				ck.resErr.Write(w, r, http.StatusUnauthorized, errMsg)

				return
			}
		}

		next.ServeHTTP(w, perm.storeInContext(r))
	})
}

// Check accepts the HTTP request only if a valid JWT is in the Cookie or in the first "Autorisation" header.
func (ck *Checker) Check(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, errMsg := ck.permFromBearerOrCookie(r)
		if errMsg != "" {
			if ck.isDevOrigin(r) {
				perm = ck.perms[0]
			} else {
				ck.resErr.Write(w, r, http.StatusUnauthorized, errMsg)

				return
			}
		}

		next.ServeHTTP(w, perm.storeInContext(r))
	})
}

func (ck *Checker) hasValidCookie(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}

	for _, c := range ck.cookies {
		if c.Value == cookie.Value {
			return true
		}
	}

	_, errMsg := ck.decomposeJWT(cookie.Value)

	return (errMsg == "")
}

func (ck *Checker) isDevOrigin(r *http.Request) bool {
	if len(ck.devOrigins) == 0 {
		return false
	}

	if len(ck.devOrigins) > 0 {
		origin := r.Header.Get("Origin")

		for _, prefix := range ck.devOrigins {
			if strings.HasPrefix(origin, prefix) {
				log.Printf("No JWT but origin=%v is a valid dev origin", origin)

				return true
			}
		}

		log.Print("No JWT and origin=", origin, " has not prefixes ", ck.devOrigins)
	}

	return false
}

func (ck *Checker) permFromBearerOrCookie(r *http.Request) (perm Perm, errC string) {
	jwt, err := ck.jwtFromBearer(r)
	if err != "" {
		jwt, errC = ck.jwtFromCookie(r)
		if errC != "" {
			err += " or " + errC
			log.Print("No JWT from Bearer or Cookie: ", err)

			return perm, err
		}
	}

	return ck.permFromJWT(jwt)
}

func (ck *Checker) permFromCookie(r *http.Request) (perm Perm, errMsg string) {
	jwt, errMsg := ck.jwtFromCookie(r)
	if errMsg != "" {
		log.Print("No JWT from cookie: ", errMsg)

		return perm, errMsg
	}

	return ck.permFromJWT(jwt)
}

func (ck *Checker) jwtFromBearer(r *http.Request) (jwt, errMsg string) {
	auth := r.Header.Get("Authorization")

	n := len(bearerPrefix)
	if len(auth) > n && auth[:n] == bearerPrefix {
		log.Print("Authorization header has JWT: ", auth[n:])

		return auth[n:], "" // Success
	}

	if auth == "" {
		log.Print("Authorization header is missing, no JWT")

		return "", `provide your JWT within the 'Authorization Bearer' HTTP header`
	}

	log.Printf("Authorization header %q does not contain %q", auth, bearerPrefix)

	return "", invalidCookie
}

func (ck *Checker) jwtFromCookie(r *http.Request) (jwt, errMsg string) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		log.Print("Cookie name="+cookieName+" is missing: ", err)

		if cookies := r.Cookies(); len(cookies) > 0 {
			log.Print("Other cookies in HTTP request: ", r.Cookies())
		}

		return "", "visit the official " + ck.cookies[0].Domain + " web site to get a valid Cookie"
	}

	log.Print("Cookie has JWT: ", c.Value)

	return c.Value, "" // Success
}

func (ck *Checker) permFromJWT(jwt string) (perm Perm, errMsg string) {
	for i, c := range ck.cookies {
		if c.Value == jwt {
			return ck.perms[i], "" // Success
		}
	}

	parts, errMsg := ck.partsFromJWT(jwt)
	if errMsg != "" {
		return perm, errMsg
	}

	perm, errMsg = ck.permFromRefreshBytes(parts)
	if errMsg != "" {
		log.Print("WRN JWT: ", errMsg)

		return perm, expiredRToken
	}

	log.Print("JWT Permission: ", perm)

	return perm, "" // Success
}

func (ck *Checker) permFromRefreshClaims(claims *tokens.StandardRefreshClaims) Perm {
	for i, p := range ck.plans {
		if p == claims.Namespace {
			log.Print("JWT has the ", p, " Namespace")

			return ck.perms[i]
		}
	}

	log.Print("WRN Set default JWT because RefreshClaims has not been identified: ", claims)

	return ck.perms[0]
}

func (ck *Checker) decomposeJWT(jwt string) (parts []string, errMsg string) {
	parts = strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, "JWT is not composed by three segments (separated by dots)"
	}

	if errMsg = ck.verifySignature(parts); errMsg != "" {
		return nil, errMsg
	}

	return parts, "" // Success
}

func (ck *Checker) partsFromJWT(jwt string) (claimsJSON []byte, errMsg string) {
	parts, errMsg := ck.decomposeJWT(jwt)
	if errMsg != "" {
		return nil, errMsg
	}

	claimsJSON, err := ck.b64encoding.DecodeString(parts[1])
	if err != nil {
		log.Print("WRN JWT Base64 decoding: ", err)

		return nil, "The token claims (second part of the JWT) is not base64-valid"
	}

	return claimsJSON, "" // Success
}

// verifySignature of HS256 tokens.
func (ck *Checker) verifySignature(parts []string) (errMsg string) {
	signingString := strings.Join(parts[0:2], ".")
	signedString := ck.sign(signingString)

	if signature := parts[2]; signature != signedString {
		log.Print("WRN JWT signature in 3rd part : ", signature)
		log.Print("WRN JWT signed first two parts: ", signedString)

		return "JWT signature mismatch"
	}

	return "" // Success
}

func (ck *Checker) permFromRefreshBytes(claimsJSON []byte) (perm Perm, errMsg string) {
	claims := &tokens.StandardRefreshClaims{
		Namespace: "",
		UserName:  "",
		StandardClaims: jwt.StandardClaims{
			Audience:  "",
			ExpiresAt: 0,
			Id:        invalidCookie,
			IssuedAt:  0,
			Issuer:    "",
			NotBefore: 0,
			Subject:   "",
		},
	}

	if err := json.Unmarshal(claimsJSON, claims); err != nil {
		log.Print("WRN JWT ", err, " while unmarshaling RefreshClaims: ", string(claimsJSON))

		return perm, noClaimsInJWT
	}

	if err := claims.Valid(); err != nil {
		return perm, err.Error()
	}

	log.Print("JWT Claims: ", *claims)

	perm = ck.permFromRefreshClaims(claims)

	return perm, "" // Success
}

// sign allocates the hasher each time to avoid race condition.
func (ck *Checker) sign(signingString string) (signature string) {
	hasher := hmac.New(crypto.SHA256.New, ck.secretKey)
	_, _ = hasher.Write([]byte(signingString))

	return ck.b64encoding.EncodeToString(hasher.Sum(nil))
}

// --------------------------------------
// Read/write permissions to/from context

// From gets the permission information from the request context.
func From(r *http.Request) Perm {
	perm, ok := r.Context().Value(permKey).(Perm)
	if !ok {
		log.Print("WRN JWT No permissions within the context ", r.URL.Path)
	}

	return perm
}

var permKey struct{}

// storeInContext stores the permission info within the request context.
func (perm Perm) storeInContext(r *http.Request) *http.Request {
	parentCtx := r.Context()
	childCtx := context.WithValue(parentCtx, permKey, perm)

	return r.WithContext(childCtx)
}
