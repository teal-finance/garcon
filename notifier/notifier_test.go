// Copyright 2022 Teal.Finance/Garcon contributors
// This file is part of Teal.Finance/Garcon,
// an API and website server under the MIT License.
// SPDX-License-Identifier: MIT

package notifier_test

import (
	"testing"

	"github.com/teal-finance/garcon/notifier"
)

func TestNotifier_Notify(t *testing.T) {
	url := "https://framateam.org/hooks/your-mattermost-hook-url"
	n := notifier.New(url)
	err := n.Notify("Hello, world!")

	want := "MattermostNotifier: 405 Method Not Allowed from host=framateam.org"
	if err.Error() != want {
		t.Error("got:  " + err.Error())
		t.Error("want: " + want)
	}
}
