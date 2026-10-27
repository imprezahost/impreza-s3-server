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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio/internal/auth"
	objectlock "github.com/minio/minio/internal/bucket/object/lock"
	xhttp "github.com/minio/minio/internal/http"
	"github.com/minio/pkg/v3/policy"
)

// newRetentionPolicyUser creates a user whose only policy is statements (a JSON array body) on bucket.
func newRetentionPolicyUser(t *testing.T, bucket, statements string) auth.Credentials {
	t.Helper()
	ctx := t.Context()
	accessKey, secretKey, err := auth.GenerateCredentials()
	if err != nil {
		t.Fatal(err)
	}
	creds := auth.Credentials{AccessKey: accessKey, SecretKey: secretKey}
	if _, err = globalIAMSys.CreateUser(ctx, creds.AccessKey, madmin.AddOrUpdateUserReq{SecretKey: creds.SecretKey, Status: madmin.AccountEnabled}); err != nil {
		t.Fatal(err)
	}
	doc := `{"Version":"2012-10-17","Statement":[` + strings.ReplaceAll(statements, "BUCKET", bucket) + `]}`
	parsed, err := policy.ParseConfig(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	name := "retention-" + mustGetUUID()
	if _, err = globalIAMSys.SetPolicy(ctx, name, *parsed); err != nil {
		t.Fatal(err)
	}
	if _, err = globalIAMSys.PolicyDBSet(ctx, creds.AccessKey, name, regUser, false); err != nil {
		t.Fatal(err)
	}
	return creds
}

// s3:PutObjectRetention is decided from the request alone. Before, a caller holding only
// s3:BypassGovernanceRetention could set GOVERNANCE retention, and clearing retention evaluated
// s3:object-lock-remaining-retention-days as the distance to year 0001 (math.Abs), so a maximum-days policy
// refused the clearing and a minimum-days policy accepted it.
func TestRetentionActionAllowedFromRequest(t *testing.T) {
	ExecObjectLayerAPITest(ExecObjectLayerAPITestArgs{
		t: t,
		objAPITest: func(obj ObjectLayer, instanceType, bucket string, _ http.Handler, _ auth.Credentials, t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "http://minio.local/"+bucket+"/object?retention", nil)
			bypassOnly := newRetentionPolicyUser(t, bucket,
				`{"Effect":"Allow","Action":["s3:BypassGovernanceRetention"],"Resource":["arn:aws:s3:::BUCKET/*"]}`)
			atMost30 := newRetentionPolicyUser(t, bucket,
				`{"Effect":"Allow","Action":["s3:PutObjectRetention"],"Resource":["arn:aws:s3:::BUCKET/*"],`+
					`"Condition":{"NumericLessThanEquals":{"s3:object-lock-remaining-retention-days":"30"}}}`)
			atLeast10 := newRetentionPolicyUser(t, bucket,
				`{"Effect":"Allow","Action":["s3:PutObjectRetention"],"Resource":["arn:aws:s3:::BUCKET/*"],`+
					`"Condition":{"NumericGreaterThanEquals":{"s3:object-lock-remaining-retention-days":"10"}}}`)
			in := func(d time.Duration) *objectlock.ObjectRetention {
				return &objectlock.ObjectRetention{Mode: objectlock.RetGovernance, RetainUntilDate: objectlock.RetentionDate{Time: UTCNow().Add(d)}}
			}
			cleared := &objectlock.ObjectRetention{}
			for _, tc := range []struct {
				name string
				cred auth.Credentials
				ret  *objectlock.ObjectRetention
				want bool
			}{
				{"bypass permission alone does not set retention", bypassOnly, in(24 * time.Hour), false},
				{"clearing counts as 0 days: within a 30-day maximum", atMost30, cleared, true},
				{"clearing counts as 0 days: below a 10-day minimum", atLeast10, cleared, false},
				{"20 days within the maximum", atMost30, in(20 * 24 * time.Hour), true},
				{"40 days over the maximum", atMost30, in(40 * 24 * time.Hour), false},
				{"20 days over the minimum", atLeast10, in(20 * 24 * time.Hour), true},
				{"a partial day counts as a day: 9 days and 1 hour is 10", atLeast10, in(9*24*time.Hour + time.Hour), true},
				{"anonymous", auth.Credentials{}, in(24 * time.Hour), false},
			} {
				t.Run(instanceType+"/"+tc.name, func(t *testing.T) {
					if got := isRetentionActionAllowed(policy.PutObjectRetentionAction, bucket, "object", tc.ret, r, tc.cred, false); got != tc.want {
						t.Fatalf("got %v, want %v", got, tc.want)
					}
				})
			}
		},
	})
}

