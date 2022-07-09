// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import "fmt"

// ConvertSize converts a size in bytes into
// the most appropriate unit among KiB, MiB, GiB, TiB, PiB and EiB.
// 1 KiB is 1024 bytes as defined by the ISO/IEC 80000-13:2008 standard. See:
// https://wikiless.org/wiki/ISO%2FIEC_80000#Units_of_the_ISO_and_IEC_80000_series
func ConvertSize(sizeInBytes int) string {
	return ConvertSize64(int64(sizeInBytes))
}

// ConvertSize64 is similar ConvertSize but takes in input an int64.
func ConvertSize64(sizeInBytes int64) string {
	const unit int64 = 1024

	if sizeInBytes < unit {
		return fmt.Sprintf("%d B", sizeInBytes)
	}

	div, exp := unit, 0
	for n := sizeInBytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	v := float64(sizeInBytes) / float64(div)
	return fmt.Sprintf("%.1f %ciB", v, "KMGTPE"[exp])
}
