// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/carlmjohnson/flagx"
	"github.com/carlmjohnson/versioninfo"

	"github.com/teal-finance/garcon/gg"
	"github.com/teal-finance/garcon/timex"
)

// V is set at build time using the `-ldflags` build flag:
//
//	v="$(git describe --tags --always --broken)"
//	go build -ldflags="-X 'github.com/teal-finance/garcon.V=$v'" ./cmd/main/package
//
// The following commands provide a semver-like version format such as
// "v1.2.0-my-branch+3" where "+3" is the number of commits since "v1.2.0".
// If no tag in the Git repo, $t is the long SHA1 of the last commit.
//
//	t="$(git describe --tags --abbrev=0 --always)"
//	b="$(git branch --show-current)"
//	[ _$b = _main ] && b="" || b="-$b"
//	n="$(git rev-list --count "$t"..)"
//	[ "$n" -eq 0 ] && n="" || n="+$n"
//	go build -ldflags="-X 'github.com/teal-finance/garcon.V=$t$b$n'" ./cmd/main/package
//
//nolint:gochecknoglobals,varnamelen // set at build time: should be global and short.
var V string

// Version format is "Program-1.2.3".
// If the program argument is empty, the format is "v1.2.3".
// If V is empty, Version uses the main module version.
func Version(serverName string) string {
	if V == "" {
		V = versioninfo.Short()
		if V == "" {
			V = "undefined-version"
		}
	}

	if serverName == "" {
		return V
	}

	serverName += "-"

	if len(V) > 1 && V[0] == 'v' {
		return serverName + V[1:] // Skip the prefix "v"
	}

	return serverName + V
}

// SetVersionFlag defines -version flag to print the version stored in V.
// See SetCustomVersionFlag for a more flexibility.
func SetVersionFlag() {
	SetCustomVersionFlag(nil, "", "")
}

// SetCustomVersionFlag register PrintVersionAndExit() for the -version flag.
//
// Example with default values:
//
//	import "github.com/teal-finance/garcon"
//
//	func main() {
//	     garcon.SetCustomVersionFlag(nil, "", "")
//	     flag.Parse()
//	}
//
// Example with custom values values:
//
//	import "github.com/teal-finance/garcon"
//
//	func main() {
//	     garcon.SetCustomVersionFlag(nil, "v", "MyApp")
//	     flag.Parse()
//	}
func SetCustomVersionFlag(fs *flag.FlagSet, flagName, program string) {
	if flagName == "" {
		flagName = "version" // default flag is: -version
	}

	f := func() error { PrintVersionAndExit(program); return nil }

	flagx.BoolFunc(fs, flagName, "Print version and exit", f)
}

// PrintVersionAndExit prints the version and exits.
// The version may also contain the (Git) commit information.
//
//nolint:forbidigo // must print on stdout
func PrintVersionAndExit(program string) {
	for _, line := range versionStrings(program) {
		fmt.Println(line)
	}
	os.Exit(0)
}

// LogVersion logs the version and (Git) commit information.
func LogVersion() {
	noProgramName := ""
	for i, line := range versionStrings(noProgramName) {
		if i == 0 {
			line = "Version: " + line
		}
		log.Init(line)
	}
}

// versionStrings computes the version and (Git) commit information.
func versionStrings(program string) []string {
	lines := make([]string, 0, 3)
	lines = append(lines, Version(program))

	if info.Short != "" {
		lines = append(lines, "ShortVersion: "+info.Short)
	}

	if info.LastCommit != "" {
		last := "LastCommit: " + info.LastCommit
		last += " (" + sinceLastCommit() + " ago)"
		lines = append(lines, last)
	}

	return lines
}

func sinceLastCommit() string {
	if versioninfo.LastCommit.IsZero() {
		return ""
	}
	return timex.DStr(time.Since(versioninfo.LastCommit))
}

// info is not a runtime constant because
// the field Ago may be updated during the execution.
//
//nolint:gochecknoglobals // set at startup time
var info = initVersionInfo()

// versionInfo is used to generate a fast JSON marshaler.
type versionInfo struct {
	Version    string
	Short      string
	LastCommit string
	Ago        string
}

// initVersionInfo computes the version and commit information (Git).
func initVersionInfo() versionInfo {
	var vi versionInfo

	noProgramName := ""
	vi.Version = Version(noProgramName)

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
	{{range .Items}}<div>{{ . }}</div>{{else}}<div>no version</div>{{end}}
</body>
</html>`

// ServeVersion send HTML or JSON depending on Accept header.
func ServeVersion() func(w http.ResponseWriter, r *http.Request) {
	t, err := template.New("version").Parse(html)
	if err != nil {
		log.Panic("ServeVersion template.New:", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "json") {
			writeJSON(w)
		} else {
			writeHTML(w, t)
		}
	}
}

// writeJSON converts the version info from string slice to JSON.
func writeJSON(w http.ResponseWriter) {
	info.Ago = sinceLastCommit()
	b, err := info.MarshalJSON()
	if err != nil {
		log.Warn("writeJSON MarshalJSON:", err)
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
}

// writeHTML converts the version info from string slice to JSON.
func writeHTML(w http.ResponseWriter, t *template.Template) {
	noProgramName := ""
	lines := versionStrings(noProgramName)
	data := struct{ Items []string }{lines}
	if err := t.Execute(w, data); err != nil {
		log.Warn("writeHTML Execute:", err)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (g *Garcon) MiddlewareServerHeader(serverName ...string) gg.Middleware {
	name := g.ServerName.String()
	if len(serverName) > 0 && serverName[0] != "" {
		name = serverName[0]
	}

	version := Version(name)

	return MiddlewareServerHeader(version)
}

// MiddlewareServerHeader is the middleware setting the Server HTTP header in the response.
func MiddlewareServerHeader(version string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		log.Info("MiddlewareServerHeader sets the HTTP header Server=" + version + " in the responses")

		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Server", version)
				next.ServeHTTP(w, r)
			})
	}
}
