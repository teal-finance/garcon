// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/carlmjohnson/flagx"
	"github.com/carlmjohnson/versioninfo"

	"github.com/teal-finance/garcon/timex"
)

// V is set at build time using the `-ldflags` build flag:
//
//    v="$(git describe --tags --always --broken)"
//    go build -ldflags="-X 'github.com/teal-finance/garcon.V=$v'" ./cmd/main/package
//
// The following commands provide a semver-like version format such as
// "v1.2.0-my-branch+3" where "+3" is the number of commits since "v1.2.0".
// If no tag in the Git repo, $t is the long SHA1 of the last commit.
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

// PrintVersion prints the version and (Git) commit information.
//nolint:forbidigo // must print on stdout
func PrintVersion(program string) {
	for _, line := range versionStrings(program) {
		fmt.Println(line)
	}
	os.Exit(0)
}

// LogVersion logs the version and (Git) commit information.
func LogVersion() {
	for i, line := range versionStrings("") {
		if i == 0 {
			line = "Version: " + line
		}
		log.Println(line)
	}
}

// versionStrings computes the version and (Git) commit information.
func versionStrings(program string) []string {
	vi := make([]string, 0, 3)
	vi = append(vi, Version(program))

	if info.Short != "" {
		vi = append(vi, "ShortVersion: "+info.Short)
	}

	if info.LastCommit != "" {
		line := "LastCommit: " + info.LastCommit
		line += " (" + sinceLastCommit() + " ago)"
		vi = append(vi, line)
	}

	return vi
}

func sinceLastCommit() string {
	if versioninfo.LastCommit.IsZero() {
		return ""
	}
	return timex.DStr(time.Since(versioninfo.LastCommit))
}

//nolint:gochecknoglobals // set at startup time
// except field Ago that is updated (possible race condition).
var info = versionStruct("")

// versionInfo is used to generate a fast JSON marshaler.
type versionInfo struct {
	Version    string
	Short      string
	LastCommit string
	Ago        string
}

// versionStruct computes the version and (Git) commit information.
func versionStruct(program string) versionInfo {
	var vi versionInfo

	vi.Version = Version(program)

	short := versioninfo.Short()
	if !strings.HasSuffix(V, short) {
		vi.Short = versioninfo.Short()
	}

	if !versioninfo.LastCommit.IsZero() {
		vi.LastCommit = versioninfo.LastCommit.Format("2006-01-02 15:04:05")
	}

	return vi
}

const html = `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Version Info</title>
</head>
<body>
	{{range .Items}}<div>{{ . }}</div>{{else}}<div><strong>no version</strong></div>{{end}}
</body>
</html>`

// ServeVersion send HTML or JSON depending on Accept header.
func (g *Garcon) ServeVersion() func(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("version").Parse(html)
	if err != nil {
		log.Panic("ServeVersion template.New:", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "json") {
			writeJSON(w)
		} else {
			writeHTML(w, tmpl)
		}
	}
}

// writeJSON converts the version info from string slice to JSON.
func writeJSON(w http.ResponseWriter) {
	info.Ago = sinceLastCommit()
	b, err := info.MarshalJSON()
	if err != nil {
		log.Print("WRN ServeVersion MarshalJSON: ", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// writeHTML converts the version info from string slice to JSON.
func writeHTML(w http.ResponseWriter, tmpl *template.Template) {
	info := versionStrings("")
	data := struct{ Items []string }{info}
	if err := tmpl.Execute(w, data); err != nil {
		log.Print("WRN ServeVersion Execute:", err)
		w.WriteHeader(http.StatusNoContent)
	}
}
