// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

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

	// MaxTotalMarkdownLength includes the
	// form fields and browser fingerprints.
	MaxTotalMarkdownLength int

	maxFieldNameLength int

	blankLines *regexp.Regexp
}

func NewContactForm(redirectURL, notifierURL string, resErr reserr.ResErr) WebForm {
	form := WebForm{
		ResErr:     resErr,
		Redirect:   redirectURL,
		Notifier:   nil,
		TextLimits: DefaultContactSettings,
		FileLimits: DefaultFileSettings,
	}

	if notifierURL == "" {
		form.Notifier = logger.NewNotifier()
	} else {
		form.Notifier = mattermost.NewNotifier(notifierURL)
	}

	form.MaxTotalMarkdownLength = 2000

	return form
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

// NotifyWebForm registers a web-form middleware
// that structures the filled form into markdown format
// and sends it to the Notifier.
func (form *WebForm) NotifyWebForm() func(w http.ResponseWriter, r *http.Request) {
	if form.Notifier == nil {
		log.Print("Middleware WebForm: no Notifier => use the logger Notifier")
		form.Notifier = logger.NewNotifier()
	}

	if form.TextLimits == nil {
		form.TextLimits = DefaultContactSettings
		log.Print("Middleware WebForm: empty TextLimits => use ", form.TextLimits)
	}

	if form.FileLimits == nil {
		form.FileLimits = DefaultFileSettings
		log.Print("Middleware WebForm: empty FileLimits => use ", form.FileLimits)
	}

	form.maxFieldNameLength = -1
	for name := range form.TextLimits {
		if form.maxFieldNameLength < len(name) {
			form.maxFieldNameLength = len(name)
		}
	}
	for name := range form.FileLimits {
		if form.maxFieldNameLength < len(name) {
			form.maxFieldNameLength = len(name)
		}
	}

	form.blankLines = regexp.MustCompile("\n\n+")

	log.Print("Middleware WebForm: empty FileLimits => use ", form.FileLimits)
	return form.notify
}

// notify converts the filled form into markdown format and sends it to the registered Notifier.
func (form *WebForm) notify(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Print("WRN WebForm ParseForm:", err)
		form.ResErr.Write(w, r, http.StatusInternalServerError, "Cannot parse the webform")
		return
	}

	md := form.messageMD(r) + FingerprintMD(r)

	if len(md) > form.MaxTotalMarkdownLength {
		md = md[:form.MaxTotalMarkdownLength] +
			"\n\n(cut len=" + strconv.Itoa(len(md)) +
			" > max=" + strconv.Itoa(form.MaxTotalMarkdownLength) + ")"
	}

	err = form.Notifier.Notify(md)
	if err != nil {
		log.Print("WRN WebForm Notify: ", err)
		form.ResErr.Write(w, r, http.StatusInternalServerError, "Cannot store webform data")
		return
	}

	http.Redirect(w, r, form.Redirect, http.StatusFound)
}

func (form *WebForm) messageMD(r *http.Request) string {
	md := ""

	for name, values := range r.Form {
		if !form.valid(name) {
			continue
		}

		if len(values) != 1 {
			log.Printf("WRN WebForm: reject name=%s because "+
				"received %d input field(s) while expected only one",
				name, len(values))
			continue
		}

		max, ok := form.TextLimits[name]
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

		if len(md) > 1 {
			md += "\n"
		}

		md += "* " + name + ": " + form.valueMD(values[0], maxBreaks)
	}

	return md
}

func (form *WebForm) valid(name string) bool {
	if nLen := len(name); nLen > form.maxFieldNameLength {
		name = security.Sanitize(name)
		if len(name) > 100 {
			name = name[:90] + " (cut)"
		}
		log.Printf("WRN WebForm: reject name=%s because len=%d > max=%d",
			name, nLen, form.maxFieldNameLength)
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

	if _, ok := form.FileLimits[name]; ok {
		log.Printf("WRN WebForm: skip name=%s because "+
			"file not yet supported", name)
		return false
	}

	return true
}

func (form *WebForm) valueMD(v string, maxBreaks int) string {
	if !strings.ContainsAny(v, "\n\r") {
		return security.Sanitize(v)
	}

	v = strings.ReplaceAll(v, "\r", "")

	// avoid successive blank lines
	v = form.blankLines.ReplaceAllString(v, "\n\n")

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
