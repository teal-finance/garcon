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

package bits

import (
	"fmt"
	"time"
)

const (
	// The expiry is stored in 3 bytes, with a 30 seconds precision, starting from 2022.
	ExpiryStartYear = 2022
	ExpiryMaxYear   = ExpiryStartYear + rangeInYears

	ExpirySize = 2 // 16 bits = 22 years with 30-second precision
	expiryBits = ExpirySize * 8
	expiryMax  = 1 << expiryBits

	PrecisionInSeconds = 30
	rangeInSeconds     = expiryMax * PrecisionInSeconds
	rangeInYears       = rangeInSeconds / secondsPerYear

	internalToUnix = (ExpiryStartYear - 1970) * secondsPerYear
	unixToInternal = -internalToUnix

	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour      // = 86400
	secondsPerYear   = 365.2425 * secondsPerDay // = 31556952 = average including leap years
)

// unixToInternalExpiry converts Unix time to the internal 3-byte coding.
func unixToInternalExpiry(unix int64) (uint32, error) {
	secondsSinceInternalStarYear := unix + unixToInternal
	internalExpiry := secondsSinceInternalStarYear / PrecisionInSeconds

	if internalExpiry < 0 {
		return 0, fmt.Errorf("unix time too low (%d s) %s < %d", unix, time.Unix(unix, 0), ExpiryStartYear)
	}
	if internalExpiry >= expiryMax {
		return 0, fmt.Errorf("unix time too high (%d s) %s > %d", unix, time.Unix(unix, 0), ExpiryMaxYear)
	}

	return uint32(internalExpiry), nil
}

// internalExpiryToUnix converts the internal expiry to Unix time format: seconds since epoch (1970 UTC).
func internalExpiryToUnix(expiry uint32) int64 {
	seconds := int64(expiry) * PrecisionInSeconds
	unix := seconds + internalToUnix
	return unix
}
