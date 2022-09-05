// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package notifier

import (
	"github.com/teal-finance/emo"
)

var log = emo.NewZone("notifier")

// Notifier interface for sending messages.
type Notifier interface {
	Notify(message string) error
}

// New selects the Notifier type depending on the endpoint pattern.
func New(endpoint string) Notifier {
	switch endpoint {
	case "":
		log.Info("empty URL => use the FakeNotifier")
		return NewFake()
	default:
		return NewMattermost(endpoint)
	}
}
