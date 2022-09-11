// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

// Package garcon is a server for API and static website
// including middlewares to manage rate-limit, Cookies, JWT,
// CORS, OPA, web traffic, Prometheus export and PProf.
package garcon

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/emo"
	"github.com/teal-finance/incorruptible"
)

var log = emo.NewZone("garcon")

type Garcon struct {
	ServerName ServerName
	Writer     Writer
	docURL     string
	urls       []*url.URL
	origins    []string
	pprofPort  int
	devMode    bool
}

func (g Garcon) IsDevMode() bool { return g.devMode }

func New(opts ...Option) *Garcon {
	var g Garcon
	for _, opt := range opts {
		if opt != nil {
			opt(&g)
		}
	}

	StartPProfServer(g.pprofPort)

	// namespace fallback = retrieve it from first URL
	if g.ServerName == "" && len(g.urls) > 0 {
		g.ServerName = ExtractName(g.urls[0].String())
	}

	// set CORS origins
	if len(g.urls) == 0 {
		g.urls = DevOrigins()
	} else if g.devMode {
		g.urls = AppendURLs(g.urls, DevOrigins()...)
	}
	g.origins = OriginsFromURLs(g.urls)

	if len(g.docURL) > 0 {
		// if docURL is just a path => complete it with the base URL (scheme + host)
		baseURL := g.urls[0].String()
		if !strings.HasPrefix(g.docURL, baseURL) &&
			!strings.Contains(g.docURL, "://") {
			g.docURL = baseURL + g.docURL
		}
	}
	g.Writer = NewWriter(g.docURL)

	return &g
}

type Option func(*Garcon)

func WithServerName(str string) Option {
	return func(g *Garcon) {
		g.ServerName = ExtractName(str)
	}
}

func WithDocURL(docURL string) Option {
	return func(g *Garcon) {
		g.docURL = docURL
	}
}

func WithDev(enable ...bool) Option {
	devMode := true
	if len(enable) > 0 {
		devMode = enable[0]

		if len(enable) >= 2 {
			log.Panic("garcon.WithDev() must be called with zero or one argument")
		}
	}

	return func(g *Garcon) {
		g.devMode = devMode
	}
}

func WithPProf(port int) Option {
	return func(g *Garcon) {
		g.pprofPort = port
	}
}

func WithURLs(addresses ...string) Option {
	return func(g *Garcon) {
		g.urls = ParseURLs(addresses)
	}
}

// ListenAndServe runs the HTTP server(s) in foreground.
// Optionally it also starts a metrics server in background (if export port > 0).
// The metrics server is for use with Prometheus or another compatible monitoring tool.
func ListenAndServe(server *http.Server) error {
	log.Print("Server listening on http://localhost" + server.Addr)

	err := server.ListenAndServe()

	_, port, e := net.SplitHostPort(server.Addr)
	if e == nil {
		log.Error("Install ncat and ss: sudo apt install ncat iproute2")
		log.Errorf("Try to listen port %v: sudo ncat -l %v", port, port)
		log.Errorf("Get the process using port %v: sudo ss -pan | grep %v", port, port)
	}

	return err
}

// Server returns a default http.Server ready to handle API endpoints, static web pages...
func Server(h http.Handler, port int, connState ...func(net.Conn, http.ConnState)) http.Server {
	if len(connState) == 0 {
		connState = []func(net.Conn, http.ConnState){nil}
	}

	return http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           h,
		TLSConfig:         nil,
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Minute, // Garcon.MiddlewareRateLimiter() delays responses, so people (attackers) who click frequently will wait longer.
		IdleTimeout:       time.Second,
		MaxHeaderBytes:    444, // 444 bytes should be enough
		TLSNextProto:      nil,
		ConnState:         connState[0],
		ErrorLog:          log.Default(),
		BaseContext:       nil,
		ConnContext:       nil,
	}
}

