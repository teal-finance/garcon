// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2022 Teal.Finance contributors
// Use of this source code is governed by a BSD-style
// license that can be found at:
// https://pkg.go.dev/std?tab=licenses
// SPDX-License-Identifier: BSD-3-Clause

package timex

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	rand "github.com/zhangyunhao116/fastrand"
)

var parseDurationTests = []struct {
	in   string
	want time.Duration
}{
	// simple
	{"0", 0},
	{"5s", 5 * time.Second},
	{"30s", 30 * time.Second},
	{"1478s", 1478 * time.Second},
	// sign
	{"-5s", -5 * time.Second},
	{"+5s", 5 * time.Second},
	{"-0", 0},
	{"+0", 0},
	// decimal
	{"5.0s", 5 * time.Second},
	{"5.6s", 5*time.Second + 600*time.Millisecond},
	{"5.s", 5 * time.Second},
	{".5s", 500 * time.Millisecond},
	{"1.0s", 1 * time.Second},
	{"1.00s", 1 * time.Second},
	{"1.004s", 1*time.Second + 4*time.Millisecond},
	{"1.0040s", 1*time.Second + 4*time.Millisecond},
	{"100.00100s", 100*time.Second + 1*time.Millisecond},
	// different units
	{"10ns", 10 * time.Nanosecond},
	{"11us", 11 * time.Microsecond},
	{"12µs", 12 * time.Microsecond}, // U+00B5
	{"12μs", 12 * time.Microsecond}, // U+03BC
	{"13ms", 13 * time.Millisecond},
	{"14s", 14 * time.Second},
	{"15m", 15 * time.Minute},
	{"16h", 16 * time.Hour},
	// composite durations
	{"3h30m", 3*time.Hour + 30*time.Minute},
	{"10.5s4m", 4*time.Minute + 10*time.Second + 500*time.Millisecond},
	{"-2m3.4s", -(2*time.Minute + 3*time.Second + 400*time.Millisecond)},
	{"1h2m3s4ms5us6ns", 1*time.Hour + 2*time.Minute + 3*time.Second + 4*time.Millisecond + 5*time.Microsecond + 6*time.Nanosecond},
	{"39h9m14.425s", 39*time.Hour + 9*time.Minute + 14*time.Second + 425*time.Millisecond},
	// large value
	{"52763797000ns", 52763797000 * time.Nanosecond},
	// more than 9 digits after decimal point, see https://golang.org/issue/6617
	{"0.3333333333333333333h", 20 * time.Minute},
	// 9007199254740993 = 1<<53+1 cannot be stored precisely in a float64
	{"9007199254740993ns", (1<<53 + 1) * time.Nanosecond},
	// largest duration that can be represented by int64 in nanoseconds
	{"9223372036854775807ns", (1<<63 - 1) * time.Nanosecond},
	{"9223372036854775.807us", (1<<63 - 1) * time.Nanosecond},
	{"9223372036s854ms775us807ns", (1<<63 - 1) * time.Nanosecond},
	// large negative value
	{"-9223372036854775807ns", -1<<63 + 1*time.Nanosecond},
	// huge string; issue 15011.
	{"0.100000000000000000000h", 6 * time.Minute},
	// This value cases the first overflow check in fractionalPart.
	{"0.830103483285477580700h", 49*time.Minute + 48*time.Second + 372539827*time.Nanosecond},
	{"+1d", 24 * time.Hour},
	{"1w1d", 8 * 24 * time.Hour},
	{"1mo", 2629746 * time.Second},
	{"1y", 31556952 * time.Second},
	{"1y2y3y", 6 * 31556952 * time.Second},
}

func TestParseDuration(t *testing.T) {
	t.Parallel()

	for _, c := range parseDurationTests {
		c := c

		t.Run(c.in, func(t *testing.T) {
			t.Parallel()

			d, err := ParseDuration(c.in)
			if err != nil || d != c.want {
				t.Errorf("ParseDuration(%v) = %v, %v, want %v, nil", c.in, DStr(d), err, c.want)
			}
		})
	}
}

