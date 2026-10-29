// Copyright (c) 2026 Impreza Host.
// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"net/url"
	"testing"
)

type forbiddenUpdateReader struct{}

func (forbiddenUpdateReader) Read([]byte) (int, error) {
	panic("disabled update must not read executable bytes")
}

func TestInPlaceUpdateMechanicsDisabled(t *testing.T) {
	u, err := url.Parse("file:///not-an-update")
	if err != nil {
		t.Fatal(err)
	}
	a, b, err := downloadBinary(u, "test")
	if err == nil || err.Error() != "in-place binary updates are disabled" || a != nil || b != nil {
		t.Fatalf("downloadBinary = %v/%v/%v", a, b, err)
	}
	if err := verifyBinary(u, nil, "release", "test", forbiddenUpdateReader{}); err == nil || err.Error() != "in-place binary updates are disabled" {
		t.Fatalf("verifyBinary = %v", err)
	}
	if err := commitBinary(); err == nil || err.Error() != "in-place binary updates are disabled" {
		t.Fatalf("commitBinary = %v", err)
	}
}