// TokenChecker is the common interface to Incorruptible and JWTChecker.
type TokenChecker interface {
	// Set is a middleware setting a cookie in the response when the request has no valid token.
	// Set searches the token in a cookie and in the first "Authorization" header.
	// Finally, Set stores the decoded token fields within the request context.
	Set(next http.Handler) http.Handler

	// Chk is a middleware accepting requests only if it has a valid cookie:
	// other requests are rejected with http.StatusUnauthorized.
	// Chk does not verify the "Authorization" header.
	// See also the Vet() function if the token should also be verified in the "Authorization" header.
	// Finally, Chk stores the decoded token fields within the request context.
	// In dev. mode, Chk accepts any request but does not store invalid tokens.
	Chk(next http.Handler) http.Handler

	// Vet is a middleware accepting accepting requests having a valid token
	// either in the cookie or in the first "Authorization" header:
	// other requests are rejected with http.StatusUnauthorized.
	// Vet also stores the decoded token in the request context.
	// In dev. mode, Vet accepts any request but does not store invalid tokens.
	Vet(next http.Handler) http.Handler

	// Cookie returns a default cookie to facilitate testing.
	Cookie(i int) *http.Cookie
}

// IncorruptibleChecker uses cookies based the fast and tiny Incorruptible token.
// IncorruptibleChecker requires g.WithURLs() to set the Cookie secure, domain and path.
func (g *Garcon) IncorruptibleChecker(secretKeyHex string, maxAge int, setIP bool) *incorruptible.Incorruptible {
	if len(secretKeyHex) != 32 {
		log.Panic("Want AES-128 key composed by 32 hexadecimal digits, but got", len(secretKeyHex), "digits")
	}
	key, err := hex.DecodeString(secretKeyHex)
	if err != nil {
		log.Panic("Cannot decode the 128-bit AES key, please provide 32 hexadecimal digits:", err)
	}

	return g.IncorruptibleCheckerBin(key, maxAge, setIP)
}

// IncorruptibleChecker uses cookies based the fast and tiny Incorruptible token.
// IncorruptibleChecker requires g.WithURLs() to set the Cookie secure, domain and path.
func (g *Garcon) IncorruptibleCheckerBin(secretKeyBin []byte, maxAge int, setIP bool) *incorruptible.Incorruptible {
	if len(secretKeyBin) != 16 {
		log.Panic("Want AES-128 key composed by 16 bytes, but got", len(secretKeyBin), "bytes")
	}

	if len(g.urls) == 0 {
		log.Panic("Missing URLs => Set first the URLs with garcon.WithURLs()")
	}

	cookieName := string(g.ServerName)
	return incorruptible.New(g.Writer.WriteErr, g.urls, secretKeyBin, cookieName, maxAge, setIP)
}

// JWTChecker requires WithURLs() to set the Cookie name, secure, domain and path.
func (g *Garcon) JWTChecker(secretKeyHex string, planPerm ...any) *JWTChecker {
	if len(g.urls) == 0 {
		log.Panic("Missing URLs => Set first the URLs with garcon.WithURLs()")
	}

	return NewJWTChecker(g.Writer, g.urls, secretKeyHex, planPerm...)
}

// OverwriteBufferContent is to erase a secret when it is no longer required.
func OverwriteBufferContent(b []byte) {
	//nolint:gosec // does not matter if written bytes are not good random values
	// "math/rand" is 40 times faster than "crypto/rand"
	// see: https://github.com/SimonWaldherr/golang-benchmarks#random
	_, _ = rand.Read(b)
}

