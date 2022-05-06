// #region <editor-fold desc="Preamble">
// Copyright (c) 2022 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon, an API and website server.
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 or any later version, at the licensee’s option.
// SPDX-License-Identifier: LGPL-3.0-or-later
//
// Teal.Finance/Garcon is distributed WITHOUT ANY WARRANTY.
// For more details, see the LICENSE file (alongside the source files)
// or online at <https://www.gnu.org/licenses/lgpl-3.0.html>
// #endregion </editor-fold>

// Package token represents the decoded form of a "session" token.
package dtoken

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/teal-finance/garcon/session/incorruptible/bits"
)

const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour      // = 86400
	secondsPerYear   = 365.2425 * secondsPerDay // = 31556952 = average including leap years
)

type DToken struct {
	Expiry int64 // Unix time UTC (seconds since 1970)
	IP     net.IP
	Values [][]byte
}

func (dt *DToken) Valid(r *http.Request) error {
	if dt.Expiry != 0 {
		if !dt.ValidExpiry() {
			return fmt.Errorf("expired or malformed date %v", dt.Expiry)
		}
	}

	if len(dt.IP) > 0 {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return fmt.Errorf("checking token but %w", err)
		}
		if !dt.IP.Equal(net.ParseIP(ip)) {
			return fmt.Errorf("token says IP=%v but got %v", dt.IP, ip)
		}
	}

	return nil
}

func (dt *DToken) SetExpiry(secondsSinceNow int64) {
	now := time.Now().Unix()
	unix := now + secondsSinceNow
	dt.Expiry = unix
}

func (dt *DToken) ExpiryTime() time.Time {
	return time.Unix(dt.Expiry, 0)
}

func (dt DToken) CompareExpiry() int {
	now := time.Now().Unix()
	if dt.Expiry < now {
		return -1
	}
	if dt.Expiry > now+secondsPerYear {
		return 1
	}
	return 0
}

func (dt DToken) ValidExpiry() bool {
	c := dt.CompareExpiry()
	return (c == 0)
}

func (dt *DToken) ShortenIP() {
	if dt.IP == nil {
		return
	}
	if v4 := dt.IP.To4(); v4 != nil {
		dt.IP = v4
	}
}

func (dt *DToken) SetUint64(i int, v uint64) error {
	if err := dt.check(i); err != nil {
		return err
	}

	var b []byte
	switch {
	case v == 0:
		b = nil
	case v < (1 << 8):
		b = []byte{byte(v)}
	case v < (1 << 16):
		b = []byte{byte(v), byte(v >> 8)}
	case v < (1 << 24):
		b = []byte{byte(v), byte(v >> 8), byte(v >> 16)}
	case v < (1 << 32):
		b = []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
	case v < (1 << 40):
		b = []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24), byte(v >> 32)}
	case v < (1 << 48):
		b = []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24), byte(v >> 32), byte(v >> 40)}
	case v < (1 << 56):
		b = []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24), byte(v >> 32), byte(v >> 40), byte(v >> 48)}
	default:
		b = []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24), byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56)}
	}

	dt.set(i, b)
	return nil
}

func (dt DToken) Uint64(i int) (uint64, error) {
	if (i < 0) || (i >= len(dt.Values)) {
		return 0, fmt.Errorf("i=%d out of range (%d values)", i, len(dt.Values))
	}

	b := dt.Values[i]

	var r uint64
	switch len(b) {
	default:
		return 0, fmt.Errorf("too much bytes (length=%d) for an integer", len(b))
	case 8:
		r |= uint64(b[7]) << 56
		fallthrough
	case 7:
		r |= uint64(b[6]) << 48
		fallthrough
	case 6:
		r |= uint64(b[5]) << 40
		fallthrough
	case 5:
		r |= uint64(b[4]) << 32
		fallthrough
	case 4:
		r |= uint64(b[3]) << 24
		fallthrough
	case 3:
		r |= uint64(b[2]) << 16
		fallthrough
	case 2:
		r |= uint64(b[1]) << 8
		fallthrough
	case 1:
		r |= uint64(b[0])
	case 0:
		r = 0
	}

	return r, nil
}

func (dt *DToken) SetBool(i int, value bool) error {
	if err := dt.check(i); err != nil {
		return err
	}

	var b []byte // false --> length=0
	if value {
		b = []byte{0} // true --> length=1
	}

	dt.set(i, b)
	return nil
}

func (dt DToken) Bool(i int) (bool, error) {
	if (i < 0) || (i >= len(dt.Values)) {
		return false, fmt.Errorf("i=%d out of range (%d values)", i, len(dt.Values))
	}

	b := dt.Values[i]

	switch len(b) {
	default:
		return false, fmt.Errorf("too much bytes (length=%d) for a boolean", len(b))
	case 1:
		return true, nil
	case 0:
		return false, nil
	}
}

func (dt *DToken) SetString(i int, s string) error {
	if err := dt.check(i); err != nil {
		return err
	}

	dt.set(i, []byte(s))
	return nil
}

func (dt DToken) String(i int) (string, error) {
	if (i < 0) || (i >= len(dt.Values)) {
		return "", fmt.Errorf("i=%d out of range (%d values)", i, len(dt.Values))
	}
	return string(dt.Values[i]), nil
}

func (dt DToken) check(i int) error {
	if i < 0 {
		return fmt.Errorf("negative i=%d", i)
	}
	if i > bits.MaxValues {
		return fmt.Errorf("cannot store more than %d values (i=%d)", bits.MaxValues, i)
	}
	return nil
}

func (dt *DToken) set(i int, b []byte) {
	if i == len(dt.Values) {
		dt.Values = append(dt.Values, b)
		return
	}

	if i >= cap(dt.Values) {
		values := make([][]byte, i+1)
		copy(values, dt.Values)
		dt.Values = values
	}

	if i >= len(dt.Values) {
		dt.Values = dt.Values[:i+1]
	}

	dt.Values[i] = b
}

// --------------------------------------
// Manage token in request context.
var tokenKey struct{}

// PutInCtx stores the decoded token in the request context.
func (dt DToken) PutInCtx(r *http.Request) *http.Request {
	parent := r.Context()
	child := context.WithValue(parent, tokenKey, dt)
	return r.WithContext(child)
}

// FromCtx gets the decoded token from the request context.
func FromCtx(r *http.Request) (DToken, error) {
	dt, ok := r.Context().Value(tokenKey).(DToken)
	if !ok {
		return dt, fmt.Errorf("no token in context %s", r.URL.Path)
	}
	return dt, nil
}
