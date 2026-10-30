package cmd

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/imprezahost/impreza-s3-server/internal/auth"
)

// The PutObjectRetention handler itself, not only isRetentionActionAllowed: a caller holding only
// s3:BypassGovernanceRetention must be refused before any bucket or object state is read.
func TestPutObjectRetentionHandlerRequiresPutObjectRetention(t *testing.T) {
	ExecObjectLayerAPITest(ExecObjectLayerAPITestArgs{
		t: t,
		objAPITest: func(_ ObjectLayer, instanceType, bucket string, router http.Handler, _ auth.Credentials, t *testing.T) {
			bypassOnly := newRetentionPolicyUser(t, bucket,
				`{"Effect":"Allow","Action":["s3:BypassGovernanceRetention"],"Resource":["arn:aws:s3:::BUCKET/*"]}`)
			retention := newRetentionPolicyUser(t, bucket,
				`{"Effect":"Allow","Action":["s3:PutObjectRetention"],"Resource":["arn:aws:s3:::BUCKET/*"]}`)
			body := []byte(`<Retention xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Mode>GOVERNANCE</Mode><RetainUntilDate>` +
				UTCNow().Add(48*time.Hour).Format(time.RFC3339) + `</RetainUntilDate></Retention>`)
			sum := md5.Sum(body)
			send := func(cred auth.Credentials) *httptest.ResponseRecorder {
				req, err := newTestSignedRequestV4(http.MethodPut, getPutObjectURL("", bucket, "object")+"?retention", int64(len(body)),
					bytes.NewReader(body), cred.AccessKey, cred.SecretKey, map[string]string{"Content-Md5": base64.StdEncoding.EncodeToString(sum[:])})
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				return rec
			}
			if rec := send(bypassOnly); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "AccessDenied") {
				t.Errorf("%s: bypass-only caller: %d %s, want 403 AccessDenied", instanceType, rec.Code, rec.Body.String())
			}
			if rec := send(retention); strings.Contains(rec.Body.String(), "AccessDenied") {
				t.Errorf("%s: PutObjectRetention caller refused as AccessDenied: %d %s", instanceType, rec.Code, rec.Body.String())
			}
		},
	})
}

// The real startup path (serverHandleEnvVars) must keep in-place update off even when MINIO_UPDATE asks for it;
// TestUpdateCheckAndInplaceUpdateRemoved only reads the initial value.
func TestUpdateStaysDisabledThroughStartupEnv(t *testing.T) {
	previous := globalInplaceUpdateDisabled
	t.Cleanup(func() { globalInplaceUpdateDisabled = previous })
	t.Setenv("MINIO_UPDATE", "on")
	globalInplaceUpdateDisabled = false
	serverHandleEnvVars()
	if !globalInplaceUpdateDisabled {
		t.Fatal("MINIO_UPDATE=on re-enabled in-place update through the startup path")
	}
}
