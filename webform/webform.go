// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

package webform

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/teal-finance/garcon/reserr"
	"github.com/teal-finance/garcon/security"
	"github.com/teal-finance/notifier"
	"github.com/teal-finance/notifier/logger"
	"github.com/teal-finance/notifier/mattermost"
)

type WebForm struct {
	ResErr   reserr.ResErr
	Notifier notifier.Notifier
	Redirect string

	// TextLimits are used as security limits
	// to avoid being flooded by large web forms
	// and unexpected field names.
	// The map key is the input field name.
	// The map value is a pair of integers:
	// the max length and the max line breaks.
	// Use -1 to disable any limit.
	TextLimits map[string][2]int

	// FileLimits is similar to TextLimits
	// but for uploaded files.
	// The map value is a pair of integers:
	// the max size in runes of one file
	// and the max occurrences having same field name.
	// Use -1 to disable any limit.
	FileLimits map[string][2]int

	maxNameLength int

	blankLines *regexp.Regexp
}

func NewContactForm(redirectURL, notifierURL string, resErr reserr.ResErr) WebForm {
	cf := WebForm{
		ResErr:     resErr,
		Redirect:   redirectURL,
		Notifier:   nil,
		TextLimits: DefaultContactSettings,
		FileLimits: DefaultFileSettings,
	}

	if notifierURL == "" {
		cf.Notifier = logger.NewNotifier()
	} else {
		cf.Notifier = mattermost.NewNotifier(notifierURL)
	}

	return cf
}

// DefaultContactSettings is compliant with standard names for web form input fields:
// https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#inappropriate-for-the-control
var DefaultContactSettings = map[string][2]int{
	"name":      {60, 1},
	"email":     {60, 1},
	"text":      {900, 20},
	"org-type":  {20, 1},
	"tel":       {30, 1},
	"want-call": {10, 1},
}

// DefaultFileSettings.
var DefaultFileSettings = map[string][2]int{
	"file": {1_000_000, 1}, // max: 1 file weighting 1 MB
}

// WebForm registers a web-form middleware
// that structures the filled form into markdown format
// and sends it to the Notifier.
func (wf *WebForm) WebForm() func(w http.ResponseWriter, r *http.Request) {
	if wf.Notifier == nil {
		log.Print("Middleware WebForm: no Notifier => use the logger Notifier")
		wf.Notifier = logger.NewNotifier()
	}

	if wf.TextLimits == nil {
		wf.TextLimits = DefaultContactSettings
		log.Print("Middleware WebForm: empty TextLimits => use ", wf.TextLimits)
	}

	if wf.FileLimits == nil {
		wf.FileLimits = DefaultFileSettings
		log.Print("Middleware WebForm: empty FileLimits => use ", wf.FileLimits)
	}

	wf.maxNameLength = -1
	for name := range wf.TextLimits {
		if wf.maxNameLength < len(name) {
			wf.maxNameLength = len(name)
		}
	}
	for name := range wf.FileLimits {
		if wf.maxNameLength < len(name) {
			wf.maxNameLength = len(name)
		}
	}

	wf.blankLines = regexp.MustCompile("\n\n+")

	log.Print("Middleware WebForm: empty FileLimits => use ", wf.FileLimits)
	return wf.webFormMD
}

// webForm structures the filled form into markdown format and sends it to the registered Notifier.
func (wf *WebForm) webFormMD(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Print("WRN WebForm ParseForm:", err)
		wf.ResErr.Write(w, r, http.StatusInternalServerError, "Cannot parse the webform")
		return
	}

	msg := ""

	for name, values := range r.Form {
		if !wf.valid(name) {
			continue
		}

		if len(values) != 1 {
			log.Printf("WRN WebForm: reject name=%s because "+
				"received %d input field(s) while expected only one",
				name, len(values))
			continue
		}

		max, ok := wf.TextLimits[name]
		if !ok {
			log.Printf("WRN WebForm: reject name=%s because "+
				"not an accepted name", name)
			continue
		}

		maxLen, maxBreaks := max[0], max[1]

		if len(values[0]) > maxLen {
			log.Printf("WRN WebForm: name=%s len=%d > max=%d", name, len(values[0]), maxLen)
			extra := len(values[0]) - maxLen
			if extra > 10 {
				values[0] = values[0][:maxLen] + "\n" + "(cut last " + strconv.Itoa(extra) + " characters)"
				maxBreaks++
			}
		}

		msg += "* " + name + ":  " + // use double space because may be a line break
			wf.markdown(values[0], maxBreaks) + "\n"
	}

	err = wf.Notifier.Notify(msg)
	if err != nil {
		log.Print("WRN WebForm Notify: ", err)
		wf.ResErr.Write(w, r, http.StatusInternalServerError, "Cannot store webform data")
		return
	}

	http.Redirect(w, r, wf.Redirect, http.StatusFound)
}

func (wf *WebForm) valid(name string) bool {
	if nLen := len(name); nLen > wf.maxNameLength {
		name = security.Sanitize(name)
		if len(name) > 100 {
			name = name[:90] + " (cut)"
		}
		log.Printf("WRN WebForm: reject name=%s because len=%d > max=%d",
			name, nLen, wf.maxNameLength)
		return false
	}

	if p := security.Printable(name); p >= 0 {
		log.Printf("WRN WebForm: reject name=%s because "+
			"contains a bad character at position %d",
			security.Sanitize(name), p)
		return false
	}

	if p := security.Printable(name); p >= 0 {
		log.Printf("WRN WebForm: reject name=%s because "+
			"contains a bad character at position %d",
			security.Sanitize(name), p)
		return false
	}

	if _, ok := wf.FileLimits[name]; ok {
		log.Printf("WRN WebForm: skip name=%s because "+
			"file not yet supported", name)
		return false
	}

	return true
}

func (wf *WebForm) markdown(v string, maxBreaks int) string {
	if !strings.ContainsAny(v, "\n\r") {
		return security.Sanitize(v)
	}

	v = strings.ReplaceAll(v, "\r", "")

	// avoid successive blank lines
	v = wf.blankLines.ReplaceAllString(v, "\n\n")

	txt := strings.Split(v, "\n")
	v = v[:0]
	for i, line := range txt {
		if i >= maxBreaks {
			v += fmt.Sprintf("\n  (too much line breaks %d > %d)", len(txt), maxBreaks)
			break
		}
		v += "\n" + "  " + // leading spaces = bullet indent
			security.Sanitize(line) +
			"  " // trailing double space = line break
	}

	return v
}
