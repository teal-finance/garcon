// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package gg

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Notifier interface for sending messages.
type Notifier interface {
	Notify(message string) error
}

// New selects the Notifier type depending on the endpoint pattern.
func NewNotifier(endpoint string) Notifier {
	switch endpoint {
	case "":
		log.Info("empty URL => use the FakeNotifier")
		return NewFakeNotifier()
	default:
		return NewMattermostNotifier(endpoint)
	}
}

// FakeNotifier implements a Notifier interface that logs the received notifications.
// FakeNotifier can be used as a mocked Notifier or for debugging purpose
// or as a fallback when a real Notifier cannot be created for whatever reason.
type FakeNotifier struct{}

// NewFakeNotifier creates a FakeNotifier.
func NewFakeNotifier() FakeNotifier {
	return FakeNotifier{}
}

// Notify prints the messages to the logs.
func (n FakeNotifier) Notify(msg string) error {
	log.State("FakeNotifier:", sanitize(msg))
	return nil
}

// MattermostNotifier for sending messages to a Mattermost server.
type MattermostNotifier struct {
	endpoint string
}

// NewMattermostNotifier creates a new MattermostNotifier given a Mattermost server endpoint (see mattermost hooks).
func NewMattermostNotifier(endpoint string) MattermostNotifier {
	return MattermostNotifier{endpoint}
}

// Notify sends a message to the Mattermost server.
func (n MattermostNotifier) Notify(msg string) error {
	buf := strconv.AppendQuoteToGraphic([]byte(`{"text":`), msg)
	buf = append(buf, byte('}'))
	body := bytes.NewBuffer(buf)

	resp, err := http.Post(n.endpoint, "application/json", body)
	if err != nil {
		return fmt.Errorf("MattermostNotifier: %w from host=%s", err, n.host())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MattermostNotifier: %s from host=%s", resp.Status, n.host())
	}
	return nil
}

func (n MattermostNotifier) host() string {
	u, err := url.Parse(n.endpoint)
	if err == nil {
		return u.Hostname()
	}
	return ""
}
