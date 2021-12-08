// Copyright (C) 2020-2021 synw
//
// This code source is a simplified extract from "github.com/synw/quid/quidlib"
//
// MIT License

package jwtperm

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/karrick/tparse/v2"
)

// RefreshClaims is the user refresh token.
type RefreshClaims struct {
	Namespace string `json:"n,omitempty"`
	UserName  string `json:"u,omitempty"`
	jwt.RegisteredClaims
}

func standardRefreshClaims(namespace, user string, timeout time.Time) *RefreshClaims {
	return &RefreshClaims{
		namespace,
		user,
		jwt.RegisteredClaims{
			Issuer:    "",
			Subject:   "",
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(timeout),
			NotBefore: nil,
			IssuedAt:  nil,
			ID:        "",
		},
	}
}

func genRefreshToken(namespace string, refreshKey []byte, timeout, maxTTL, user string) (string, error) {
	isAuthorized, err := isTimeoutAuthorized(timeout, maxTTL)
	if err != nil {
		return "", err
	}

	if !isAuthorized {
		return "", nil
	}

	to, _ := tparse.ParseNow(time.RFC3339, "now+"+timeout)
	claims := standardRefreshClaims(namespace, user, to.UTC())
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	token, err := t.SignedString(refreshKey)
	if err != nil {
		return "", err
	}

	return token, nil
}

func isTimeoutAuthorized(timeout, maxTimeout string) (bool, error) {
	requested, err := tparse.ParseNow(time.RFC3339, "now+"+timeout)
	if err != nil {
		return false, err
	}

	max, err := tparse.ParseNow(time.RFC3339, "now+1s+"+maxTimeout)
	if err != nil {
		return false, err
	}

	return requested.UTC().Before(max.UTC()), nil
}
