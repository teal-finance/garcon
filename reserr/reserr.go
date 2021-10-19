// Teal.Finance/Server is an opinionated boilerplate API and website server.
// Copyright (C) 2021 Teal.Finance contributors
//
// This file is part of Teal.Finance/Server, licensed under LGPL-3.0-or-later.
//
// Teal.Finance/Server is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 of the License, or (at your option) any later version.
//
// Teal.Finance/Server is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.

package reserr

import (
	"log"
	"net/http"
)

type ResErr string

// New is useless.
func New(docURL string) ResErr {
	return ResErr(docURL)
}

func (resErr ResErr) NotImplemented(w http.ResponseWriter, r *http.Request) {
	resErr.Write(w, r, http.StatusNotImplemented, "Path is reserved for future use. Please contact us to share your ideas.")
}

func (resErr ResErr) InvalidPath(w http.ResponseWriter, r *http.Request) {
	resErr.Write(w, r, http.StatusBadRequest, "Path is not valid. Please refer to the API doc.")
}

type msg struct {
	Error string
	Doc   string
	Path  string
	Query string
}

func (resErr ResErr) Write(w http.ResponseWriter, r *http.Request, statusCode int, text string) {
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
		log.Printf("ResErr MarshalJSON %v err: %v", m, err)

		return
	}

	_, err = w.Write(b)
	if err != nil {
		log.Printf("ResErr Write %v err: %v", m, err)
	}
}

func Write(w http.ResponseWriter, r *http.Request, statusCode int, text string) {
	ResErr("").Write(w, r, statusCode, text)
}
