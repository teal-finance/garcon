// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2022 Teal.Finance contributors
// Use of this source code is governed by a BSD-style
// license that can be found at:
// https://pkg.go.dev/std?tab=licenses
// SPDX-License-Identifier: BSD-3-Clause

// Package timex extends the standard package time.
package timex

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
)

const Inf = "inf"

const Infinite = math.MaxInt32 - 1

var InfTime = time.Time{}

// DT stringifies a time using the layout "2006-01-02.15:04:05".
// DT is used in debug logs when retrieving data from InfluxDB.
func DT(t time.Time) string {
	if t.IsZero() {
		return Inf
	}
	if d := t.Truncate(Day); t.Equal(d) {
		return d.Format("2006-01-02")
	}
	return t.Format("2006-01-02.15:04:05")
}

// YMD stringifies a time using the layout "2006-01-02".
// YMD is used in responses for /ui endpoints.
func YMD(t time.Time) string {
	if t.IsZero() {
		return Inf
	}
	return t.Format("2006-01-02")
}

// ISO stringifies a time using the layout RFC3339Nano (ISO 8601).
// Used in JSON response for /{topic} endpoints.
func ISO(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

// ISODefault uses the ISO-8601 (RFC-3339) format with the nanosecond precision,
// but defaults to defaultVal when t.IsZero().
func ISODefault(t time.Time, defaultVal string) string {
	if t.IsZero() {
		return defaultVal
	}
	return ISO(t)
}

// DStr stringifies the duration in number of days, seconds, microseconds and nanoseconds.
// DStr truncates the string depending on the precision.
func DStr(d time.Duration) string {
	s := ""

	if d <= -Day || d >= Day {
		days := d.Nanoseconds() / DayNs
		s = fmt.Sprint(days) + "d"

		if d < 0 {
			d = -d // sign "-" already marked
			days = -days
		}

		if d >= Week {
			return s // no sub-day precision when greater than a week
		}

		d -= time.Duration(days * DayNs)

		// no sub-second precision
		if d < Second {
			return s
		}
		d = d.Round(Second)
	} else if d <= -Minute || d >= Minute {
		d = d.Round(Second) // no sub-second precision when greater than a hour
	}

	return s + d.String()
}

// NsStr stringifies nanoseconds using the DStr pretty format.
func NsStr(nanoseconds int64) string {
	return DStr(time.Duration(nanoseconds))
}

// SecStr stringifies seconds using the standard duration format.
func SecStr(seconds int32) string {
	return DStr(time.Duration(seconds) * Second)
}

// SameDate returns true if t1 and t2 have same YYYY-MM-DD.
func SameDate(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return (y1 == y2) && (m1 == m2) && (d1 == d2)
}

// SameHour returns true if t1 and t2 have same HH.
func SameHour(t1, t2 time.Time) bool {
	return t1.Hour() == t2.Hour()
}

// SameMinuteSecond returns true if t1 and t2 have same minute and second.
func SameMinuteSecond(t1, t2 time.Time) bool {
	m1, s1 := t1.Minute(), t1.Second()
	m2, s2 := t2.Minute(), t2.Second()
	return (m1 == m2) && (s1 == s2)
}

// Durations in nanoseconds from 1 nanosecond to 1 year.
//nolint:revive // "Hour" (= time.Hour) is not a unit-specific suffix.
const (
	MinuteSec = 60           // MinuteSec = number of seconds in one minute.
	HourSec   = 3600         // HourSec = number of seconds in one hour.
	DaySec    = 24 * HourSec // DaySec = number of seconds in one day.
	WeekSec   = 7 * DaySec   // WeekSec = number of seconds in one week.
	MonthSec  = YearSec / 12 // MonthSec = number of seconds in one month.
	YearSec   = 31556952     // SecondsInOneYear is the average of the the number of seconds in one year.

	MicrosecondNs = 1000                 // Number of nanoseconds in 1 microsecond
	MillisecondNs = 1000 * MicrosecondNs // Number of nanoseconds in 1 millisecond
	SecondNs      = 1000 * MillisecondNs // Number of nanoseconds in 1 second
	MinuteNs      = SecondNs * MinuteSec // Number of nanoseconds in 1 minute
	HourNs        = SecondNs * HourSec   // Number of nanoseconds in 1 hour
	DayNs         = SecondNs * DaySec    // Number of nanoseconds in 1 day
	WeekNs        = SecondNs * WeekSec   // Number of nanoseconds in 1 week
	MonthNs       = SecondNs * MonthSec  // Number of nanoseconds in 1 month
	YearNs        = SecondNs * YearSec   // Number of nanoseconds in 1 year

	Nanosecond  = time.Nanosecond   // Nanosecond = one billionth of a second
	Microsecond = time.Microsecond  // Microsecond = 1000 nanoseconds
	Millisecond = time.Millisecond  // Millisecond = 1000 microseconds
	Second      = time.Second       // Second = 1000 milliseconds
	Minute      = time.Minute       // Minute = 60 seconds (no leap seconds)
	Hour        = time.Hour         // Hour = 60 minutes
	Day         = Second * DaySec   // Day = 24 hours (ignoring daylight savings effects)
	Week        = Second * WeekSec  // Week = 7 days
	Month       = Second * MonthSec // Month = one twelfth of a year (2629746 seconds, about 30.4 days)
	Year        = Second * YearSec  // Year = 31556952 seconds (average considering leap years, about 365.24 days)

	y2000ns = (2000 - 1970) * YearNs
	y2000s  = (2000 - 1970) * YearSec
	y2100ns = (2100 - 1970) * YearNs
	y2100s  = (2100 - 1970) * YearSec
)

var unitMap = map[string]int64{
	"ns": 1,
	"us": MicrosecondNs,
	"µs": MicrosecondNs, // U+00B5 = micro symbol
	"μs": MicrosecondNs, // U+03BC = Greek letter mu
	"ms": MillisecondNs,
	"s":  SecondNs,
	"m":  MinuteNs,
	"h":  HourNs,
	"d":  DayNs,
	"w":  WeekNs,
	"mo": MonthNs,
	"y":  YearNs,
}

var (
	// ErrMissingTimeAndUnit indicates time parser was expecting at least a time value followed by a time unit.
	ErrMissingTimeAndUnit = errors.New("time: expecting a value followed by a unit")

	// ErrExpectingInt indicates time parser was expecting one or more successive numbers.
	ErrExpectingInt = errors.New("time: expecting [0-9]*")

	// ErrNoDigits indicates time parser was expecting at least one number, but none.
	ErrNoDigits = errors.New("time: no digits (e.g. '.s' or '-.s')")

	// ErrNoUnit indicates time parser was expecting a time unit, but none.
	ErrNoUnit = errors.New("time: missing unit (ns, us, ms, s, m, h, d, w, mo, y)")

	// ErrUnknownUnit indicates time parser did not recognized the time unit.
	ErrUnknownUnit = errors.New("time: unknown unit (valid: ns, us, ms, s, m, h, d, w, mo, y)")

	// ErrOverflowTime indicates time parser is computing a time value larger that 63 bits.
	ErrOverflowTime = errors.New("time: time value overflow 63 bits")
)

const (
	DateOnly = "2006-01-02"
	TimeOnly = "15:04:05"
)

var YearZero = time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)

