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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/imprezahost/impreza-s3-server/internal/auth"
	objectlock "github.com/imprezahost/impreza-s3-server/internal/bucket/object/lock"
	xhttp "github.com/imprezahost/impreza-s3-server/internal/http"
)

// setBucketDefaultRetention turns on Object Lock and versioning for bucket, with a 2-day default retention in
// mode (no default rule when mode is empty).
func setBucketDefaultRetention(t *testing.T, obj ObjectLayer, bucket, mode string) {
	t.Helper()
	meta, err := globalBucketMetadataSys.Get(bucket)
	if err != nil {
		t.Fatal(err)
	}
	rule := ""
	if mode != "" {
		rule = fmt.Sprintf(`<Rule><DefaultRetention><Mode>%s</Mode><Days>2</Days></DefaultRetention></Rule>`, mode)
	}
	meta.ObjectLockConfigXML = []byte(`<ObjectLockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><ObjectLockEnabled>Enabled</ObjectLockEnabled>` + rule + `</ObjectLockConfiguration>`)
	meta.VersioningConfigXML = []byte(`<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>Enabled</Status></VersioningConfiguration>`)
	if err := meta.parseAllConfigs(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	globalBucketMetadataSys.Set(bucket, meta)
}

// A legal-hold header, ON or OFF, is independent of retention: a PUT that carries one still inherits the bucket's
// default retention. Before, any legal-hold header suppressed the default, so a caller with s3:PutObjectLegalHold
// could write outside the bucket's mandatory retention.
func TestObjectLockDefaultRetentionWithLegalHold(t *testing.T) {
	ExecObjectLayerAPITest(ExecObjectLayerAPITestArgs{
		t: t,
		objAPITest: func(obj ObjectLayer, instanceType, bucket string, _ http.Handler, _ auth.Credentials, t *testing.T) {
			for _, mode := range []objectlock.RetMode{objectlock.RetCompliance, objectlock.RetGovernance} {
				setBucketDefaultRetention(t, obj, bucket, string(mode))
				for _, hold := range []string{"", "OFF", "ON"} {
					t.Run(fmt.Sprintf("%s/%s/hold=%q", instanceType, mode, hold), func(t *testing.T) {
						r := httptest.NewRequest(http.MethodPut, "http://minio.local/"+bucket+"/object", nil)
						if hold != "" {
							r.Header.Set(xhttp.AmzObjectLockLegalHold, hold)
						}
						before := UTCNow()
						gotMode, date, legalHold, code := checkPutObjectLockAllowed(t.Context(), r, bucket, "object", obj.GetObjectInfo, ErrNone, ErrNone, false)
						if code != ErrNone || gotMode != mode {
							t.Fatalf("default retention not applied: mode=%q date=%v code=%s", gotMode, date.Time, niceError(code))
						}
						if date.Before(before.Add(47*time.Hour)) || date.After(UTCNow().Add(49*time.Hour)) {
							t.Fatalf("retain-until %v is not now + 2 days", date.Time)
						}
						if want := objectlock.LegalHoldStatus(hold); hold != "" && legalHold.Status != want {
							t.Fatalf("legal hold %q not kept beside the retention: %+v", hold, legalHold)
						}
					})
				}
			}
			// The legal hold still needs its own permission.
			r := httptest.NewRequest(http.MethodPut, "http://minio.local/"+bucket+"/object", nil)
			r.Header.Set(xhttp.AmzObjectLockLegalHold, "OFF")
			if _, _, _, code := checkPutObjectLockAllowed(t.Context(), r, bucket, "object", obj.GetObjectInfo, ErrNone, ErrAccessDenied, false); code != ErrAccessDenied {
				t.Fatalf("legal hold without permission: %s", niceError(code))
			}
		},
	})
}

// A write that names an existing version is checked against that version's lock state. A version that cannot be
// read is not known to be unlocked, so only "not found" lets the write through; unexpired GOVERNANCE retention
// blocks it like COMPLIANCE does.
func TestObjectLockVersionPreCheckFailsClosed(t *testing.T) {
	ExecObjectLayerAPITest(ExecObjectLayerAPITestArgs{
		t: t,
		objAPITest: func(obj ObjectLayer, instanceType, bucket string, _ http.Handler, _ auth.Credentials, t *testing.T) {
			setBucketDefaultRetention(t, obj, bucket, "")
			vid := mustGetUUID()
			locked := func(mode objectlock.RetMode, until time.Time) map[string]string {
				return map[string]string{
					strings.ToLower(xhttp.AmzObjectLockMode):            string(mode),
					strings.ToLower(xhttp.AmzObjectLockRetainUntilDate): until.UTC().Format(time.RFC3339),
				}
			}
			for _, tc := range []struct {
				name string
				info ObjectInfo
				err  error
				want APIErrorCode
			}{
				{name: "read error", err: InsufficientReadQuorum{}, want: ErrSlowDownRead},
				{name: "version not found", err: VersionNotFound{Bucket: bucket, Object: "object", VersionID: vid}, want: ErrNone},
				{name: "object not found", err: ObjectNotFound{Bucket: bucket, Object: "object"}, want: ErrNone},
				{name: "governance unexpired", info: ObjectInfo{UserDefined: locked(objectlock.RetGovernance, UTCNow().Add(time.Hour))}, want: ErrObjectLocked},
				{name: "compliance unexpired", info: ObjectInfo{UserDefined: locked(objectlock.RetCompliance, UTCNow().Add(time.Hour))}, want: ErrObjectLocked},
				{name: "governance expired", info: ObjectInfo{UserDefined: locked(objectlock.RetGovernance, UTCNow().Add(-time.Hour))}, want: ErrNone},
			} {
				t.Run(instanceType+"/"+tc.name, func(t *testing.T) {
					getInfo := func(context.Context, string, string, ObjectOptions) (ObjectInfo, error) {
						return tc.info, tc.err
					}
					r := httptest.NewRequest(http.MethodPut, "http://minio.local/"+bucket+"/object?versionId="+vid, nil)
					if err := r.ParseForm(); err != nil {
						t.Fatal(err)
					}
					_, _, _, code := checkPutObjectLockAllowed(t.Context(), r, bucket, "object", getInfo, ErrNone, ErrNone, false)
					if tc.want == ErrNone && code != ErrNone || tc.want != ErrNone && code == ErrNone {
						t.Fatalf("got %s, want %s", niceError(code), niceError(tc.want))
					}
					if tc.want == ErrObjectLocked && code != ErrObjectLocked {
						t.Fatalf("got %s, want ErrObjectLocked", niceError(code))
					}
				})
			}
		},
	})
}