// The stored retention decides what a PutObjectRetention may change, once s3:PutObjectRetention is held.
func TestEnforceRetentionBypassForPut(t *testing.T) {
	ExecObjectLayerAPITest(ExecObjectLayerAPITestArgs{
		t: t,
		objAPITest: func(obj ObjectLayer, instanceType, bucket string, _ http.Handler, _ auth.Credentials, t *testing.T) {
			stored := func(mode objectlock.RetMode, d time.Duration) ObjectInfo {
				return ObjectInfo{Bucket: bucket, Name: "object", UserDefined: map[string]string{
					strings.ToLower(xhttp.AmzObjectLockMode):            string(mode),
					strings.ToLower(xhttp.AmzObjectLockRetainUntilDate): UTCNow().Add(d).UTC().Format(time.RFC3339),
				}}
			}
			req := func(mode objectlock.RetMode, d time.Duration) *objectlock.ObjectRetention {
				if mode == "" {
					return &objectlock.ObjectRetention{}
				}
				return &objectlock.ObjectRetention{Mode: mode, RetainUntilDate: objectlock.RetentionDate{Time: UTCNow().Add(d)}}
			}
			stranger := auth.Credentials{AccessKey: "no-such-user"}
			day := 24 * time.Hour
			for _, tc := range []struct {
				name   string
				oi     ObjectInfo
				ret    *objectlock.ObjectRetention
				bypass bool
				cred   auth.Credentials
				owner  bool
				want   error
			}{
				{"no stored retention", ObjectInfo{Bucket: bucket, Name: "object", UserDefined: map[string]string{}}, req(objectlock.RetGovernance, day), false, stranger, false, nil},
				{"expired governance may be cleared", stored(objectlock.RetGovernance, -time.Hour), req("", 0), false, stranger, false, nil},
				{"compliance extended", stored(objectlock.RetCompliance, day), req(objectlock.RetCompliance, 2*day), false, stranger, false, nil},
				{"compliance shortened", stored(objectlock.RetCompliance, 2*day), req(objectlock.RetCompliance, day), true, globalActiveCred, true, ObjectLocked{}},
				{"compliance to governance", stored(objectlock.RetCompliance, day), req(objectlock.RetGovernance, 2*day), true, globalActiveCred, true, ObjectLocked{}},
				{"governance extended", stored(objectlock.RetGovernance, day), req(objectlock.RetGovernance, 2*day), false, stranger, false, nil},
				{"governance cleared without the bypass header", stored(objectlock.RetGovernance, day), req("", 0), false, globalActiveCred, true, ObjectLocked{}},
				{"governance cleared with the header but no permission", stored(objectlock.RetGovernance, day), req("", 0), true, stranger, false, errAuthentication},
				{"governance cleared with the header and permission", stored(objectlock.RetGovernance, day), req("", 0), true, globalActiveCred, true, nil},
			} {
				t.Run(instanceType+"/"+tc.name, func(t *testing.T) {
					r := httptest.NewRequest(http.MethodPut, "http://minio.local/"+bucket+"/object?retention", nil)
					if tc.bypass {
						r.Header.Set(xhttp.AmzObjectLockBypassGovernance, "true")
					}
					err := enforceRetentionBypassForPut(t.Context(), r, tc.oi, tc.ret, tc.cred, tc.owner)
					switch want := tc.want.(type) {
					case nil:
						if err != nil {
							t.Fatalf("refused: %v", err)
						}
					case ObjectLocked:
						if !errors.As(err, &want) {
							t.Fatalf("got %v, want ObjectLocked", err)
						}
					default:
						if !errors.Is(err, tc.want) {
							t.Fatalf("got %v, want %v", err, tc.want)
						}
					}
				})
			}
		},
	})
}
