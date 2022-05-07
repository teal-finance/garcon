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

package session

import (
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/session/dtoken"
	"github.com/teal-finance/garcon/session/incorruptible/bits"
)

var expiry = time.Date(bits.ExpiryStartYear, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

var cases = []struct {
	name    string
	wantErr bool
	dtoken  dtoken.DToken
}{
	{
		"noIP", false, dtoken.DToken{
			Expiry: expiry,
			IP:     nil,
			Values: nil,
		},
	},
	{
		"noIPnoExpiry", false, dtoken.DToken{
			Expiry: 0,
			IP:     nil,
			Values: nil,
		},
	},
	{
		"noExpiry", false, dtoken.DToken{
			Expiry: 0,
			IP:     net.IPv4(0, 0, 0, 0),
			Values: nil,
		},
	},
	{
		"noneIPv4", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IPv4(11, 22, 33, 44),
			Values: nil,
		},
	},
	{
		"noneIPv6", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{},
		},
	},
	{
		"1emptyIPv6", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{[]byte("")},
		},
	},
	{
		"4emptyIPv6", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{[]byte(""), []byte(""), []byte(""), []byte("")},
		},
	},
	{
		"1smallIPv6", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{[]byte("1")},
		},
	},
	{
		"1valIPv6", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{[]byte("123456789-B-123456789-C-123456789-D-123456789-E-123456789")},
		},
	},
	{
		"1moreIPv6", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{[]byte("123456789-B-123456789-C-123456789-D-123456789-E-123456789-")},
		},
	},
	{
		"Compress 10valIPv6", false, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{
				[]byte("123456789-B-123456789-C-123456789-D-123456789-E-123456789"),
				[]byte("123456789-F-123456789-C-123456789-D-123456789-E-123456789"),
				[]byte("123456789-G-123456789-C-123456789-D-123456789-E-123456789"),
				[]byte("123456789-H-123456789-C-123456789-D-123456789-E-123456789"),
				[]byte("123456789-I-123456789-C-123456789-D-123456789-E-123456789"),
				[]byte("123456789-J-123456789-C-123456789-D-123456789-E-123456789"),
				[]byte("123456789-K-123456789-C-123456789-D-123456789-E-123456789"),
			},
		},
	},
	{
		"too much values", true, dtoken.DToken{
			Expiry: expiry,
			IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			Values: [][]byte{{1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}, {9},
				{10}, {11}, {12}, {13}, {14}, {15}, {16}, {17}, {18}, {19},
				{20}, {21}, {22}, {23}, {24}, {25}, {26}, {27}, {28}, {29},
				{30}, {31}, {32}, {33}, {34}, {35}, {36}, {37}, {38}, {39},
				{40}, {41}, {42}, {43}, {44}, {45}, {46}, {47}, {48}, {49},
				{50}, {51}, {52}, {53}, {54}, {55}, {56}, {57}, {58}, {59},
				{60}, {61}, {62}, {63}, {64}, {65}, {66}, {67}, {68}, {69}},
		},
	},
}

func TestDecode(t *testing.T) {
	for _, c := range cases {
		u, err := url.Parse("http://host:8080/path/url")
		if err != nil {
			t.Error("url.Parse() error", err)
			return
		}

		key := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6}

		s := New([]*url.URL{u}, reserr.New("path/doc"), key[:], 0, true)

		t.Run(c.name, func(t *testing.T) {
			c.dtoken.ShortenIP()

			str, err := s.Encode(c.dtoken)
			if (err == nil) == c.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, c.wantErr)
				return
			}

			t.Log("len(str)", len(str))

			n := len(str)
			if n == 0 {
				return
			}
			if n > 70 {
				n = 70 // print max the first 70 characters
			}
			t.Logf("str len=%d [:%d]=%q", len(str), n, str[:n])

			got, err := s.Decode(str)
			if err != nil {
				t.Errorf("Decode() error = %v", err)
				return
			}

			min := c.dtoken.Expiry - bits.PrecisionInSeconds
			max := c.dtoken.Expiry + bits.PrecisionInSeconds
			validExpiry := (min <= got.Expiry) && (got.Expiry <= max)
			if !validExpiry {
				t.Errorf("Expiry too different got=%v original=%v want in [%d %d]",
					got.Expiry, c.dtoken.Expiry, min, max)
			}

			if (len(got.IP) > 0 || len(c.dtoken.IP) > 0) &&
				!reflect.DeepEqual(got.IP, c.dtoken.IP) {
				t.Errorf("Mismatch IP got %v, want %v", got.IP, c.dtoken.IP)
			}

			if (len(got.Values) > 0 || len(c.dtoken.Values) > 0) &&
				!reflect.DeepEqual(got.Values, c.dtoken.Values) {
				t.Errorf("Mismatch Values got %v, want %v", got.Values, c.dtoken.Values)
			}

			cookie, err := s.NewCookie(c.dtoken)
			if err != nil {
				t.Errorf("NewCookie() %v", err)
				return
			}

			err = cookie.Valid()
			if err != nil {
				t.Errorf("Invalid cookie %v", err)
				return
			}
		})
	}
}
