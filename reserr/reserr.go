// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

package reserr

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

const (
	pathReserved = "Path is reserved for future use. Please contact us to share your ideas."
	pathInvalid  = "Path is not valid. Please refer to the documentation."
)

type ResErr string

// New is useless.
func New(docURL string) ResErr {
	return ResErr(docURL)
}

func (resErr ResErr) NotImplemented(w http.ResponseWriter, r *http.Request) {
	resErr.Write(w, r, http.StatusNotImplemented, pathReserved)
}

func (resErr ResErr) InvalidPath(w http.ResponseWriter, r *http.Request) {
	resErr.Write(w, r, http.StatusBadRequest, pathInvalid)
}

type msg struct {
	Error string
	Doc   string
	Path  string
	Query string
}

func (resErr ResErr) SafeWrite(w http.ResponseWriter, r *http.Request, statusCode int, text string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	m := msg{
		Error: text,
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
	if err != nil {
		log.Print("ResErr MarshalJSON ", m, " err: ", err)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		log.Print("ResErr Write ", m, " err: ", err)
	}
}

// Write is a faster and prettier implementation, but maybe unsafe.
func (resErr ResErr) Write(w http.ResponseWriter, r *http.Request, statusCode int, values ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	if len(values) == 0 {
		return
	}
	message, ok := values[0].(string)
	if !ok {
		return
	}

	b := make([]byte, 0, 300)
	b = append(b, []byte(`{"error":`)...)
	b = strconv.AppendQuote(b, message)

	if r != nil {
		b = append(b, []byte(",\n"+`"path":`)...)
		b = strconv.AppendQuote(b, r.URL.Path)
		if r.URL.RawQuery != "" {
			b = append(b, []byte(",\n"+`"query":`)...)
			b = strconv.AppendQuote(b, r.URL.RawQuery)
		}
	}

	if string(resErr) != "" {
		b = append(b, []byte(",\n"+`"doc":"`)...)
		b = append(b, []byte(string(resErr))...)
	}

	b = append(b, []byte(`"}`+"\n")...)
	_, _ = w.Write(b)
}

func Write(w http.ResponseWriter, r *http.Request, statusCode int, a ...any) {
	ResErr("").Write(w, r, statusCode, fmt.Sprint(a...))
}

func NotImplemented(w http.ResponseWriter, r *http.Request) {
	ResErr("").NotImplemented(w, r)
}

func InvalidPath(w http.ResponseWriter, r *http.Request) {
	ResErr("").InvalidPath(w, r)
}
