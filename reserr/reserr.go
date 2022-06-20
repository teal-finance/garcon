// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

// Package reserr writes useful JSON message on HTTP error.
package reserr

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	pathReserved = "Path is reserved for future use. Please contact us to share your ideas."
	pathInvalid  = "Path is not valid. Please refer to the documentation."
)

type ResErr string

// New creates a ResErr structure.
func New(docURL string) ResErr {
	return ResErr(docURL)
}

func NotImplemented(w http.ResponseWriter, r *http.Request) {
	ResErr("").NotImplemented(w, r)
}

func InvalidPath(w http.ResponseWriter, r *http.Request) {
	ResErr("").InvalidPath(w, r)
}

func (resErr ResErr) NotImplemented(w http.ResponseWriter, r *http.Request) {
	resErr.Write(w, r, http.StatusNotImplemented, pathReserved)
}

func (resErr ResErr) InvalidPath(w http.ResponseWriter, r *http.Request) {
	resErr.Write(w, r, http.StatusBadRequest, pathInvalid)
}

func Write(w http.ResponseWriter, r *http.Request, statusCode int, a ...any) {
	ResErr("").Write(w, r, statusCode, a)
}

// msg is only used by SafeWrite to generate a fast JSON marshaler.
type msg struct {
	Error string
	Doc   string
	Path  string
	Query string
}

// SafeWrite is a safe alternative to Write, may be slower despite the easyjson generated code.
// SafeWrite concatenates all messages in "error" field.
func (resErr ResErr) SafeWrite(w http.ResponseWriter, r *http.Request, statusCode int, messages ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	m := msg{
		Error: fmt.Sprint(messages...),
		Doc:   string(resErr),
		Path:  "",
		Query: "",
	}

	if r != nil {
		m.Path = r.URL.Path
		if r.URL.RawQuery != "" {
			m.Query = r.URL.RawQuery
		}
	}

	b, err := m.MarshalJSON()
	if err == nil {
		_, _ = w.Write(b)
	}
}

// Write is a fast pretty-JSON marshaler dedicated to the HTTP error response.
// Write extends the JSON content when more than two messages are provided.
func (resErr ResErr) Write(w http.ResponseWriter, r *http.Request, statusCode int, messages ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	buf := make([]byte, 0, 1024)
	buf = append(buf, '{')

	buf, comma := appendMessages(buf, messages)

	if r != nil {
		if comma {
			buf = append(buf, ',', '\n')
		}
		buf = appendURL(buf, r.URL)
		comma = true
	}

	if string(resErr) != "" {
		if comma {
			buf = append(buf, ',', '\n')
		}
		buf = resErr.appendDoc(buf)
	}

	buf = append(buf, '}')
	_, _ = w.Write(buf)
}

func appendMessages(buf []byte, messages []any) ([]byte, bool) {
	if len(messages) == 0 {
		return buf, false
	}

	buf = append(buf, []byte(`"error":`)...)

	if len(messages) == 2 {
		s := fmt.Sprintf("%v%v", messages[0], messages[1])
		buf = strconv.AppendQuoteToGraphic(buf, s)
		return buf, true
	}

	buf = appendQuote(buf, messages[0])
	for i := 1; i < len(messages); i += 2 {
		buf = append(buf, ',', '\n')
		buf = appendQuote(buf, messages[i])
		buf = append(buf, ':')
		if i+1 < len(messages) {
			buf = appendValue(buf, messages[i+1])
		} else {
			buf = append(buf, '0')
		}
	}

	return buf, true
}

func appendValue(buf []byte, a any) []byte {
	switch val := a.(type) {
	case bool:
		return strconv.AppendBool(buf, val)
	case float32:
		return strconv.AppendFloat(buf, float64(val), 'f', 9, 32)
	case float64:
		return strconv.AppendFloat(buf, val, 'f', 9, 64)
	case int:
		return strconv.AppendInt(buf, int64(val), 10)
	case int8:
		return strconv.AppendInt(buf, int64(val), 10)
	case int16:
		return strconv.AppendInt(buf, int64(val), 10)
	case int32:
		return strconv.AppendInt(buf, int64(val), 10)
	case int64:
		return strconv.AppendInt(buf, val, 10)
	case uint:
		return strconv.AppendUint(buf, uint64(val), 10)
	case uint8:
		return strconv.AppendUint(buf, uint64(val), 10)
	case uint16:
		return strconv.AppendUint(buf, uint64(val), 10)
	case uint32:
		return strconv.AppendUint(buf, uint64(val), 10)
	case uint64:
		return strconv.AppendUint(buf, val, 10)
	case uintptr:
		return strconv.AppendUint(buf, uint64(val), 10)
	default: // string []byte complex64 complex128
		return appendQuote(buf, val)
	}
}

func appendQuote(buf []byte, a any) []byte {
	switch val := a.(type) {
	case string:
		return strconv.AppendQuoteToGraphic(buf, val)
	case []byte:
		return strconv.AppendQuoteToGraphic(buf, string(val))
	default:
		return strconv.AppendQuoteToGraphic(buf, fmt.Sprint(val))
	}
}

func appendURL(buf []byte, u *url.URL) []byte {
	buf = append(buf, []byte(`"path":`)...)
	buf = strconv.AppendQuote(buf, u.Path)
	if u.RawQuery != "" {
		buf = append(buf, []byte(",\n"+`"query":`)...)
		buf = strconv.AppendQuote(buf, u.RawQuery)
	}
	return buf
}

func (resErr ResErr) appendDoc(buf []byte) []byte {
	buf = append(buf, '"', 'd', 'o', 'c', '"', ':', '"')
	buf = append(buf, []byte(string(resErr))...)
	buf = append(buf, '"')
	return buf
}
