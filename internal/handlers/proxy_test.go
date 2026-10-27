// Copyright (c) 2015-2021 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package handlers

import (
	"net/http"
	"os"
	"testing"
)

type headerTest struct {
	key      string // header key
	val      string // header val
	expected string // expected result
}

func TestGetScheme(t *testing.T) {
	headers := []headerTest{
		{xForwardedProto, "https", "https"},
		{xForwardedProto, "http", "http"},
		{xForwardedProto, "HTTP", "http"},
		{xForwardedScheme, "https", "https"},
		{xForwardedScheme, "http", "http"},
		{xForwardedScheme, "HTTP", "http"},
		{forwarded, `For="[2001:db8:cafe::17]:4711`, ""},                    // No proto
		{forwarded, `for=192.0.2.43, for=198.51.100.17;proto=https`, ""},    // Multiple params, will be empty.
		{forwarded, `for=172.32.10.15; proto=https;by=127.0.0.1;`, "https"}, // Space before proto
		{forwarded, `for=192.0.2.60;proto=http;by=203.0.113.43`, "http"},    // Multiple params
	}
	for _, v := range headers {
		req := &http.Request{
			Header: http.Header{
				v.key: []string{v.val},
			},
		}
		res := GetSourceScheme(req)
		if res != v.expected {
			t.Errorf("wrong header for %s: got %s want %s", v.key, res,
				v.expected)
		}
	}
}

// TestGetSourceIP - check the source ip of a request is parsed correctly when
// the forwarded headers are trusted (behind a proxy that overwrites them).
func TestGetSourceIP(t *testing.T) {
	trustForwardedHeaders = true
	t.Cleanup(func() { trustForwardedHeaders = false })
	headers := []headerTest{
		{xForwardedFor, "8.8.8.8", "8.8.8.8"},                                         // Single address
		{xForwardedFor, "8.8.8.8, 8.8.4.4", "8.8.8.8"},                                // Multiple
		{xForwardedFor, "", ""},                                                       // None
		{xRealIP, "8.8.8.8", "8.8.8.8"},                                               // Single address
		{xRealIP, "[2001:db8:cafe::17]:4711", "[2001:db8:cafe::17]"},                  // IPv6 address
		{xRealIP, "", ""},                                                             // None
		{forwarded, `for="_gazonk"`, "_gazonk"},                                       // Hostname
		{forwarded, `For="[2001:db8:cafe::17]:4711`, `[2001:db8:cafe::17]`},           // IPv6 address
		{forwarded, `for=192.0.2.60;proto=http;by=203.0.113.43`, `192.0.2.60`},        // Multiple params
		{forwarded, `for=192.0.2.43, for=198.51.100.17`, "192.0.2.43"},                // Multiple params
		{forwarded, `for="workstation.local",for=198.51.100.17`, "workstation.local"}, // Hostname
	}

	for _, v := range headers {
		req := &http.Request{
			Header: http.Header{
				v.key: []string{v.val},
			},
		}
		res := GetSourceIP(req)
		if res != v.expected {
			t.Errorf("wrong header for %s: got %s want %s", v.key, res,
				v.expected)
		}
	}
}

// Untrusted by default: none of X-Forwarded-For, X-Real-IP or Forwarded changes the source IP, which stays the
// connection's address. Trusted (_MINIO_API_XFF_HEADER=on), X-Forwarded-For takes precedence.
func TestForwardedHeadersIgnoredByDefault(t *testing.T) {
	if os.Getenv("_MINIO_API_XFF_HEADER") == "" && trustForwardedHeaders {
		t.Fatal("forwarded headers are trusted without _MINIO_API_XFF_HEADER=on")
	}
	trustForwardedHeaders = false
	t.Cleanup(func() { trustForwardedHeaders = false })
	for _, h := range []http.Header{
		{xForwardedFor: []string{"8.8.8.8"}},
		{xRealIP: []string{"1.1.1.1"}},
		{forwarded: []string{"for=192.0.2.60;proto=http"}},
		{xForwardedFor: []string{"8.8.8.8"}, xRealIP: []string{"1.1.1.1"}, forwarded: []string{"for=192.0.2.60"}},
	} {
		req := &http.Request{Header: h, RemoteAddr: "127.0.0.1:41000"}
		if got := GetSourceIPFromHeaders(req); got != "" {
			t.Errorf("%v: header address used: %s", h, got)
		}
		if got := GetSourceIP(req); got != "127.0.0.1" {
			t.Errorf("%v: got %s, want the connection address 127.0.0.1", h, got)
		}
	}

	trustForwardedHeaders = true
	req := &http.Request{
		Header:     http.Header{xForwardedFor: []string{"8.8.8.8"}, xRealIP: []string{"1.1.1.1"}},
		RemoteAddr: "127.0.0.1:41000",
	}
	if got := GetSourceIP(req); got != "8.8.8.8" {
		t.Errorf("trusted: X-Forwarded-For takes precedence: got %s, want 8.8.8.8", got)
	}
}