var parseDurationErrorTests = []struct {
	in   string
	want string
}{
	// invalid
	{"", "''"},
	{"3", "'3'"},
	{"3n", "'3n'"},
	{"456u", "'456u'"},
	{"-", "'-'"},
	{"s", "'s'"},
	{".", "'.'"},
	{"-.", "'-.'"},
	{".s", "'.s'"},
	{"+.s", "'+.s'"},
	// overflow
	{"9223372036854775810ns", "'9223372036854775810ns'"},
	{"9223372036854775808ns", "'9223372036854775808ns'"},
	// largest negative value of type int64 in nanoseconds should fail
	// see https://go-review.googlesource.com/#/c/2461/
	{"-9223372036854775808ns", "'-9223372036854775808ns'"},
	{"9223372036854776us", "'9223372036854776us'"},
	{"3000000h", "'3000000h'"},
	{"9223372036854775.808us", "'9223372036854775.808us'"},
	{"9223372036854ms775us808ns", "'9223372036854ms775us808ns'"},
	{"not-a-date", "'not-a-date'"},
}

func TestParseDurationErrors(t *testing.T) {
	t.Parallel()

	for _, c := range parseDurationErrorTests {
		c := c

		t.Run(c.in, func(t *testing.T) {
			t.Parallel()

			_, err := ParseDuration(c.in)
			if err == nil {
				t.Errorf("ParseDuration(%q) err: got=nil want=non-nil", c.in)
			} else if !strings.Contains(err.Error(), c.want) {
				t.Errorf("ParseDuration(%q)", c.in)
				t.Errorf("got err: %q", err)
				t.Errorf("want err with: %q", c.want)
			}
		})
	}
}

func TestParseDurationRoundTrip(t *testing.T) {
	t.Parallel()

	const maxLoops = 100
	for i := 0; i < maxLoops; i++ {
		// Resolutions finer than milliseconds will result in imprecise round-trips.
		ns := rand.Intn(WeekNs) % MillisecondNs
		name := strconv.Itoa(ns) + "ns"

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// DStr() rounds to days when duration is more than or equal to one week
			// DStr() rounds to seconds when duration is more than or equal to one minute
			d0 := time.Duration(ns)
			s := DStr(d0)
			d1, err := ParseDuration(s)

			if err != nil || d0 != d1 {
				t.Errorf("round-trip failed: %d => %v => %d, %v", d0, s, d1, err)
			}
		})
	}
}

func TestStringifyTime(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{"zero", time.Time{}, "default-value"},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := ISODefault(c.in, "default-value"); got != c.want {
				t.Errorf("StringifyTime() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		in     string
		want   time.Time
		wantOK bool
	}{
		{"zero", "", time.Time{}, true},
		{"err", "not-a-date", time.Time{}, false},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ParseTime(c.in)

			if ok != c.wantOK {
				t.Errorf("ParseTime() ok = %v, wantOK = %v", ok, c.wantOK)
				return
			}

			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ParseTime() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestConsumeDigits(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		in           string
		wantI        int
		wantValue    int64
		wantFraction int64
		wantScale    float64
		wantOK       bool
	}{
		{"nothing", "", 0, 0, 0, 0, false},
		{"letters", "abcDEF", 0, 0, 0, 0, false},
		{"zero", "0", 1, 0, 0, 0, true},
		{"five", "5", 1, 5, 0, 0, true},
		{"ten", "10", 2, 10, 0, 0, true},
		{"sixteen", "16", 2, 16, 0, 0, true},
		{"hundred", "100ABCdef", 3, 100, 0, 0, true},
		{"pi", "3.1415", 6, 3, 1415, 10000, true},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			gotI, gotValue, gotFraction, gotScale, err := consumeDigits(c.in)

			if ok := (err == nil); ok != c.wantOK {
				if !ok {
					t.Error("consumeDigits() err:", err)
				}
				t.Errorf("consumeDigits() ok = %v, wantOK = %v", ok, c.wantOK)
				return
			}

			if gotI != c.wantI {
				t.Errorf("consumeDigits() gotI = %v, want %v", gotI, c.wantI)
			}
			if gotValue != c.wantValue {
				t.Errorf("consumeDigits() gotValue = %v, want %v", gotValue, c.wantValue)
			}
			if gotFraction != c.wantFraction {
				t.Errorf("consumeDigits() gotFraction = %v, want %v", gotFraction, c.wantFraction)
			}
			if gotScale != c.wantScale {
				t.Errorf("consumeDigits() gotScale = %v, want %v", gotScale, c.wantScale)
			}
		})
	}
}

