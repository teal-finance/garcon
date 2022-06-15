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

type msg struct {
	Error string
	Doc   string
	Path  string
	Query string
}

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

// Write is a fast and pretty JSON marshaler.
func (resErr ResErr) Write(w http.ResponseWriter, r *http.Request, statusCode int, messages ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	b := make([]byte, 0, 1024)
	b = append(b, '{')

	comma := false
	if len(messages) > 0 {
		b = appendMessages(b, messages)
		comma = true
	}

	if r != nil {
		if comma {
			b = append(b, ',')
			b = append(b, '\n')
		}
		b = appendURL(b, r.URL)
		comma = true
	}

	if string(resErr) != "" {
		if comma {
			b = append(b, ',')
			b = append(b, '\n')
		}
		b = resErr.appendDoc(b)
	}

	b = append(b, '}')
	_, _ = w.Write(b)
}

func appendMessages(b []byte, messages []any) []byte {
	b = append(b, []byte(`"error":`)...)
	b = appendKey(b, messages[0])

	for i := 1; i < len(messages); i += 2 {
		b = append(b, ',')
		b = append(b, '\n')
		b = appendKey(b, messages[i])
		b = append(b, ':')
		if i+1 < len(messages) {
			b = appendValue(b, messages[i+1])
		} else {
			b = append(b, '0')
		}
	}

	return b
}

func appendURL(b []byte, u *url.URL) []byte {
	b = append(b, []byte(`"path":`)...)
	b = strconv.AppendQuote(b, u.Path)
	if u.RawQuery != "" {
		b = append(b, []byte(",\n"+`"query":`)...)
		b = strconv.AppendQuote(b, u.RawQuery)
	}
	return b
}

func (resErr ResErr) appendDoc(b []byte) []byte {
	b = append(b, []byte(`"doc":"`)...)
	b = append(b, []byte(string(resErr))...)
	b = append(b, '"')
	return b
}

func appendKey(b []byte, a any) []byte {
	switch v := a.(type) {
	case string:
		return strconv.AppendQuote(b, v)
	case []byte:
		return strconv.AppendQuote(b, string(v))
	default:
		return strconv.AppendQuote(b, fmt.Sprint(v))
	}
}

func appendValue(b []byte, a any) []byte {
	switch v := a.(type) {
	case bool:
		return strconv.AppendBool(b, v)
	case float32:
		return strconv.AppendFloat(b, float64(v), 'f', 9, 32)
	case float64:
		return strconv.AppendFloat(b, v, 'f', 9, 64)
	case int:
		return strconv.AppendInt(b, int64(v), 10)
	case int8:
		return strconv.AppendInt(b, int64(v), 10)
	case int16:
		return strconv.AppendInt(b, int64(v), 10)
	case int32:
		return strconv.AppendInt(b, int64(v), 10)
	case int64:
		return strconv.AppendInt(b, v, 10)
	case uint:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint8:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint16:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint32:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint64:
		return strconv.AppendUint(b, v, 10)
	case uintptr:
		return strconv.AppendUint(b, uint64(v), 10)
	case string:
		return strconv.AppendQuote(b, v)
	case []byte:
		return strconv.AppendQuote(b, string(v))
	default: // complex64 complex128
		return strconv.AppendQuote(b, fmt.Sprint(v))
	}
}
