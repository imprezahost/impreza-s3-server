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
		name    string
		value   string
		unset   bool
		enabled bool
	}{
		{name: "unset", unset: true},
		{name: "empty"},
		{name: "explicit_off", value: "off"},
		{name: "explicit_on", value: "on", enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MINIO_BROWSER", tc.value)
			if tc.unset {
				if err := os.Unsetenv("MINIO_BROWSER"); err != nil {
					t.Fatal(err)
				}
			}
			globalBrowserEnabled = !tc.enabled
			serverHandleEarlyEnvVars()
			if globalBrowserEnabled != tc.enabled {
				t.Fatalf("Console enabled = %v, want %v", globalBrowserEnabled, tc.enabled)
			}
		})
	}
}
