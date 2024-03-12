// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

// Package gg is the Garcon toolbox.
package gg

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/go-chi/chi/v5"

	"github.com/teal-finance/emo"
)

var log = emo.NewZone("garcon")

// OverwriteBufferContent is to erase a secret when it is no longer required.
func OverwriteBufferContent(b []byte) {
	//nolint:gosec // does not matter if written bytes are not good random values
	// "math/rand" is 40 times faster than "crypto/rand"
	// see: https://github.com/SimonWaldherr/golang-benchmarks#random
	rand.Read(b)
}

// SplitClean splits the string into sanitized tokens.
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

func KeepSchemeHostOnly(urls []*url.URL) []string {
	sh := make([]string, 0, len(urls))
	for _, u := range urls {
		o := u.Scheme + "://" + u.Host
		sh = append(sh, o)
	}
	return sh
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
		safe = safe[:400] + " (trimmed last " + strconv.Itoa(len(safe)-400) + " bytes)"
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

// ConvertSize64 is ConvertSize with int64 input.
// A good alternative is `ByteSize()“ from "github.com/gofiber/fiber".
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
		if str != "" {
			integer, err := strconv.Atoi(str)
			if err != nil {
				log.Panicf("want integer but got %v=%q err: %v", envvar, str, err)
			}
			return integer
		}
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}

func EncodeHexOrB64(in string, isHex bool) string {
	return string(EncodeHexOrB64Bytes([]byte(in), isHex))
}

func EncodeHexOrB64Bytes(bin []byte, isHex bool) []byte {
	var txt []byte
	if isHex {
		txt = make([]byte, hex.EncodedLen(len(bin)))
		hex.Encode(txt, bin)
	} else {
		txt = make([]byte, base64.RawURLEncoding.EncodedLen(len(bin)))
		base64.RawURLEncoding.Encode(txt, bin)
	}
	return txt
}

// DecodeHexOrB64 tries to decode the input string as hexadecimal or Base64
// depending on the given output length.
// DecodeHexOrB64 supports the unpadded Base64 as defined in RFC 4648 §5 (URL encoding).
func DecodeHexOrB64(in string, outLen int) ([]byte, error) {
	return DecodeHexOrB64Bytes([]byte(in), outLen, true)
}

// DecodeHexOrB64Bytes tries to decode the input bytes as hexadecimal or Base64
// depending on the given output length.
// DecodeHexOrB64Bytes supports the unpadded Base64 defined in RFC 4648 §5 for URL encoding.
// The "reuse" parameter allows to reuse the input bytes reducing the memory allocation.
// Caution: the input bytes are overwritten with reuse=true.
func DecodeHexOrB64Bytes(in []byte, outLen int, reuse bool) ([]byte, error) {
	inLen := len(in)
	hexLen := hex.EncodedLen(outLen)
	b64Len := base64.RawURLEncoding.EncodedLen(outLen)

	switch inLen {
	case hexLen, b64Len: // OK
	default:
		return nil, &sizeError{inLen, hexLen, b64Len}
	}

	var out []byte
	if reuse {
		out = in
	} else {
		out = make([]byte, outLen)
	}

	n, err := decodeHexOrB64Bytes(out, in, inLen == hexLen)
	if err != nil {
		log.Warn(err)
		return nil, &decodeError{err, inLen, inLen == hexLen}
	}
	if n != outLen {
		log.Panic("input=", inLen, "want=", outLen, "got=", len(out))
	}

	return out[:n], nil
}

func decodeHexOrB64Bytes(dst, src []byte, isHex bool) (int, error) {
	if isHex {
		return hex.Decode(dst, src)
	}
	return base64.RawURLEncoding.Decode(dst, src)
}

type sizeError struct {
	inLen  int
	hexLen int
	b64Len int
}

func (e *sizeError) Error() string {
	return fmt.Sprintf("got %d bytes but want %d hexadecimal digits or %d unpadded Base64 characters (RFC 4648 §5)", e.inLen, e.hexLen, e.b64Len)
}

type decodeError struct {
	err   error
	inLen int
	isHex bool
}

func (e *decodeError) Error() string {
	base := "Base64"
	if e.isHex {
		base = "Hexadecimal"
	}
	return fmt.Sprintf("cannot decode the %d bytes as %s: %s", e.inLen, base, e.err.Error())
}

func (e *decodeError) Unwrap() error {
	return e.err
}

// B2S (Bytes to String) returns a string pointing to a []byte without copying.
func B2S(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// Namify extracts the wider "[a-zA-Z0-9_]+" string from the end of str.
// If str is a path or an URL, keep the last basename.
// Example: keep "myapp" from "https://example.com/path/myapp/"
// Namify also removes all punctuation characters except "_" and "-".
func Namify(str string) string {
	str = strings.Trim(str, "/.")

	// keep last directory name (basename)
	if i := strings.LastIndex(str, "/"); i >= 0 {
		str = str[i+1:]
	}

	// remove file or domain extension (if any)
	if i := strings.LastIndex(str, "."); i > 0 {
		str = str[:i]
	}

	// use dash between sub domain and main TLS
	str = strings.ReplaceAll(str, ".", "-")

	// keep alphanumeric characters only
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	str = re.ReplaceAllLiteralString(str, "")

	return strings.Trim(str, "_-")
}
