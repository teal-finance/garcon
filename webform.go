// Copyright 2021 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package garcon

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/teal-finance/garcon/gg"
)

type WebForm struct {
	Writer   Writer
	Notifier gg.Notifier
	Redirect string

	// TextLimits are used as security limits
	// to avoid being flooded by large web forms
	// and unexpected field names.
	// The map key is the input field name.
	// The map value is a pair of integers:
	// the max length and the max line breaks.
	// Zero (or negative) value for unlimited value size.
	TextLimits map[string][2]int

	// FileLimits is similar to TextLimits
	// but for uploaded files.
	// The map value is a pair of integers:
	// the max size in runes of one file
	// and the max occurrences having same field name.
	// Zero (or negative) value for unlimited file size.
	FileLimits map[string][2]int

	// MaxBodyBytes limits someone hogging the host resources.
	// Zero (or negative) value disables this security check.
	MaxBodyBytes int64

	// MaxMDBytes includes the form fields and browser fingerprints.
	// Zero (or negative) value disables this security check.
	MaxMDBytes int

	maxFieldNameLength int
}

func (g *Garcon) NewContactForm(redirectURL string) WebForm {
	return NewContactForm(g.Writer, redirectURL)
}

// NewContactForm initializes a new WebForm with the default contact-form settings.
func NewContactForm(gw Writer, redirectURL string) WebForm {
	return WebForm{
		Writer:             gw,
		Notifier:           nil,
		Redirect:           redirectURL,
		TextLimits:         DefaultContactSettings(),
		FileLimits:         DefaultFileSettings(),
		MaxBodyBytes:       5555,
		MaxMDBytes:         4000,
		maxFieldNameLength: 0,
	}
}

// DefaultContactSettings is compliant with standard names for web form input fields:
// https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#inappropriate-for-the-control
func DefaultContactSettings() map[string][2]int {
	return map[string][2]int{
		"name":      {60, 1},
		"email":     {60, 1},
		"text":      {3000, 80},
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

func (wf *WebForm) init() {
	if wf.TextLimits == nil {
		wf.TextLimits = DefaultContactSettings()
		log.Info("Middleware WebForm: empty TextLimits => use", wf.TextLimits)
	}

	if wf.FileLimits == nil {
		wf.FileLimits = DefaultFileSettings()
		log.Info("Middleware WebForm: empty FileLimits => use", wf.FileLimits)
	}

	wf.maxFieldNameLength = 0
	for name := range wf.TextLimits {
		if wf.maxFieldNameLength < len(name) {
			wf.maxFieldNameLength = len(name)
		}
	}
	for name := range wf.FileLimits {
		if wf.maxFieldNameLength < len(name) {
			wf.maxFieldNameLength = len(name)
		}
	}

	log.Info("Middleware WebForm redirects to", wf.Redirect)
}

// Notify returns a handler that
// converts the received web-form into markdown format
// and sends it to the notifierURL.
func (wf *WebForm) Notify(notifierURL string) func(w http.ResponseWriter, r *http.Request) {
	wf.init()

	notifier := gg.NewNotifier(notifierURL)

	return func(w http.ResponseWriter, r *http.Request) {
		if wf.MaxBodyBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, wf.MaxBodyBytes)
		}

		err := r.ParseForm()
		if err != nil {
			log.Warning("WebForm ParseForm:", err)
			wf.Writer.WriteErr(w, r, http.StatusBadRequest, "cannot parse the webform", "reason", err.Error())
			return
		}

		md := wf.toMarkdown(r)
		err = notifier.Notify(md)
		if err != nil {
			log.Warning("WebForm Notify:", err)
		}

		http.Redirect(w, r, wf.Redirect, http.StatusFound)
	}
}

func (wf *WebForm) toMarkdown(r *http.Request) string {
	log.Infof("WebForm with %d input fields", len(r.Form))
	md := wf.formMD(r.Form) + FingerprintMD(r)
	if extra := overflow25(len(md), wf.MaxMDBytes); extra > 0 {
		md = md[:wf.MaxMDBytes] + "\n\n" +
			"(cut last " + strconv.Itoa(extra) + " characters)"
	}
	return md
}

func (wf *WebForm) formMD(fields url.Values) string {
	md := ""

	for name, values := range fields {
		if !wf.valid(name, values) {
			continue
		}

		max, ok := wf.TextLimits[name]
		maxLen, maxLines := max[0], max[1]
		if !ok {
			log.Warningf("WebForm: reject name=%s not in allowlist", name)
			continue
		}

		if extra := overflow25(len(values[0]), maxLen); extra > 0 {
			values[0] = values[0][:maxLen] + "\n" +
				"(cut last " + strconv.Itoa(extra) + " characters)"
			maxLines++
		}

		if md == "" { // no break line at first loop
			md += "- **"
		} else {
			md += "\n" + "- **" // double star -> bold
		}
		md += name + "**: " + wf.bulletParagraph(values[0], maxLines)
	}

	return md
}

func (wf *WebForm) valid(name string, values []string) bool {
	if len(values) == 1 && values[0] == "" {
		return false // skip empty values
	}

	if len(values) != 1 {
		log.Warningf("WebForm: reject name=%s because "+
			"received %d input field(s) while expected only one", name, len(values))
		return false
	}

	return wf.validName(name)
}

func (wf *WebForm) validName(name string) bool {
	nLen := len(name)
	if nLen > wf.maxFieldNameLength {
		name = gg.Sanitize(name)
		state := "(sanitized)"
		maxDisplay := 8 * wf.maxFieldNameLength
		if nLen > maxDisplay+10 {
			name = name[:maxDisplay]
			state = "(sanitized and cut)"
		}
		log.Warningf("WebForm: reject name=%q %s too long (%d > %d)", name, state, nLen, wf.maxFieldNameLength)
		return false
	}

	if p := gg.Printable(name); p >= 0 {
		log.Warningf("WebForm: reject name=%q contains a bad character at position %d", gg.Sanitize(name), p)
		return false
	}

	if _, ok := wf.FileLimits[name]; ok {
		log.Warningf("WebForm: skip name=%s because file not yet supported (TODO)", name)
		return false
	}

	return true
}

// overflow25 returns the overflow if n is 25% above max, else returns zero.
// max=0 means max is infinite.
func overflow25(n, max int) int {
	if (n > max+max/4) && (max > 0) {
		return n - max
	}
	return 0
}

// Markdown encoding.
const (
	lineBreak    = "  " // trailing double space -> line break
	bulletIndent = " "  // leading spaces -> bullet indent
)

func (wf *WebForm) bulletParagraph(str string, maxLines int) string {
	md := ""

	count := 0
	blank := false
	txt := gg.SplitCleanedLines(str)
	for i := range txt {
		// skip top blank lines, redundant blank lines and bottom blank lines
		if txt[i] == "" {
			if md != "" {
				blank = true
			}
			continue
		}

		if blank {
			blank = false
			md += lineBreak + "\n\n" + bulletIndent
		} else if md != "" {
			md += lineBreak + "\n" + bulletIndent
		}
		md += txt[i]
		count++

		remaining := len(txt) - i
		if (count > maxLines) && (maxLines > 0) && (remaining > maxLines/2) {
			md += fmt.Sprintf("\n  (skip %d lines)", remaining)
			break
		}
	}

	return md
}
