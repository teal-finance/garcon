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

import "net"

type Token struct {
	Expiry uint64 // Unix time (seconds since epoch) in UTC zone
	IP     net.IP
	Values [][]byte
}

func (token *Token) ShortenIP() {
	if v4 := token.IP.To4(); v4 != nil {
		token.IP = v4
	}
}
