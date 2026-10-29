// Copyright (c) 2026 Impreza Host.
// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"testing"
)

// Exercise the startup environment entry point, including overriding stale state.
func TestConsoleDefaultOff(t *testing.T) {
	previous := globalBrowserEnabled
	t.Cleanup(func() { globalBrowserEnabled = previous })
	for _, tc := range []struct {
		name  string
		value string
		unset bool
	}{
		{name: "unset", unset: true},
		{name: "empty"},
		{name: "explicit_off", value: "off"},
		{name: "explicit_on", value: "on"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MINIO_BROWSER", tc.value)
			if tc.unset {
				if err := os.Unsetenv("MINIO_BROWSER"); err != nil {
					t.Fatal(err)
				}
			}
			globalBrowserEnabled = true
			serverHandleEarlyEnvVars()
			if globalBrowserEnabled {
				t.Fatal("removed Console became enabled")
			}
		})
	}
}
