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
	"net"
	"time"
)

const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour      // = 86400
	secondsPerYear   = 365.2425 * secondsPerDay // = 31556952 = average number of seconds including leap years
)

type Token struct {
	Expiry int64 // Unix time UTC (seconds since 1970)
	IP     net.IP
	Values [][]byte
}

func (t *Token) SetExpiryDelay(seconds int64) {
	now := time.Now().Unix()
	unix := now + seconds
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
