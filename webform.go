// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/teal-finance/notifier"
	"github.com/teal-finance/notifier/logger"
	"github.com/teal-finance/notifier/mattermost"
)

type WebForm struct {
	Writer   Writer
	Notifier notifier.Notifier
	Redirect string

	// TextLimits are used as security limits
	// to avoid being flooded by large web forms
	// and unexpected field names.
	// The map key is the input field name.
	// The map value is a pair of integers:
	// the max length and the max line breaks.
	// Use 0 to disable any limit.
	TextLimits map[string][2]int

	// FileLimits is similar to TextLimits
	// but for uploaded files.
	// The map value is a pair of integers:
	// the max size in runes of one file
	// and the max occurrences having same field name.
	// Use 0 to disable any limit.
	FileLimits map[string][2]int

	// MaxTotalLength includes the
	// form fields and browser fingerprints.
	MaxTotalLength int

	maxFieldNameLength int
}

func (g *Garcon) NewContactForm(redirectURL string) WebForm {
	return NewContactForm(g.Writer, redirectURL)
}

// NewContactForm initializes a new WebForm with the default contact-form settings.
func NewContactForm(gw Writer, redirectURL string) WebForm {
	form := WebForm{
		Writer:             gw,
		Notifier:           nil,
		Redirect:           redirectURL,
		TextLimits:         DefaultContactSettings(),
		FileLimits:         DefaultFileSettings(),
		MaxTotalLength:     0,
		maxFieldNameLength: 0,
	}

	form.MaxTotalLength = 2000

	return form
}

// DefaultContactSettings is compliant with standard names for web form input fields:
// https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#inappropriate-for-the-control
func DefaultContactSettings() map[string][2]int {
	return map[string][2]int{
		"name":      {60, 1},
		"email":     {60, 1},
		"text":      {900, 20},
		"org-type":  {20, 1},
		"tel":       {30, 1},
		"want-call": {10, 1},
	}
}

// DefaultFileSettings sets FileLimits with only "file".
func DefaultFileSettings() map[string][2]int {
	return map[string][2]int{
		"file": {1_000_000, 1}, // max: 1 file weighting 1 MB
	}
}

func (form *WebForm) init() {
	if form.TextLimits == nil {
		form.TextLimits = DefaultContactSettings()
		log.Print("INF Middleware WebForm: empty TextLimits => use ", form.TextLimits)
	}

	if form.FileLimits == nil {
		form.FileLimits = DefaultFileSettings()
		log.Print("INF Middleware WebForm: empty FileLimits => use ", form.FileLimits)
	}

	form.maxFieldNameLength = 0
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

	log.Print("INF Middleware WebForm redirect=", form.Redirect)
}

// Notify registers a web-form middleware
// that structures the filled form into markdown format
// and sends it to the Notifier.
// notify converts the received web-form into markdown format
// and sends it to the registered Notifier.
func (form *WebForm) Notify(notifierURL string) func(w http.ResponseWriter, r *http.Request) {
	form.init()

	var n notifier.Notifier
	if notifierURL == "" {
		log.Print("INF Middleware WebForm: no Notifier => use the logger Notifier")
		n = logger.NewNotifier()
	} else {
		n = mattermost.NewNotifier(notifierURL)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			log.Print("WRN WebForm ParseForm: ", err)
			form.Writer.WriteErr(w, r, http.StatusInternalServerError, "Cannot parse the webform")
			return
		}

		md := form.toMarkdown(r)
		err = n.Notify(md)
		if err != nil {
			log.Print("WRN WebForm Notify: ", err)
			form.Writer.WriteErr(w, r, http.StatusInternalServerError, "Cannot store webform data")
			return
		}

		http.Redirect(w, r, form.Redirect, http.StatusFound)
	}
}

func (form *WebForm) toMarkdown(r *http.Request) string {
	log.Printf("WebForm with %d input fields", len(r.Form))

	md := ""
	for name, values := range r.Form {
		if !form.valid(name, values) {
			continue
		}

		max, ok := form.TextLimits[name]
		if !ok {
			log.Printf("WRN WebForm: reject name=%s because "+
				"not an accepted name", name)
			continue
		}

		maxLen, maxBreaks := max[0], max[1]

		if len(values[0]) > maxLen && maxLen > 0 {
			extra := len(values[0]) - maxLen
			if extra > 30 {
				values[0] = values[0][:maxLen] +
					"\n" + "(cut last " + strconv.Itoa(extra) + " characters)"
				maxBreaks++
			}
		}

		if md != "" { // no break line at first loop
			md += "\n"
		}

		md += "* " + name + ": " + form.valueMD(values[0], maxBreaks)
	}

	md += FingerprintMD(r)

	if len(md) > form.MaxTotalLength && form.MaxTotalLength > 0 {
		md = md[:form.MaxTotalLength] +
			"\n\n(cut because len=" + strconv.Itoa(len(md)) +
			" > max=" + strconv.Itoa(form.MaxTotalLength) + ")"
	}

	return md
}

func (form *WebForm) valid(name string, values []string) bool {
	if len(values) == 1 && values[0] == "" {
		return false // skip empty values
	}

	if len(values) != 1 {
		log.Printf("WRN WebForm: reject name=%s because "+
			"received %d input field(s) while expected only one", name, len(values))
		return false
	}

	return form.validName(name)
}

func (form *WebForm) validName(name string) bool {
	if nLen := len(name); nLen > form.maxFieldNameLength {
		name = Sanitize(name)
		if len(name) > 100 {
			name = name[:90] + " (cut)"
		}
		log.Printf("WRN WebForm: reject name=%s because len=%d > max=%d",
			name, nLen, form.maxFieldNameLength)
		return false
	}

	if p := printable(name); p >= 0 {
		log.Printf("WRN WebForm: reject name=%s because "+
			"contains a bad character at position %d",
			Sanitize(name), p)
		return false
	}

	if p := printable(name); p >= 0 {
		log.Printf("WRN WebForm: reject name=%s because "+
			"contains a bad character at position %d",
			Sanitize(name), p)
		return false
	}

	if _, ok := form.FileLimits[name]; ok {
		log.Printf("WRN WebForm: skip name=%s because "+
			"file not yet supported", name)
		return false
	}

	return true
}

func (form *WebForm) valueMD(str string, maxBreaks int) string {
	if !strings.ContainsAny(str, "\n\r") {
		return Sanitize(str)
	}

	str = strings.ReplaceAll(str, "\r", "")
	txt := strings.Split(str, "\n")

	md := ""
	previous := ""
	breaks := 0
	for _, line := range txt {
		line = Sanitize(line)
		if line == "" && previous == "" {
			// no blank lines in the beginning and no successive blank lines
			continue
		}

		if breaks >= maxBreaks && maxBreaks > 0 {
			md += fmt.Sprintf("\n  (too much line breaks %d > %d)", breaks, maxBreaks)
			break
		}

		if md != "" {
			md += "\n" + "  " // leading spaces = bullet indent
		}
		md += line + "  " // trailing double space = line break

		previous = line
		breaks++
	}

	return md
}