// ParseTime converts string to time.Time.
func ParseTime(s string) (_ time.Time, ok bool) {
	switch len(s) {
	case 0:
		return InfTime, true // returned time value has: IsZero() = true

	case len(TimeOnly):
		if t, err := time.Parse(TimeOnly, s); err == nil {
			hms := t.Sub(YearZero)
			today := time.Now().UTC().Truncate(Day)
			return today.Add(hms), true
		}

	case len(DateOnly):
		if t, err := time.Parse(DateOnly, s); err == nil {
			return t.UTC(), true
		}

	case len(DateOnly + "T" + TimeOnly):
		if t, err := time.Parse(DateOnly+"T"+TimeOnly, s); err == nil {
			return t.UTC(), true
		}
		if t, err := time.Parse(DateOnly+"."+TimeOnly, s); err == nil {
			return t.UTC(), true
		}
		if t, err := time.Parse(DateOnly+"_"+TimeOnly, s); err == nil {
			return t.UTC(), true
		}

	default:
		if len(s) <= len(DateOnly+"T"+TimeOnly) {
			break
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, true
		}
	}

	return parseTimeFallback(s)
}

// ParseTime converts string to time.Time.
func parseTimeFallback(s string) (_ time.Time, ok bool) {
	// Supported time units: "ns", "us" (or "µs"), "ms", "s", "m", "h", "d", "w", "mo" (month) and "y".
	if d, err := ParseDuration(s); err == nil {
		t := time.Now().UTC().Add(d)
		return t, true
	}

	// Consider numeric value as Unix time express either in seconds or nanoseconds since epoch
	if t, err := strconv.ParseInt(s, 10, 0); err == nil {
		if y2000ns < t && t < y2100ns {
			return time.Unix(0, t), true // t is the number of nanoseconds since epoch
		}
		if y2000s < t && t < y2100s {
			return time.Unix(t, 0), true // t is the number of seconds since epoch
		}
	}

	return time.Time{}, false // Failure
}

