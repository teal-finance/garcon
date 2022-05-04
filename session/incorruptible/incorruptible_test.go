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

package incorruptible

import (
	"net"
	"reflect"
	"testing"

	"github.com/teal-finance/garcon/session/token"
)

func TestUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		magic   uint8
		wantErr bool
		token   token.Token
	}{
		{
			"noneIPv4", 0x51, false,
			token.Token{
				Expiry: 123456789,
				IP:     net.IPv4(11, 22, 33, 44),
				Values: [][]byte{},
			},
		},
		{
			"noneIPv6", 0x51, false,
			token.Token{
				Expiry: 123456789,
				IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Values: [][]byte{},
			},
		},
		{
			"1emptyIPv6", 0x51, false,
			token.Token{
				Expiry: 123456789,
				IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Values: [][]byte{[]byte("")},
			},
		},
		{
			"4emptyIPv6", 0x51, false,
			token.Token{
				Expiry: 123456789,
				IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Values: [][]byte{[]byte(""), []byte(""), []byte(""), []byte("")},
			},
		},
		{
			"1smallIPv6", 0x51, false,
			token.Token{
				Expiry: 123456789,
				IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Values: [][]byte{[]byte("1")},
			},
		},
		{
			"1valIPv6", 0x51, false,
			token.Token{
				Expiry: 123456789,
				IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Values: [][]byte{[]byte("123456789-B-123456789-C-123456789-D-123456789-E-123456789")},
			},
		},
		{
			"1moreIPv6", 0x51, false,
			token.Token{
				Expiry: 123456789,
				IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Values: [][]byte{[]byte("123456789-B-123456789-C-123456789-D-123456789-E-123456789-")},
			},
		},
		{
			"Compress 10valIPv6", 0x51, false,
			token.Token{
				Expiry: 123456789,
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
			"too much values", 0x51, true,
			token.Token{
				Expiry: 123456789,
				IP:     net.IP{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Values: [][]byte{{1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}, {9},
					{10}, {11}, {12}, {13}, {14}, {15}, {16}, {17}, {18}, {19},
					{20}, {21}, {22}, {23}, {24}, {25}, {26}, {27}, {28}, {29},
					{30}, {31}, {32}, {33}, {34}, {35}, {36}, {37}, {38}, {39},
					{40}, {41}, {42}, {43}, {44}, {45}, {46}, {47}, {48}, {49},
					{50}, {51}, {52}, {53}, {54}, {55}, {56}, {57}, {58}, {59},
					{60}, {61}, {62}, {63}, {64}, {65}, {66}, {67}, {68}, {69}}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.token.ShortenIP()

			b, err := Marshal(c.token, c.magic)
			if (err == nil) == c.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, c.wantErr)
				return
			}

			t.Log("len(b)", len(b))

			n := 70
			if n > len(b) {
				n = len(b)
			}
			if n == 0 {
				return
			}

			t.Logf("b[:%d] %v", n, b[:n])

			magic := MagicCode(b)
			if magic != c.magic {
				t.Errorf("MagicCode() got = %x, want = %x", magic, c.magic)
				return
			}

			got, err := Unmarshal(b)
			if err != nil {
				t.Errorf("Unmarshal() error = %v", err)
				return
			}

			if !reflect.DeepEqual(got, c.token) {
				t.Errorf("Unmarshal() = %v, want %v", got, c.token)
			}
		})
	}
}
