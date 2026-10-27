// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/minio/minio/internal/auth"
)

// With the default configuration, a client cannot choose aws:SourceIp with X-Forwarded-For, X-Real-IP or
// Forwarded: the condition sees the connection's address. An onion service reaches the server over loopback, so
// without this every Tor client could pick the address an IpAddress condition and the audit log see.
func TestSourceIPConditionIgnoresForwardedHeaders(t *testing.T) {
	if os.Getenv("_MINIO_API_XFF_HEADER") != "" {
		t.Skip("_MINIO_API_XFF_HEADER is set in this environment")
	}
	for _, h := range []struct{ key, val string }{
		{"X-Forwarded-For", "8.8.8.8"},
		{"X-Real-IP", "8.8.4.4"},
		{"Forwarded", "for=192.0.2.60"},
	} {
		r := httptest.NewRequest(http.MethodGet, "http://minio.local/bucket/object", nil)
		r.RemoteAddr = "127.0.0.1:41000"
		r.Header.Set(h.key, h.val)
		got := getConditionValues(r, "", auth.Credentials{AccessKey: "lowpriv"})["SourceIp"]
		if !slices.Equal(got, []string{"127.0.0.1"}) {
			t.Errorf("%s: aws:SourceIp is %v, want the connection address 127.0.0.1", h.key, got)
		}
	}
}

// The update check and in-place update contact dl.min.io; this build never offers them.
func TestUpdateCheckAndInplaceUpdateRemoved(t *testing.T) {
	if !globalInplaceUpdateDisabled {
		t.Fatal("in-place update is enabled")
	}
	if MinioReleaseBaseURL != "" {
		t.Fatalf("a release URL is still configured: %q", MinioReleaseBaseURL)
	}
}
