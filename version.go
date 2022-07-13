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
//    go build -ldflags="-X 'github.com/teal-finance/garcon.V=$v'" ./cmd/main/package
//
// The following formats the version as "v1.2.0-my-branch+3".
// The trailing "+3" is the number of commits since v1.2.0.
// If no tag in Git repo, $t is the current commit long SHA1.
//
//    t="$(git describe --tags --abbrev=0 --always)"
//    b="$(git branch --show-current)"
//    [[ $b == main ]] && b="" || b="-$b"
//    n="$(git rev-list --count "$t"..)"
//    [[ $n == 0 ]] && n="" || n="+$n"
//    go build -ldflags="-X 'github.com/teal-finance/garcon.V=$t$b$n'" ./cmd/main/package
//
//nolint:gochecknoglobals,varnamelen // set at build time: should be global and short.
var V string

// Version format is "Program-1.2.3".
// If the program argument is empty, the format is "v1.2.3".
// If V is empty, Version uses the main module version.
func Version(program string) string {
	if V == "" {
		V = versioninfo.Short()
		if V == "" {
			V = "undefined-version"
		}
	}

	if program == "" {
		return V
	}

	program += "-"

	if len(V) > 1 && V[0] == 'v' {
		return program + V[1:] // Skip the prefix "v"
	}

	return program + V
}

// PrintVersion computes the version and (Git) commit information.
func VersionInfo(program string) []string {
	info := make([]string, 0, 3)
	info = append(info, Version(program))

	short := versioninfo.Short()
	if !strings.HasSuffix(V, short) {
		info = append(info, "ShortVersion: "+versioninfo.Short())
	}

	if !versioninfo.LastCommit.IsZero() {
		line := "LastCommit: " + versioninfo.LastCommit.Format("2006-01-02 15:04:05")
		line += " (" + timex.DStr(time.Since(versioninfo.LastCommit)) + " ago)"
		info = append(info, line)
	}

	return info
}

// LogVersion logs the version and (Git) commit information.
func LogVersion() {
	for i, line := range VersionInfo("") {
		if i == 0 {
			line = "Version: " + line
		}
		log.Println(line)
	}
}

// PrintVersion prints the version and (Git) commit information.
//nolint:forbidigo // must print on stdout
func PrintVersion(program string) {
	for _, line := range VersionInfo(program) {
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
//          garcon.SetCustomVersionFlag(nil, "v", "MyApp")
//          flag.Parse()
//     }
//
func SetCustomVersionFlag(fs *flag.FlagSet, flagName, program string) {
	if flagName == "" {
		flagName = "version" // default flag is: -version
	}

	f := func() error { PrintVersion(program); return nil }

	flagx.BoolFunc(fs, flagName, "Print version and exit", f)
}
