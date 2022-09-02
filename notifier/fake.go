// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package notifier

import (
	"strings"
	"unicode/utf8"
)

// FakeNotifier implements a Notifier interface that logs the received notifications.
// FakeNotifier can be used as a mocked Notifier or for debugging purpose
// or as a fallback when a real Notifier cannot be created for whatever reason.
type FakeNotifier struct{}

// NewFake creates a FakeNotifier.
func NewFake() FakeNotifier {
	return FakeNotifier{}
}

// Notify prints the messages to the logs.
func (n FakeNotifier) Notify(msg string) error {
	log.State("FakeNotifier:", sanitize(msg))
	return nil
}

// The code points in the surrogate range are not valid for UTF-8.
const (
	surrogateMin = 0xD800
	surrogateMax = 0xDFFF
)

// sanitize replaces control codes by the tofu symbol
// and invalid UTF-8 codes by the replacement character.
// sanitize can be used to prevent log injection.
//
// Inspired from:
// - https://wikiless.org/wiki/Replacement_character#Replacement_character
// - https://graphicdesign.stackexchange.com/q/108297
func sanitize(str string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r < 32, r == 127: // The .notdef character is often represented by the empty box (tofu)
			return '􏿮' // to indicate a valid but not rendered character.
		case surrogateMin <= r && r <= surrogateMax, utf8.MaxRune < r:
			return '�' // The replacement character U+FFFD indicates an invalid UTF-8 character.
		}
		return r
	}, str)
}
