package security

import (
	"crypto/rand"
	"encoding/base64"
	"hash"
	"log"
	"net/http"
	"strings"

	"github.com/minio/highwayhash"
	"github.com/teal-finance/garcon/reserr"
)

const hashKey32bytes = "0123456789ABCDEF0123456789ABCDEF"

// DoesContainLineBreak returns true if input string
// contains a Carriage Return "\r" or a Line Feed "\n"
// to prevent log injection.
func DoesContainLineBreak(s string) bool {
	return strings.Contains(s, "\r") || strings.Contains(s, "\n")
}

// SanitizeLineBreaks replaces
// Carriage Return "\r" by <CR> and
// Line Feed "\n" by <LF>.
func SanitizeLineBreaks(s string) string {
	s = strings.ReplaceAll(s, "\r", "<CR>")
	s = strings.ReplaceAll(s, "\n", "<LF>")

	return s
}

// RejectLineBreakInURI rejects HTTP requests having
// a Carriage Return "\r" or a Line Feed "\n"
// within the URI to prevent log injection.
func RejectLineBreakInURI(next http.Handler) http.Handler {
	log.Print("Middleware security: RejectLineBreakInURI")

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if DoesContainLineBreak(r.RequestURI) {
				reserr.Write(w, r, http.StatusBadRequest, "Invalid URI containing a line break (CR or LF)")
				log.Print("WRN WebServer: reject URI with <CR> or <LF>:", SanitizeLineBreaks(r.RequestURI))

				return
			}

			next.ServeHTTP(w, r)
		})
}

// ValidPath replies a HTTP error on invalid path to prevent path traversal attacks.
func ValidPath(w http.ResponseWriter, r *http.Request) bool {
	if strings.Contains(r.URL.Path, "..") {
		reserr.Write(w, r, http.StatusBadRequest, "Invalid URL Path Containing '..'")
		log.Print("WRN WebServer: reject path with '..' ", SanitizeLineBreaks(r.URL.Path))

		return false
	}

	return true
}

type Hash struct {
	h hash.Hash
}

func NewHash() (Hash, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return Hash{nil}, err
	}

	h, err := highwayhash.New(key)

	return Hash{h}, err
}

// Obfuscate hashes the input string to prevent logging sensitive information.
// HighwayHash is a hashing algorithm enabling high speed (especially on AMD64).
func (h Hash) Obfuscate(s string) (string, error) {
	h.h.Reset()
	checksum := h.h.Sum([]byte(s))

	return base64.StdEncoding.EncodeToString(checksum), nil
}

// Obfuscate hashes the input string to prevent logging sensitive information.
// HighwayHash is a hashing algorithm enabling high speed (especially on AMD64).
func Obfuscate(s string) (string, error) {
	hash, err := highwayhash.New([]byte(hashKey32bytes))
	if err != nil {
		return s, err
	}

	checksum := hash.Sum([]byte(s))

	return base64.StdEncoding.EncodeToString(checksum), nil
}