func TestConsumeUnit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		in              string
		wantI           int
		wantNanoseconds int64
		wantOK          bool
	}{
		{"nothing", "", 0, 0, false},
		{"digit", "0", 0, 0, false},
		{"no-unit", "QWERTY", 6, 0, false},
		{"nano", "ns", 2, 1, true},
		{"second", "s", 1, 1e9, true},
		{"minute", "m12s123us", 1, 60e9, true},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			gotI, gotNanoseconds, err := consumeUnit(c.in)

			if ok := (err == nil); ok != c.wantOK {
				if !ok {
					t.Error("consumeUnit() err:", err)
				}
				t.Errorf("consumeUnit() ok = %v, wantOK = %v", ok, c.wantOK)
				return
			}

			if gotI != c.wantI {
				t.Errorf("consumeUnit() gotI = %v, want %v", gotI, c.wantI)
			}
			if gotNanoseconds != c.wantNanoseconds {
				t.Errorf("consumeUnit() gotNanoseconds = %v, want %v", gotNanoseconds, c.wantNanoseconds)
			}
		})
	}
}

func TestComputeNanoseconds(t *testing.T) {
	t.Parallel()

	type in struct {
		unitNs  int64
		intPart int64
		fraPart int64
		scale   float64
	}

	cases := []struct {
		name            string
		in              in
		wantNanoseconds int64
		wantOK          bool
	}{
		{"10.1234ns", in{1, 10, 1234, 10000}, 10, true},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			gotNanoseconds, err := computeNanoseconds(c.in.unitNs, c.in.intPart, c.in.fraPart, c.in.scale)

			if ok := (err == nil); ok != c.wantOK {
				if !ok {
					t.Error("computeNanoseconds() err:", err)
				}
				t.Errorf("computeNanoseconds() ok = %v, wantOK = %v", ok, c.wantOK)
				return
			}

			if gotNanoseconds != c.wantNanoseconds {
				t.Errorf("computeNanoseconds() = %v, want %v", gotNanoseconds, c.wantNanoseconds)
			}
		})
	}
}

func TestIntegralPart(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		in     string
		wantV  int64
		wantI  int
		wantOK bool
	}{
		{"pi", "3.1415", 3, 1, true},
		{"dot", ".1415", 0, 0, true},
		{"unit", "year", 0, 0, true},
		{"overflow", "999999999999999999999999", 999999999999999999, 18, false},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			gotV, gotI, err := integralPart(c.in)

			if ok := (err == nil); ok != c.wantOK {
				if !ok {
					t.Error("integralPart() err:", err)
				}
				t.Errorf("integralPart() ok = %v, wantOK = %v", ok, c.wantOK)
				return
			}

			if gotV != c.wantV {
				t.Errorf("integralPart() gotV = %v, want %v", gotV, c.wantV)
			}
			if gotI != c.wantI {
				t.Errorf("integralPart() gotI = %v, want %v", gotI, c.wantI)
			}
		})
	}
}

