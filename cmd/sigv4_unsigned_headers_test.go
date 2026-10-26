// Copyright (c) 2015-2026 MinIO, Inc.
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

package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// CVE-2026-97731 through the real server. A SigV4-signed PUT that gains an unsigned x-amz-copy-source must
// be refused instead of becoming a server-side copy executed as the signer (on the unpatched base the copy goes
// through with 200). The same copy with the header signed still works, and object tagging keeps working now that
// the tag header the server derives from the body or the stored object is injected after signature verification.

func (s *TestSuiteCommon) makeBucketWithObjectV4(c *check, objectName string, data []byte) string {
	c.Helper()
	bucketName := getRandomBucketName()
	req, err := newTestSignedRequestV4(http.MethodPut, getMakeBucketURL(s.endPoint, bucketName), 0, nil, s.accessKey, s.secretKey, nil)
	c.Assert(err, nil)
	resp, err := s.client.Do(req)
	c.Assert(err, nil)
	c.Assert(resp.StatusCode, http.StatusOK)
	req, err = newTestSignedRequestV4(http.MethodPut, getPutObjectURL(s.endPoint, bucketName, objectName),
		int64(len(data)), bytes.NewReader(data), s.accessKey, s.secretKey, nil)
	c.Assert(err, nil)
	resp, err = s.client.Do(req)
	c.Assert(err, nil)
	c.Assert(resp.StatusCode, http.StatusOK)
	return bucketName
}

func (s *TestSuiteCommon) TestSigV4UnsignedCopySourceRefused(c *check) {
	c.Helper()
	bucketName := s.makeBucketWithObjectV4(c, "private.txt", []byte("private bytes\n"))

	// A signed one-object PUT, with x-amz-copy-source added after signing (what a presigned-URL holder can do).
	data := []byte("innocent\n")
	req, err := newTestSignedRequestV4(http.MethodPut, getPutObjectURL(s.endPoint, bucketName, "copy-target.txt"),
		int64(len(data)), bytes.NewReader(data), s.accessKey, s.secretKey, nil)
	c.Assert(err, nil)
	req.Header.Set("X-Amz-Copy-Source", url.QueryEscape(bucketName+"/private.txt"))
	resp, err := s.client.Do(req)
	c.Assert(err, nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	c.Assert(resp.StatusCode, http.StatusBadRequest)
	c.Assert(strings.Contains(string(body), "AccessDenied"), true)

	// The target was not created by a copy.
	req, err = newTestSignedRequestV4(http.MethodGet, getGetObjectURL(s.endPoint, bucketName, "copy-target.txt"), 0, nil, s.accessKey, s.secretKey, nil)
	c.Assert(err, nil)
	resp, err = s.client.Do(req)
	c.Assert(err, nil)
	resp.Body.Close()
	c.Assert(resp.StatusCode, http.StatusNotFound)

	// The same copy with the header signed, as every S3 client sends it, still works.
	req, err = newTestSignedRequestV4(http.MethodPut, getPutObjectURL(s.endPoint, bucketName, "copy-target.txt"), 0, nil,
		s.accessKey, s.secretKey, map[string]string{"X-Amz-Copy-Source": url.QueryEscape(bucketName + "/private.txt")})
	c.Assert(err, nil)
	resp, err = s.client.Do(req)
	c.Assert(err, nil)
	resp.Body.Close()
	c.Assert(resp.StatusCode, http.StatusOK)
}

func (s *TestSuiteCommon) TestObjectTaggingAfterSignatureVerification(c *check) {
	c.Helper()
	bucketName := s.makeBucketWithObjectV4(c, "tagged.txt", []byte("tagged\n"))
	taggingURL := makeTestTargetURL(s.endPoint, bucketName, "tagged.txt", url.Values{"tagging": []string{""}})

	tagging := []byte(`<Tagging><TagSet><Tag><Key>project</Key><Value>tagged</Value></Tag></TagSet></Tagging>`)
	req, err := newTestSignedRequestV4(http.MethodPut, taggingURL, int64(len(tagging)), bytes.NewReader(tagging), s.accessKey, s.secretKey, nil)
	c.Assert(err, nil)
	resp, err := s.client.Do(req)
	c.Assert(err, nil)
	resp.Body.Close()
	c.Assert(resp.StatusCode, http.StatusOK)

	req, err = newTestSignedRequestV4(http.MethodGet, taggingURL, 0, nil, s.accessKey, s.secretKey, nil)
	c.Assert(err, nil)
	resp, err = s.client.Do(req)
	c.Assert(err, nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	c.Assert(resp.StatusCode, http.StatusOK)
	c.Assert(strings.Contains(string(body), "<Key>project</Key>"), true)

	// The object now has tags, which DeleteObjectTagging injects as a header for policy conditions.
	req, err = newTestSignedRequestV4(http.MethodDelete, taggingURL, 0, nil, s.accessKey, s.secretKey, nil)
	c.Assert(err, nil)
	resp, err = s.client.Do(req)
	c.Assert(err, nil)
	resp.Body.Close()
	c.Assert(resp.StatusCode, http.StatusNoContent)
}
