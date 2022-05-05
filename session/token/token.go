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
)

const (
	// The expiry is stored in 3 bytes, with a 30 seconds precision, starting from 2022.
	expiryStartYear    = 2022
	precisionInSeconds = 30
	expiryMin          = expiryStartYear * secondsPerDay / precisionInSeconds
	expiryMax          = expiryMin + 2 ^ 24

	internalToUnix = (expiryStartYear - 1970) * secondsPerDay

	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour      // = 86400
	secondsPerYear   = 365.2425 * secondsPerDay // = 31556952 = average number of seconds including leap years

)

type Token struct {
	Expiry uint32 // 30 seconds precision (2²⁴ = 16 years)
	IP     net.IP
	Values [][]byte
}

func (t *Token) SetExpiry(unix int64) error {
	sinceInternal := unix - internalToUnix
	e := sinceInternal / precisionInSeconds

	if e < expiryMin {
		return fmt.Errorf("unix time (%d sec.) cannot be stored in the Token because too low", unix)
	}
	if e > expiryMax {
		return fmt.Errorf("unix time (%d sec.) cannot be stored in the Token because too high", unix)
	}

	t.Expiry = uint32(e)
	return nil
}

func (t *Token) SetExpiryDelay(seconds int64) error {
	now := time.Now().Unix()
	unix := now + seconds
	return t.SetExpiry(unix)
}

// ExpiryUnix returns the Expiry in Unix time format, seconds since epoch (1970 UTC).
func (t *Token) ExpiryUnix() int64 {
	seconds := int64(t.Expiry) * precisionInSeconds
	unix := seconds + internalToUnix
	return unix
}

func (t *Token) CompareExpiry() int {
	now := time.Now().Unix()
	e := t.ExpiryUnix()
	if e < now {
		return -1
	}
	if e > now+secondsPerYear {
		return 1
	}
	return 0
}

func (t *Token) IsExpiryValid() bool {
	return (t.CompareExpiry() == 0)
}

func (t *Token) ShortenIP() {
	if v4 := t.IP.To4(); v4 != nil {
		t.IP = v4
	}
}