// SplitClean splits the values and sanitize them.
func SplitClean(values string, separators ...rune) []string {
	list := Split(values, separators...)
	result := make([]string, 0, len(list))
	for _, v := range list {
		v = strings.TrimSpace(v)
		v = Sanitize(v)
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

func Split(values string, separators ...rune) []string {
	f := separatorFunc(separators...)
	return strings.FieldsFunc(values, f)
}

func separatorFunc(separators ...rune) func(rune) bool {
	if len(separators) == 0 {
		return isSeparator
	}

	return func(r rune) bool {
		for _, s := range separators {
			if s == r {
				return true
			}
		}
		return false
	}
}

func isSeparator(r rune) bool {
	switch {
	case r <= 32, // tabulation, carriage return, line feed, space...
		r == ',',    // COMMA
		r == 0x007F, // DELETE
		r == 0x0085, // NEXT LINE (NEL)
		r == 0x00A0: // NO-BREAK SPACE
		return true
	}
	return false
}

func AppendPrefixes(origins []string, prefixes ...string) []string {
	for _, p := range prefixes {
		origins = appendOnePrefix(origins, p)
	}
	return origins
}

func appendOnePrefix(origins []string, prefix string) []string {
	for i, url := range origins {
		// if `url` is already a prefix of `prefix` => stop
		if len(url) <= len(prefix) {
			if url == prefix[:len(url)] {
				return origins
			}
			continue
		}

		// preserve origins[0]
		if i == 0 {
			continue
		}

		// if `prefix` is a prefix of `url` => update origins[i]
		if url[:len(prefix)] == prefix {
			origins[i] = prefix // replace `o` by `p`
			return origins
		}
	}

	return append(origins, prefix)
}

func AppendURLs(urls []*url.URL, prefixes ...*url.URL) []*url.URL {
	for _, p := range prefixes {
		urls = appendOneURL(urls, p)
	}
	return urls
}

func appendOneURL(urls []*url.URL, prefix *url.URL) []*url.URL {
	for i, url := range urls {
		if url.Scheme != prefix.Scheme {
			continue
		}

		// if `url` is already a prefix of `prefix` => stop
		if len(url.Host) <= len(prefix.Host) {
			if url.Host == prefix.Host[:len(url.Host)] {
				return urls
			}
			continue
		}

		// preserve urls[0]
		if i == 0 {
			continue
		}

		// if `prefix` is a prefix of `url` => update urls[i]
		if url.Host[:len(prefix.Host)] == prefix.Host {
			urls[i] = prefix // replace `u` by `prefix`
			return urls
		}
	}

	return append(urls, prefix)
}

func ParseURLs(origins []string) []*url.URL {
	urls := make([]*url.URL, 0, len(origins))

	for _, o := range origins {
		u, err := url.ParseRequestURI(o) // strip #fragment
		if err != nil {
			log.Panic("WithURLs:", err)
		}

		if u.Host == "" {
			log.Panic("WithURLs: missing host in", o)
		}

		urls = append(urls, u)
	}

	return urls
}

func OriginsFromURLs(urls []*url.URL) []string {
	origins := make([]string, 0, len(urls))
	for _, u := range urls {
		o := u.Scheme + "://" + u.Host
		origins = append(origins, o)
	}
	return origins
}

var ErrNonPrintable = errors.New("non-printable")

// Value returns the /endpoint/{key} (URL path)
// else the "key" form (HTTP body)
// else the "key" query string (URL)
// else the HTTP header.
// Value requires chi.URLParam().
func Value(r *http.Request, key, header string) (string, error) {
	value := chi.URLParam(r, key)
	if value == "" {
		value = r.FormValue(key)
		if value == "" && header != "" {
			// Check only the first Header,
			// because we do not know how to manage several ones.
			value = r.Header.Get(header)
		}
	}

	if i := printable(value); i >= 0 {
		return value, fmt.Errorf("%s %w at %d", key, ErrNonPrintable, i)
	}
	return value, nil
}

// Values requires chi.URLParam().
func Values(r *http.Request, key string) ([]string, error) {
	form := r.Form[key]

	if i := Printable(form...); i >= 0 {
		return form, fmt.Errorf("%s %w at %d", key, ErrNonPrintable, i)
	}

	// no need to test v because Garcon already verifies the URI
	if v := chi.URLParam(r, key); v != "" {
		return append(form, v), nil
	}

	return form, nil
}

func ReadRequest(w http.ResponseWriter, r *http.Request, maxBytes ...int) ([]byte, error) {
	return readBodyAndError(w, "", r.Body, r.Header, maxBytes...)
}

func ReadResponse(r *http.Response, maxBytes ...int) ([]byte, error) {
	return readBodyAndError(nil, statusErr(r), r.Body, r.Header, maxBytes...)
}

// UnmarshalJSONRequest unmarshals the JSON from the request body.
func UnmarshalJSONRequest[T json.Unmarshaler](w http.ResponseWriter, r *http.Request, msg T, maxBytes ...int) error {
	return unmarshalJSON(w, "", r.Body, r.Header, msg, maxBytes...)
}

// UnmarshalJSONResponse unmarshals the JSON from the request body.
func UnmarshalJSONResponse[T json.Unmarshaler](r *http.Response, msg T, maxBytes ...int) error {
	return unmarshalJSON(nil, statusErr(r), r.Body, r.Header, msg, maxBytes...)
}

// DecodeJSONRequest decodes the JSON from the request body.
func DecodeJSONRequest(w http.ResponseWriter, r *http.Request, msg any, maxBytes ...int) error {
	return decodeJSON(w, "", r.Body, r.Header, msg, maxBytes...)
}

// DecodeJSONResponse decodes the JSON from the request body.
func DecodeJSONResponse(r *http.Response, msg any, maxBytes ...int) error {
	return decodeJSON(nil, statusErr(r), r.Body, r.Header, msg, maxBytes...)
}

func statusErr(r *http.Response) string {
	ok := 200 <= r.StatusCode && r.StatusCode <= 299
	if ok {
		return ""
	}
	return r.Status
}

// unmarshalJSON unmarshals the JSON body of either a request or a response.
func unmarshalJSON[T json.Unmarshaler](w http.ResponseWriter, statusErr string, body io.ReadCloser, header http.Header, msg T, maxBytes ...int) error {
	buf, err := readBodyAndError(w, statusErr, body, header, maxBytes...)
	if err != nil {
		return err
	}

	err = msg.UnmarshalJSON(buf)
	if err != nil {
		return fmt.Errorf("unmarshalJSON %w got: %s", err, extractReadable(buf, header))
	}
	return nil
}

// decodeJSON decodes the JSON body of either a request or a response.
// decodeJSON does not use son.NewDecoder(body).Decode(msg)
// because we want to read again the body in case of error.
func decodeJSON(w http.ResponseWriter, statusErr string, body io.ReadCloser, header http.Header, msg any, maxBytes ...int) error {
	buf, err := readBodyAndError(w, statusErr, body, header, maxBytes...)
	if err != nil {
		return err
	}

	err = json.Unmarshal(buf, msg)
	if err != nil {
		return fmt.Errorf("decodeJSON %w got: %s", err, extractReadable(buf, header))
	}

	return nil
}

func readBodyAndError(w http.ResponseWriter, statusErr string, body io.ReadCloser, header http.Header, maxBytes ...int) ([]byte, error) {
	buf, err := readBody(w, body, maxBytes...)
	if err != nil {
		return nil, err
	}

	if statusErr != "" { // status code is always from a response
		return buf, errorFromResponseBody(statusErr, header, buf)
	}

	return buf, nil
}

const defaultMaxBytes = 80_000 // 80 KB should be enough for most of the cases

// readBody reads up to maxBytes.
func readBody(w http.ResponseWriter, body io.ReadCloser, maxBytes ...int) ([]byte, error) {
	max := defaultMaxBytes // optional parameter
	if len(maxBytes) > 0 {
		max = maxBytes[0]
	}

	if max > 0 { // protect against body abnormally too large
		body = http.MaxBytesReader(w, body, int64(max))
	}

	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("body (max=%s): %w", ConvertSize(max), err)
	}

	// check limit
	nearTheLimit := (max - len(buf)) < max/2
	readManyBytes := len(buf) > 8*defaultMaxBytes
	if nearTheLimit || readManyBytes {
		percentage := 100 * len(buf) / max
		if nearTheLimit {
			log.Warnf("body: read %s = %d%% of the limit %s, please increase maxBytes=%d", ConvertSize(len(buf)), percentage, ConvertSize(max), max)
		} else {
			log.Infof("body: read many bytes %s but only %d%% of the limit %s (%d bytes)", ConvertSize(len(buf)), percentage, ConvertSize(max), max)
		}
	}

	return buf, nil
}

func errorFromResponseBody(statusErr string, header http.Header, buf []byte) error {
	if len(buf) == 0 {
		return errors.New("(empty body)")
	}

	str := "response: " + statusErr + " (" + ConvertSize(len(buf)) + ") " + extractReadable(buf, header)
	return errors.New(str)
}

func extractReadable(buf []byte, header http.Header) string {
	// convert HTML body to markdown
	if buf[0] == byte('<') || isHTML(header) {
		converter := md.NewConverter("", true, nil)
		markdown, e := converter.ConvertBytes(buf)
		if e != nil {
			buf = append([]byte("html->md: "), markdown...)
		}
	}

	safe := Sanitize(string(buf))

	if len(safe) > 500 {
		safe = safe[:400] + " (cut)"
	}

	return safe
}

func isHTML(header http.Header) bool {
	const textHTML = "text/html"
	ct := header.Get("Content-Type")
	return (len(ct) >= len(textHTML) && ct[:len(textHTML)] == textHTML)
}

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

// ExtractWords converts comma-separated values
// into a slice of unique words found in the dictionary.
//
// The search is case-insensitive and is based on common prefix:
// the input value "foo" selects the first word in
// the dictionary that starts with "foo" (as "food" for example).
//
// Moreover the special value "ALL" means all the dictionary words.
//
// No guarantees are made about ordering.
// However the returned words are not duplicated.
// Note this operation alters the content of the dictionary:
// the found words are replaced by the last dictionary words.
// Clone the input dictionary if it needs to be preserved:
//
//	d2 := append([]string{}, dictionary...)
//	words := garcon.ExtractWords(csv, d2)
func ExtractWords(csv string, dictionary []string) []string {
	prefixes := strings.Split(csv, ",")

	n := len(prefixes)
	if n > len(dictionary) {
		n = len(dictionary)
	}
	result := make([]string, 0, n)

	for _, p := range prefixes {
		p = strings.TrimSpace(p)
		p = strings.ToLower(p)

		switch p {
		case "":
			continue

		case "all":
			return append(dictionary, result...)

		default:
			for i, w := range dictionary {
				if len(p) <= len(w) && p == strings.ToLower(w[:len(p)]) {
					result = append(result, w)
					// make result unique => drop dictionary[i]
					dictionary = remove(dictionary, i)
					break
				}
			}
		}
	}

	return result
}

// remove alters the original slice.
//
// A one-line alternative but it also alters original slice:
//
//	slice = append(slice[:i], slice[i+1:]...)
//
// or:
//
//	import "golang.org/x/exp/slices"
//	slice = slices.Delete(slice, i, i+1)
func remove[T any](slice []T, i int) []T {
	slice[i] = slice[len(slice)-1] // copy last element at index #i
	return slice[:len(slice)-1]    // drop last element
}

// Deduplicate makes a slice of elements unique:
// it returns a slice with only the unique elements in it.
func Deduplicate[T comparable](duplicates []T) []T {
	uniques := make([]T, 0, len(duplicates))

	took := make(map[T]struct{}, len(duplicates))
	for _, v := range duplicates {
		if _, ok := took[v]; !ok {
			took[v] = struct{}{} // means "v has already been taken"
			uniques = append(uniques, v)
		}
	}

	return uniques
}

// EnvStr searches the environment variable (envvar)
// and returns its value if found,
// otherwise returns the optional fallback value.
// In absence of fallback, "" is returned.
func EnvStr(envvar string, fallback ...string) string {
	if value, ok := os.LookupEnv(envvar); ok {
		return value
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

// EnvInt does the same as EnvStr
// but expects the value is an integer.
// EnvInt panics if the envvar value cannot be parsed as an integer.
func EnvInt(envvar string, fallback ...int) int {
	if str, ok := os.LookupEnv(envvar); ok {
		integer, err := strconv.Atoi(str)
		if err != nil {
			log.Panicf("want integer but got %v=%q err: %v", envvar, str, err)
		}
		return integer
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}