func TestFractionalPart(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        string
		wantF     int64
		wantI     int
		wantScale float64
	}{
		{"nothing", "", 0, 0, 1},
		{"letters", "abcDEF", 0, 0, 1},
		{"dots", "....", 0, 0, 1},
		{"zero", "0", 0, 1, 10},
		{"zeros", "00000zz", 0, 5, 100000},
		{"five", "5", 5, 1, 10},
		{"ten", "10", 10, 2, 100},
		{"sixteen", "16", 16, 2, 100},
		{"hundred", "100ABCdef", 100, 3, 1000},
		{"pi", "3.1415", 3, 1, 10},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			gotF, gotI, gotScale := fractionalPart(c.in)
			if gotF != c.wantF {
				t.Errorf("fractionalPart() gotF = %v, want %v", gotF, c.wantF)
			}
			if gotI != c.wantI {
				t.Errorf("fractionalPart() gotI = %v, want %v", gotI, c.wantI)
			}
			if gotScale != c.wantScale {
				t.Errorf("fractionalPart() gotScale = %v, want %v", gotScale, c.wantScale)
			}
		})
	}
}

func TestDStr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"999d1s1ms", "999d"},
		{"999d1ms", "999d"},
		{"999d1s", "999d"},
		{"999d", ""},

		{"7d1s1ms", "7d"},
		{"7d1ms", "7d"},
		{"7d1s", "7d"},
		{"7d", ""},

		{"6d1s1ms", "6d1s"},
		{"6d1ms", "6d"},
		{"6d1s", ""},
		{"6d", ""},

		{"-7d1s1ms", "-7d"},
		{"-7d1ms", "-7d"},
		{"-7d1s", "-7d"},
		{"-7d", ""},

		{"-6d1s1ms", "-6d1s"},
		{"-6d1ms", "-6d"},
		{"-6d1s", ""},
		{"-6d", ""},

		{"-99d", ""},
		{"-99d1ms", "-99d"},
		{"-99d1s", "-99d"},
		{"-99d1s1ms", "-99d"},

		{"48h", "2d"},
		{"2d1h", "2d1h0m0s"},
		{"48h60m", "2d1h0m0s"},
		{"-48h", "-2d"},
		{"-48h60m", "-2d1h0m0s"},

		{"1h", "1h0m0s"},
		{"60m1ms", "1h0m0s"},
		{"60s1ms", "1m0s"},
		{"59s1ms", "59.001s"},

		{"-60m1ms", "-1h0m0s"},
		{"-60s1ms", "-1m0s"},
		{"-59s1ms", "-59.001s"},
	}

	for _, c := range cases {
		c := c

		t.Run(c.in, func(t *testing.T) {
			t.Parallel()

			d, err := ParseDuration(c.in)
			if err != nil {
				t.Error("ParseDuration() err", err.Error())
			}

			got := DStr(d)

			want := c.want
			if want == "" {
				want = c.in
			}

			if got != want {
				t.Errorf("DStr() = %v, want %v", got, want)
			}
		})
	}
}

func TestDT(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"23:59:59", "this string is filled in the beginning of this test"},
		{"2021-12-31" /***padding**/, "2021-12-31"},
		{"2021-12-31T00:00:00Z" /**/, "2021-12-31"},
		{"2021-12-31T23:59:59Z" /**/, "2021-12-31.23:59:59"},
		{"2021-12-31T23:59:59" /***/, "2021-12-31.23:59:59"},
		{"2021-12-31.23:59:59" /***/, "2021-12-31.23:59:59"},
		{"2021-12-31_23:59:59" /***/, "2021-12-31.23:59:59"},
	}

	t.Run("fill expected result for 23:59:59", func(_ *testing.T) {
		today := time.Now().UTC().Truncate(Day)
		str := DT(today)
		cases[0].want = str[0:10] + "." + cases[0].in
	})

	for _, c := range cases {
		c := c

		t.Run(c.in, func(t *testing.T) {
			t.Parallel()

			date, ok := ParseTime(c.in)
			if !ok {
				t.Errorf("ParseTime(%v) failed", c.in)
			}

			if got := DT(date); got != c.want {
				t.Errorf("DT(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