// ParseDuration parses a duration string.
// A duration string is a possibly signed sequence of
// decimal numbers, each with optional fraction and a unit suffix,
// such as "300ms", "-1.5h" or "2h45m".
// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h", "d", "w", "mo" (month) and "y".
func ParseDuration(s string) (time.Duration, error) {
	// Check problematic cases
	if len(s) <= 1 {
		if s == "0" {
			return time.Duration(0), nil
		}
		return time.Duration(0), fmt.Errorf("%w in '%v'", ErrMissingTimeAndUnit, s)
	}

	// Beginning of the time value
	i := 0
	if s[0] == '-' || s[0] == '+' {
		if s[1:] == "0" {
			return time.Duration(0), nil
		}
		i = 1
	}

	var duration int64

	for s[i:] != "" {
		shift, ns, err := consumeToken(s[i:])
		if err != nil {
			return time.Duration(0), fmt.Errorf("consumeToken: %w in '%v' at position=%v", err, s, i)
		}

		if duration > math.MaxInt64-ns {
			return time.Duration(0), fmt.Errorf("ParseDuration: %w in '%v' at position=%v", ErrOverflowTime, s, i)
		}
		duration += ns

		i += shift
	}

	// check if negative
	if s[0] == '-' {
		duration = -duration
	}

	return time.Duration(duration), nil
}

// consumeToken converts digits and its unit into nanoseconds.
func consumeToken(s string) (i int, ns int64, _ error) {
	i, value, fraPart, scale, err := consumeDigits(s)
	if err != nil {
		return 0, 0, err
	}

	shift, unitNs, err := consumeUnit(s[i:])
	if err != nil {
		return 0, 0, err
	}

	ns, err = computeNanoseconds(unitNs, value, fraPart, scale)
	if err != nil {
		return 0, 0, err
	}

	return i + shift, ns, nil
}

// consumeDigits parses leading digits in string and returns position and integer/fraPart parts.
func consumeDigits(s string) (i int, intPart, fraPart int64, scale float64, _ error) {
	// Consume [0-9]*
	intPart, intLen, err := integralPart(s[i:])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	i += intLen

	// Consume (\.[0-9]*)?
	var fraLen int // number of digits in the fractional part
	if s[i:] != "" && s[i] == '.' {
		i++
		fraPart, fraLen, scale = fractionalPart(s[i:])
		i += fraLen
	}

	// No digits?
	if intLen == 0 && fraLen == 0 {
		return 0, 0, 0, 0, ErrNoDigits
	}

	return i, intPart, fraPart, scale, nil
}

// consumeUnit parses the time unit in the string and returns the number of nanoseconds corresponding .
func consumeUnit(s string) (i int, nanoseconds int64, _ error) {
	// Consume unit
	for ; i < len(s); i++ {
		c := s[i]
		if c == '.' || ('0' <= c && c <= '9') {
			break
		}
	}
	if i == 0 {
		return 0, 0, ErrNoUnit
	}

	// Convert unit string into nanoseconds
	unit := s[:i]
	nanoseconds, ok := unitMap[unit]
	if !ok {
		return i, 0, ErrUnknownUnit
	}

	return i, nanoseconds, nil
}

func computeNanoseconds(unitNs, value, fraPart int64, scale float64) (nanoseconds int64, _ error) {
	// Convert the integer part in nanoseconds
	if value > (math.MaxInt64)/unitNs {
		return 0, ErrOverflowTime
	}
	nanoseconds = value * unitNs

	// No fractional part => stop here
	if scale == 0 {
		return nanoseconds, nil
	}

	// Convert the fractional part using nanosecond precision if possible
	if scale < math.MaxInt64 {
		scaleInt := int64(scale)
		modulo := unitNs % scaleInt
		if modulo == 0 {
			multiplier := unitNs / scaleInt
			if fraPart < math.MaxInt64/multiplier {
				value = fraPart * multiplier
				goto ADD_FRACTIONAL_PART
			}
		}
	}

	// cannot be nanosecond accurate with "y" time unit
	value = int64(float64(fraPart) / scale * float64(unitNs))

ADD_FRACTIONAL_PART:

	if nanoseconds > math.MaxInt64-value {
		return 0, ErrOverflowTime
	}

	return nanoseconds + value, nil
}

// integralPart consumes the leading [0-9]* from s.
func integralPart(s string) (v int64, i int, _ error) {
	for i = 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if v > (math.MaxInt64)/10 {
			return v, i, ErrOverflowTime
		}
		v = v*10 + int64(c) - '0'
		if v < 0 {
			return v, i, ErrOverflowTime
		}
	}
	return v, i, nil
}

// fractionalPart consumes the leading [0-9]* from s.
// It is used only for fractions, so does not return an error on overflow,
// it just stops accumulating precision.
func fractionalPart(s string) (f int64, i int, scale float64) {
	scale = 1
	overflow := false

	for i = 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if overflow {
			continue
		}
		if f > (math.MaxInt64)/10 {
			// It's possible for overflow to give a positive number, so take care.
			overflow = true
			continue
		}
		y := f*10 + int64(c) - '0'
		if y < 0 {
			overflow = true
			continue
		}
		f = y
		scale *= 10
	}

	return f, i, scale
}

// Relative computes a relative time.
func Relative(t time.Time, days int) time.Time {
	if days == Infinite {
		return InfTime
	}

	if days == 0 {
		return t
	}

	return t.Add(time.Duration(days) * Day)
}
