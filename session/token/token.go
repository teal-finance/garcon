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

package token

import (
	"fmt"
	"net"
	"time"

	"github.com/teal-finance/garcon/session/incorruptible/bits"
)

const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour      // = 86400
	secondsPerYear   = 365.2425 * secondsPerDay // = 31556952 = average including leap years
)

type Token struct {
	Expiry int64 // Unix time UTC (seconds since 1970)
	IP     net.IP
	Values [][]byte
}

func (t *Token) SetExpiry(secondsSinceNow int64) {
	now := time.Now().Unix()
	unix := now + secondsSinceNow
	t.Expiry = unix
}

func (t *Token) CompareExpiry() int {
	now := time.Now().Unix()
	if t.Expiry < now {
		return -1
	}
	if t.Expiry > now+secondsPerYear {
		return 1
	}
	return 0
}

func (t *Token) IsExpiryValid() bool {
	c := t.CompareExpiry()
	return (c == 0)
}

func (t *Token) ShortenIP() {
	if v4 := t.IP.To4(); v4 != nil {
		t.IP = v4
	}
}

func (t *Token) SetUint64(i int, v uint64) error {
	if err := t.check(i); err != nil {
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

	t.set(i, b)
	return nil
}

func (t *Token) Uint64(i int) (uint64, error) {
	if (i < 0) || (i >= len(t.Values)) {
		return 0, fmt.Errorf("i=%d out of range (%d values)", i, len(t.Values))
	}

	b := t.Values[i]

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

func (t *Token) SetBool(i int, value bool) error {
	if err := t.check(i); err != nil {
		return err
	}

	var b []byte // false --> length=0
	if value {
		b = []byte{0} // true --> length=1
	}

	t.set(i, b)
	return nil
}

func (t *Token) Bool(i int) (bool, error) {
	if (i < 0) || (i >= len(t.Values)) {
		return false, fmt.Errorf("i=%d out of range (%d values)", i, len(t.Values))
	}

	b := t.Values[i]

	switch len(b) {
	default:
		return false, fmt.Errorf("too much bytes (length=%d) for a boolean", len(b))
	case 1:
		return true, nil
	case 0:
		return false, nil
	}
}

func (t *Token) SetString(i int, s string) error {
	if err := t.check(i); err != nil {
		return err
	}

	t.set(i, []byte(s))
	return nil
}

func (t *Token) String(i int) (string, error) {
	if (i < 0) || (i >= len(t.Values)) {
		return "", fmt.Errorf("i=%d out of range (%d values)", i, len(t.Values))
	}
	return string(t.Values[i]), nil
}

func (t *Token) check(i int) error {
	if i < 0 {
		return fmt.Errorf("negative i=%d", i)
	}
	if i > bits.MaxValues {
		return fmt.Errorf("cannot store more than %d values (i=%d)", bits.MaxValues, i)
	}
	return nil
}

func (t *Token) set(i int, b []byte) {
	if i == len(t.Values) {
		t.Values = append(t.Values, b)
		return
	}

	if i >= cap(t.Values) {
		values := make([][]byte, i+1)
		copy(values, t.Values)
		t.Values = values
	}

	if i >= len(t.Values) {
		t.Values = t.Values[:i+1]
	}

	t.Values[i] = b
}
