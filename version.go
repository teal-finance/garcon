// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/carlmjohnson/flagx"
	"github.com/carlmjohnson/versioninfo"

	"github.com/teal-finance/garcon/timex"
)

// Version format is "Program-1.2.3".
// If the program argument is empty, the format is "v1.2.3".
// The "vvv=v1.2.3" argument can be set during build-time
// with `-ldflags"-X 'myapp/pkg/info.V=v1.2.3'"`.
// If vvv is empty, Version uses the main module version.
func Version(program, vvv string) string {
	if vvv == "" {
		vvv = versioninfo.Short()
		if vvv == "" {
			vvv = "none"
		}
	}

	if program != "" {
		program += "-"
		if len(vvv) > 1 && vvv[0] == 'v' {
			vvv = vvv[1:] // Skip the prefix "v"
		}
	}

	return program + vvv
}

// PrintVersion computes and prints the version and (Git) commit information.
//nolint:forbidigo // must print on stdout
func PrintVersion(v string) {
	if v == "" {
		v = Version("", "")
	}

	fmt.Println(v)

	short := versioninfo.Short()
	if !strings.HasSuffix(v, short) {
		fmt.Println("ShortVersion:", versioninfo.Short())
	}

	if !versioninfo.LastCommit.IsZero() {
		fmt.Println("LastCommit:", versioninfo.LastCommit.Format("2006-01-02 15:04:05"),
			"—", timex.DStr(time.Since(versioninfo.LastCommit)), "ago")
	}

	os.Exit(0)
}

// SetVersionFlag register PrintVersion() for the version flag.
//
// Example with default values:
//
//     import "github.com/teal-finance/garcon"
//
//     func main() {
//          garcon.SetVersionFlag(nil, "", "")
//          flag.Parse()
//     }
//
// Example with custom values values:
//
//     import "github.com/teal-finance/garcon"
//
//     func main() {
//          v := garcon.Version("MyApp")
//          garcon.SetVersionFlag(nil, "v", v)
//          flag.Parse()
//     }
//
func SetVersionFlag(fs *flag.FlagSet, flagName, v string) {
	if flagName == "" {
		flagName = "version" // default is: -version
	}

	f := func() error { PrintVersion(v); return nil }

	flagx.BoolFunc(fs, flagName, "Print version and exit", f)
}
