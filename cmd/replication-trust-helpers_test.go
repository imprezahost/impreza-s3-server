// Copyright (c) 2015-2026 MinIO, Inc.
// Copyright (c) 2026 PGSTY
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio/internal/auth"
	"github.com/minio/pkg/v3/policy"
)

// Test helpers used by replication-trust_test.go. Upstream (pgsty/silo) defines them in test files of features this
// fork does not carry (object-copy-checksum_test.go and object-attributes-ssec-authz_test.go); they are copied here
// unchanged so the replication trust tests can run on their own.

func putCopyChecksumSource(t *testing.T, apiRouter http.Handler, credentials auth.Credentials,
	bucket, object string, data []byte, headers map[string]string,
) {
	t.Helper()
	req, err := newTestSignedRequestV4(http.MethodPut, getPutObjectURL("", bucket, object),
		int64(len(data)), bytes.NewReader(data), credentials.AccessKey, credentials.SecretKey, headers)
	if err != nil {
		t.Fatalf("failed to build PutObject request: %v", err)
	}
	rec := httptest.NewRecorder()
	apiRouter.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PutObject(%s) failed: %d %s", object, rec.Code, rec.Body.String())
	}
}

func newObjectAttributesAuthzUser(t *testing.T, instanceType, bucketName, actions string) auth.Credentials {
	t.Helper()
	ctx := t.Context()

	accessKey, secretKey, err := auth.GenerateCredentials()
	if err != nil {
		t.Fatalf("%s: generate credentials: %v", instanceType, err)
	}
	creds := auth.Credentials{AccessKey: accessKey, SecretKey: secretKey}
	if _, err = globalIAMSys.CreateUser(ctx, creds.AccessKey, madmin.AddOrUpdateUserReq{
		SecretKey: creds.SecretKey,
		Status:    madmin.AccountEnabled,
	}); err != nil {
		t.Fatalf("%s: create attributes user: %v", instanceType, err)
	}

	policyJSON := `{
 "Version": "2012-10-17",
 "Statement": [{
  "Effect": "Allow",
  "Action": [` + actions + `],
  "Resource": ["arn:aws:s3:::` + bucketName + `/*"]
 }]
}`
	parsed, err := policy.ParseConfig(strings.NewReader(policyJSON))
	if err != nil {
		t.Fatalf("%s: parse attributes policy: %v", instanceType, err)
	}
	policyName := "attributes-authz-" + mustGetUUID()
	if _, err = globalIAMSys.SetPolicy(ctx, policyName, *parsed); err != nil {
		t.Fatalf("%s: install attributes policy: %v", instanceType, err)
	}
	if _, err = globalIAMSys.PolicyDBSet(ctx, creds.AccessKey, policyName, regUser, false); err != nil {
		t.Fatalf("%s: attach attributes policy: %v", instanceType, err)
	}
	return creds
}
