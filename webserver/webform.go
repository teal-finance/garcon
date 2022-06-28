// Copyright (c) 2021-2022 Teal.Finance contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server, under the MIT License.
// SPDX-License-Identifier: MIT

package webserver

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/teal-finance/garcon/security"
	"github.com/teal-finance/notifier/logger"
)

type FormSettings struct {
	// Multi is the number of form values having the same name.
	Multi int
	// Max is the length limit of a value (string).
	Max int
	// Attached indicates to attach the value as a file.
	Attached bool
}

// fallback provides the default security limits to avoid being flooded by large web forms.
var fallback = map[string]FormSettings{
	"name":    {Multi: 1, Max: 60, Attached: false},
	"email":   {Multi: 1, Max: 60, Attached: false},
	"message": {Multi: 1, Max: 900, Attached: false},
	"company": {Multi: 1, Max: 20, Attached: false},
	"tel":     {Multi: 1, Max: 30, Attached: false},
	"call":    {Multi: 1, Max: 10, Attached: false},
	"file":    {Multi: 1, Max: 999000, Attached: true},
	"total":   {Multi: 0, Max: 1000, Attached: true},
}

// DefaultFormLimits returns the default form settings: accepted names, length limits...
func DefaultFormLimits() map[string]FormSettings {
	return fallback
}

// WebForm sends the filled form to a notifier in markdown format.
func (ws *WebServer) WebForm() func(w http.ResponseWriter, r *http.Request) {
	if ws.Notifier == nil {
		log.Print("Middleware WebForm: no Notifier => Set dummy Notifier")
		ws.Notifier = logger.NewNotifier("dummy-notifier-url")
	}

	if len(ws.FormLimits) == 0 {
		log.Print("Middleware WebForm: empty FormLimits => Set fallback ", fallback)
		ws.FormLimits = fallback
	}

	if _, ok := ws.FormLimits["total"]; !ok {
		ws.FormLimits["total"] = fallback["total"]
		log.Print("Middleware WebForm: Missing 'total' => Set total.Max=", fallback["total"].Max)
	}

	re := regexp.MustCompile("\n\n+")
	ws.reduceLF = re

	return ws.webForm
}

// webForm sends the filled form to a notifier in markdown format.
func (ws *WebServer) webForm(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Print("WRN WebForm ParseForm:", err)
		ws.ResErr.Write(w, r, http.StatusInternalServerError, "Cannot parse the webform")
		return
	}

	msg := ""

	for name, values := range r.Form {
		limit, ok := ws.FormLimits[name]
		if !ok {
			log.Printf("WRN WebForm: forbidden name=%s #%d", name, len(values))
			continue
		}

		if limit.Attached {
			log.Printf("WRN WebForm: do not yet support attached name=%s #%d", name, len(values))
			continue
		}

		for i, v := range values {
			if i > limit.Multi {
				log.Printf("WRN WebForm: name=%s #%d exceed multi=%d", name, len(values), limit.Multi)
				continue
			}

			number := ""
			if len(values) > 1 {
				number = " #" + strconv.Itoa(i) + " "
			}
			msg += "* " + name + number + ":  "

			stop := false
			cut := len(v)
			msgMax := ws.FormLimits["total"].Max

			if len(v) > limit.Max {
				log.Printf("WRN WebForm: name=%s #%d len=%d > limit.Max=%d", name, i, len(v), limit.Max)
				cut = limit.Max - 10
				if len(msg)+cut > msgMax {
					cut = msgMax - len(msg)
					stop = true
				}
			} else if len(msg)+len(v) > msgMax {
				cut = msgMax - len(msg)
				stop = true
			}

			if cut < 10 {
				cut = 10
			}
			if len(v)-cut < 10 {
				cut = len(v)
			}

			str := v
			if cut < len(v) {
				str = v[:cut] + " (cut last " + strconv.Itoa(len(v)-cut) + " chars)"
			}

			if strings.ContainsAny(str, "\n") {
				str = strings.ReplaceAll(str, "\r", "")
				str = ws.reduceLF.ReplaceAllString(str, "\n\n")
				txt := strings.Split(str, "\n")
				for _, line := range txt {
					msg += "\n  " + security.Sanitize(line) + "  "
				}
			} else {
				msg += security.Sanitize(str)
			}

			if stop {
				msg += fmt.Sprintf("\n\n(len=%d max=%d, stop here)", len(msg), msgMax)
				break
			}

			msg += "\n"
		}
	}

	err = ws.Notifier.Notify(msg)
	if err != nil {
		log.Print("WRN WebForm Notify: ", err)
		ws.ResErr.Write(w, r, http.StatusInternalServerError, "Cannot store webform data")
		return
	}

	http.Redirect(w, r, ws.Redirect, http.StatusFound)
}
