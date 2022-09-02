// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package notifier

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// MattermostNotifier for sending messages to a Mattermost server.
type MattermostNotifier struct {
	endpoint string
}

// NewMattermost creates a new MattermostNotifier given a Mattermost server endpoint (see mattermost hooks).
func NewMattermost(endpoint string) MattermostNotifier {
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
	if url, er := url.Parse(n.endpoint); er == nil {
		return url.Hostname()
	}
	return ""
}
