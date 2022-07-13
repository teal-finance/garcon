// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/carlmjohnson/flagx"
	"github.com/carlmjohnson/versioninfo"

	"github.com/teal-finance/garcon/timex"
)

// V is set using the following link flag `-ldflags`:
//
//    v="$(git describe --tags --always --broken)"
//    go build -ldflags="-X 'github.com/teal-finance/garcon.V=$v'" ./your/main/package
//
//nolint:gochecknoglobals // This is set at build time
var V string

// Version format is "Program-1.2.3".
// If the program argument is empty, the format is "v1.2.3".
// The "vvv=v1.2.3" argument can be set during build-time
// with `-ldflags"-X 'myapp/pkg/info.V=v1.2.3'"`.
// If vvv is empty, Version uses the main module version.
func Version(program, version string) string {
	if version == "" {
		version = V
		if version == "" {
			version = versioninfo.Short()
			if version == "" {
				version = "undefined-version"
			}
			V = version
		}
	}

	if program != "" {
		program += "-"
		if len(version) > 1 && version[0] == 'v' {
			version = version[1:] // Skip the prefix "v"
		}
	}

	return program + version
}

// PrintVersion computes the version and (Git) commit information.
func VersionInfo(version string) []string {
	info := make([]string, 0, 3)

	if version == "" {
		version = Version("", "")
	}
	info = append(info, version)

	short := versioninfo.Short()
	if !strings.HasSuffix(version, short) {
		info = append(info, fmt.Sprint("ShortVersion: ", versioninfo.Short()))
	}

	if !versioninfo.LastCommit.IsZero() {
		info = append(info, fmt.Sprint(
			"LastCommit: ", versioninfo.LastCommit.Format("2006-01-02 15:04:05"),
			" (", timex.DStr(time.Since(versioninfo.LastCommit)), " ago)"))
	}

	return info
}

// LogVersion logs the version and (Git) commit information.
func LogVersion(v string) {
	for i, line := range VersionInfo(v) {
		if i == 0 && v == "" {
			line = "Version: " + line
		}
		log.Println(line)
	}
}

// PrintVersion prints the version and (Git) commit information.
//nolint:forbidigo // must print on stdout
func PrintVersion(v string) {
	for _, line := range VersionInfo(v) {
		fmt.Println(line)
	}
	os.Exit(0)
}

// SetVersionFlag defines -version flag to print the version stored in V.
// See SetCustomVersionFlag for a more flexibility.
func SetVersionFlag() {
	SetCustomVersionFlag(nil, "", "")
}

// SetCustomVersionFlag register PrintVersion() for the version flag.
//
// Example with default values:
//
//     import "github.com/teal-finance/garcon"
//
//     func main() {
//          garcon.SetCustomVersionFlag(nil, "", "")
//          flag.Parse()
//     }
//
// Example with custom values values:
//
//     import "github.com/teal-finance/garcon"
//
//     func main() {
//          v := garcon.Version("MyApp")
//          garcon.SetCustomVersionFlag(nil, "v", v)
//          flag.Parse()
//     }
//
func SetCustomVersionFlag(fs *flag.FlagSet, flagName, v string) {
	if flagName == "" {
		flagName = "version" // default flag is: -version
	}

	f := func() error { PrintVersion(v); return nil }

	flagx.BoolFunc(fs, flagName, "Print version and exit", f)
}
