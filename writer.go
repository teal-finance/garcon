// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

const (
	pathReserved = "Path is reserved for future use. Please contact us to share your ideas."
	pathInvalid  = "Path is not valid. Please refer to the documentation."
)

// Writer enables writing useful JSON error message in the HTTP response body.
type Writer string

// NewWriter creates a Writer structure.
func NewWriter(docURL string) Writer {
	return Writer(docURL)
}

func NotImplemented(w http.ResponseWriter, r *http.Request) {
	Writer("").NotImplemented(w, r)
}

func InvalidPath(w http.ResponseWriter, r *http.Request) {
	Writer("").InvalidPath(w, r)
}

func (gw Writer) NotImplemented(w http.ResponseWriter, r *http.Request) {
	gw.WriteErr(w, r, http.StatusNotImplemented, pathReserved)
}

func (gw Writer) InvalidPath(w http.ResponseWriter, r *http.Request) {
	gw.WriteErr(w, r, http.StatusBadRequest, pathInvalid)
}

func WriteErr(w http.ResponseWriter, r *http.Request, statusCode int, a ...any) {
	Writer("").WriteErr(w, r, statusCode, a...)
}

func WriteErrSafe(w http.ResponseWriter, r *http.Request, statusCode int, a ...any) {
	Writer("").WriteErrSafe(w, r, statusCode, a...)
}

func WriteOK(w http.ResponseWriter, a ...any) {
	Writer("").WriteOK(w, a...)
}

// msg is only used by SafeWrite to generate a fast JSON marshaler.
type msg struct {
	Message string // "message" is more common than "error"
	Doc     string
	Path    string
	Query   string
}

// WriteErrSafe is a safe alternative to Write, may be slower despite the easyjson generated code.
// Disadvantage: WriteErrSafe concatenates all key-values (kv) in "message" field.
func (gw Writer) WriteErrSafe(w http.ResponseWriter, r *http.Request, statusCode int, kv ...any) {
	response := msg{
		Message: fmt.Sprint(kv...),
		Doc:     string(gw),
		Path:    "",
		Query:   "",
	}

	if r != nil {
		response.Path = r.URL.Path
		if r.URL.RawQuery != "" {
			response.Query = r.URL.RawQuery
		}
	}

	buf, err := response.MarshalJSON()
	if err != nil {
		log.Print("WRN WriteSafeJSONErr: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf)
}

// WriteErr is a fast pretty-JSON marshaler dedicated to the HTTP error response.
// WriteErr extends the JSON content when more than two key-values (kv) are provided.
func (gw Writer) WriteErr(w http.ResponseWriter, r *http.Request, statusCode int, kv ...any) {
	buf := make([]byte, 0, 1024)
	buf = append(buf, '{')

	buf, comma := appendMessages(buf, kv)

	if r != nil {
		if comma {
			buf = append(buf, ',', '\n')
		}
		buf = appendURL(buf, r.URL)
		comma = true
	}

	if string(gw) != "" {
		if comma {
			buf = append(buf, ',', '\n')
		}
		buf = gw.appendDoc(buf)
	}

	buf = append(buf, '}')

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf)
}

// WriteOK is a fast pretty-JSON marshaler dedicated to the HTTP successful response.
func (gw Writer) WriteOK(w http.ResponseWriter, kv ...any) {
	var buf []byte
	var err error

	switch {
	case len(kv) == 0:
		buf = []byte("{}")

	case len(kv) == 1:
		buf, err = json.Marshal(kv[0])
		if err != nil {
			gw.WriteErr(w, nil, http.StatusInternalServerError,
				"Cannot serialize success JSON response", "error", err)
			return
		}

	default:
		buf = make([]byte, 0, 1024) // 1024 = max bytes of most of the JSON responses
		buf = append(buf, '{')
		buf = appendKeyValues(buf, false, kv)
		buf = append(buf, '}')
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf)
}

func appendMessages(buf []byte, kv []any) (_ []byte, comma bool) {
	if len(kv) == 0 {
		return buf, false
	}

	// {"message":"xxxxx"} is more common than {"error":"xxxxx"}
	buf = append(buf, []byte(`"message":`)...)

	if len(kv) == 2 {
		s := fmt.Sprintf("%v%v", kv[0], kv[1])
		buf = strconv.AppendQuoteToGraphic(buf, s)
		return buf, true
	}

	buf = appendValue(buf, kv[0])

	if len(kv) > 1 {
		buf = appendKeyValues(buf, true, kv[1:])
	}

	return buf, true
}

func appendKeyValues(buf []byte, comma bool, kv []any) []byte {
	if (len(kv) == 0) || (len(kv)%2 != 0) {
		log.Panic("Writer: want non-zero even len(kv) but got ", len(kv))
	}

	for i := 0; i < len(kv); i += 2 {
		if comma {
			buf = append(buf, ',', '\n')
		} else {
			comma = true
		}
		buf = appendKey(buf, kv[i])
		buf = append(buf, ':')
		buf = appendValue(buf, kv[i+1])
	}

	return buf
}

//nolint:cyclop,gocyclo // cannot reduce cyclomatic complexity
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
	case string:
		return strconv.AppendQuoteToGraphic(buf, val)
	case []byte:
		return strconv.AppendQuoteToGraphic(buf, string(val))
	case complex64, complex128:
		return strconv.AppendQuoteToGraphic(buf, fmt.Sprint(val))
	case error:
		return strconv.AppendQuoteToGraphic(buf, val.Error())
	default:
		return appendJSON(buf, val)
	}
}

func appendJSON(buf []byte, a any) []byte {
	b, err := json.Marshal(a)
	if err != nil {
		log.Printf("ERR Writer jsonify %+v %v", a, err)
	}
	return append(buf, b...)
}

func appendKey(buf []byte, a any) []byte {
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

func (gw Writer) appendDoc(buf []byte) []byte {
	buf = append(buf, '"', 'd', 'o', 'c', '"', ':', '"')
	buf = append(buf, []byte(string(gw))...)
	buf = append(buf, '"')
	return buf
}
