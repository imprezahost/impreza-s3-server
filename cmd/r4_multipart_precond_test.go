package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tinylib/msgp/msgp"
)

// Multi-pool conditional CompleteMultipartUpload: the condition must be
// evaluated against the logical latest object across pools, and atomically
// with the completion (under the object write lock).

func r4Pools(t *testing.T) (*erasureServerPools, string) {
	t.Helper()
	z, bucket, _ := r4PoolsDirs(t)
	return z, bucket
}

func r4PoolsDirs(t *testing.T) (*erasureServerPools, string, []string) {
	t.Helper()
	objLayer, fsDirs, err := prepareErasurePools()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeRoots(fsDirs) })
	z := objLayer.(*erasureServerPools)
	bucket := "r4-precond"
	if err = z.MakeBucket(t.Context(), bucket, MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	return z, bucket, fsDirs
}

func r4Put(t *testing.T, z *erasureServerPools, bucket, object string, pool int, data string, opts ObjectOptions) ObjectInfo {
	t.Helper()
	oi, err := z.serverPools[pool].PutObject(t.Context(), bucket, object,
		mustGetPutObjReader(t, bytes.NewBufferString(data), int64(len(data)), "", ""), opts)
	if err != nil {
		t.Fatal(err)
	}
	return oi
}

func r4Upload(t *testing.T, z *erasureServerPools, bucket, object string, pool int) (string, []CompletePart) {
	t.Helper()
	ctx := t.Context()
	mp, err := z.serverPools[pool].NewMultipartUpload(ctx, bucket, object, ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pi, err := z.PutObjectPart(ctx, bucket, object, mp.UploadID, 1,
		mustGetPutObjReader(t, bytes.NewBufferString("part"), 4, "", ""), ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return mp.UploadID, []CompletePart{{PartNumber: 1, ETag: pi.ETag}}
}

// The callbacks keep the early returns of checkPreconditionsPUT: a zero
// modtime or a delete marker passes the condition, so the object layer must
// turn a delete-marker latest into absence itself.
func r4IfMatch(etag string) ObjectOptions {
	return ObjectOptions{HasIfMatch: true, CheckPrecondFn: func(oi ObjectInfo) bool {
		if oi.ModTime.IsZero() || oi.DeleteMarker {
			return false
		}
		return oi.ETag != etag
	}}
}

func r4IfNoneMatchStar() ObjectOptions {
	return ObjectOptions{CheckPrecondFn: func(oi ObjectInfo) bool {
		if oi.ModTime.IsZero() || oi.DeleteMarker {
			return false
		}
		return oi.ETag != ""
	}}
}

// The upload lives in pool 0 next to a stale copy; the logical latest is in pool 1.
func TestR4CompleteIfMatchStaleETagRejected(t *testing.T) {
	z, bucket := r4Pools(t)
	now := time.Now()
	old := r4Put(t, z, bucket, "obj", 0, "old", ObjectOptions{MTime: now.Add(-2 * time.Minute)})
	r4Put(t, z, bucket, "obj", 1, "new", ObjectOptions{MTime: now.Add(-time.Minute)})
	id, parts := r4Upload(t, z, bucket, "obj", 0)
	_, err := z.CompleteMultipartUpload(t.Context(), bucket, "obj", id, parts, r4IfMatch(old.ETag))
	var pcf PreConditionFailed
	if !errors.As(err, &pcf) {
		t.Fatalf("If-Match with the stale pool-0 ETag must fail with PreConditionFailed, got %v", err)
	}
}

func TestR4CompleteIfMatchCurrentETagAccepted(t *testing.T) {
	z, bucket := r4Pools(t)
	now := time.Now()
	r4Put(t, z, bucket, "obj", 0, "old", ObjectOptions{MTime: now.Add(-2 * time.Minute)})
	cur := r4Put(t, z, bucket, "obj", 1, "new", ObjectOptions{MTime: now.Add(-time.Minute)})
	id, parts := r4Upload(t, z, bucket, "obj", 0)
	if _, err := z.CompleteMultipartUpload(t.Context(), bucket, "obj", id, parts, r4IfMatch(cur.ETag)); err != nil {
		t.Fatalf("If-Match with the logical latest ETag must pass, got %v", err)
	}
}

// The newest state is a delete marker in pool 0; pool 1 keeps an older copy.
// The key reads as absent, so If-Match must fail and If-None-Match must pass,
// whatever the pool iteration order.
func r4DeleteMarkerNewest(t *testing.T, dmPool, copyPool int) (*erasureServerPools, string, ObjectInfo) {
	t.Helper()
	z, bucket := r4Pools(t)
	now := time.Now()
	stale := r4Put(t, z, bucket, "obj", copyPool, "stale", ObjectOptions{Versioned: true, MTime: now.Add(-3 * time.Minute)})
	r4Put(t, z, bucket, "obj", dmPool, "gone", ObjectOptions{Versioned: true, MTime: now.Add(-2 * time.Minute)})
	dm, err := z.serverPools[dmPool].DeleteObject(t.Context(), bucket, "obj", ObjectOptions{Versioned: true, MTime: now.Add(-time.Minute)})
	if err != nil || !dm.DeleteMarker {
		t.Fatalf("delete marker in pool %d: %v (dm=%v)", dmPool, err, dm.DeleteMarker)
	}
	return z, bucket, stale
}

func TestR4CompleteDeleteMarkerNewestIsAbsence(t *testing.T) {
	for _, c := range []struct {
		name   string
		dm, cp int
	}{{"marker-in-pool0", 0, 1}, {"marker-in-pool1", 1, 0}} {
		t.Run(c.name+"/if-match", func(t *testing.T) {
			z, bucket, stale := r4DeleteMarkerNewest(t, c.dm, c.cp)
			id, parts := r4Upload(t, z, bucket, "obj", c.cp)
			opts := r4IfMatch(stale.ETag)
			opts.Versioned = true
			_, err := z.CompleteMultipartUpload(t.Context(), bucket, "obj", id, parts, opts)
			if err == nil {
				t.Fatal("If-Match against a key whose latest is a delete marker must fail; it passed")
			}
			if !isErrObjectNotFound(err) {
				t.Fatalf("If-Match on an absent key must fail as ObjectNotFound (404), got %T %v", err, err)
			}
		})
		t.Run(c.name+"/if-none-match", func(t *testing.T) {
			z, bucket, _ := r4DeleteMarkerNewest(t, c.dm, c.cp)
			id, parts := r4Upload(t, z, bucket, "obj", c.cp)
			opts := r4IfNoneMatchStar()
			opts.Versioned = true
			if _, err := z.CompleteMultipartUpload(t.Context(), bucket, "obj", id, parts, opts); err != nil {
				t.Fatalf("If-None-Match:* on a key whose latest is a delete marker must pass, got %v", err)
			}
		})
	}
}

// Atomicity: a writer holding the object lock creates the key while the
// completion waits. If-None-Match:* must then see the object and refuse;
// evaluating the condition before taking the object lock lets the
// completion overwrite it.
func TestR4CompleteIfNoneMatchEvaluatedUnderObjectLock(t *testing.T) {
	// winner-same-pool isolates atomicity (the base passes it: its set-local
	// check runs under the object lock); winner-other-pool needs both the
	// cross-pool read and the lock.
	for _, c := range []struct {
		name       string
		winnerPool int
	}{{"winner-same-pool", 0}, {"winner-other-pool", 1}} {
		t.Run(c.name, func(t *testing.T) { r4AtomicIfNoneMatch(t, c.winnerPool) })
	}
}

func r4AtomicIfNoneMatch(t *testing.T, winnerPool int) {
	z, bucket := r4Pools(t)
	ctx := t.Context()
	id, parts := r4Upload(t, z, bucket, "obj", 0)

	lk := z.NewNSLock(bucket, "obj")
	lkctx, err := lk.GetLock(ctx, globalOperationTimeout)
	if err != nil {
		t.Fatal(err)
	}
	type res struct {
		oi  ObjectInfo
		err error
	}
	done := make(chan res, 1)
	go func() {
		oi, err := z.CompleteMultipartUpload(context.Background(), bucket, "obj", id, parts, r4IfNoneMatchStar())
		done <- res{oi, err}
	}()
	time.Sleep(500 * time.Millisecond)
	select {
	case r := <-done:
		lk.Unlock(lkctx)
		t.Fatalf("completion finished while another writer held the object lock: %v", r.err)
	default:
	}
	winner := r4Put(t, z, bucket, "obj", winnerPool, "winner", ObjectOptions{NoLock: true})
	lk.Unlock(lkctx)

	r := <-done
	var pcf PreConditionFailed
	if !errors.As(r.err, &pcf) {
		t.Fatalf("If-None-Match:* must refuse once the concurrent writer created the key, got err=%v", r.err)
	}
	latest, _, err := z.getLatestObjectInfoWithIdx(ctx, bucket, "obj", ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if latest.ETag != winner.ETag {
		t.Fatalf("the concurrent writer's object was overwritten: latest %s, writer %s", latest.ETag, winner.ETag)
	}
}

// Fail closed: the newest copy lives in pool 1 but its metadata cannot be
// read (read quorum lost). An unreadable pool is not absence, so neither
// If-Match with the stale pool-0 ETag nor If-None-Match:* may pass.
func TestR4CompleteUnreadablePoolFailsClosed(t *testing.T) {
	for _, c := range []struct {
		name  string
		opts  func(stale ObjectInfo) ObjectOptions
		stale bool
	}{
		{"if-match-stale", func(stale ObjectInfo) ObjectOptions { return r4IfMatch(stale.ETag) }, true},
		{"if-none-match", func(ObjectInfo) ObjectOptions { return r4IfNoneMatchStar() }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			z, bucket, dirs := r4PoolsDirs(t)
			now := time.Now()
			var stale ObjectInfo
			if c.stale {
				stale = r4Put(t, z, bucket, "obj", 0, "old", ObjectOptions{MTime: now.Add(-2 * time.Minute)})
			}
			r4Put(t, z, bucket, "obj", 1, "new", ObjectOptions{MTime: now.Add(-time.Minute)})
			corrupted := 0
			for _, d := range dirs[16:] {
				meta := filepath.Join(d, bucket, "obj", xlStorageFormatFile)
				if _, err := os.Stat(meta); err != nil {
					continue
				}
				if err := os.WriteFile(meta, []byte("not xl.meta"), 0o600); err != nil {
					t.Fatal(err)
				}
				corrupted++
			}
			if corrupted < 13 {
				t.Fatalf("corrupted only %d xl.meta copies in pool 1", corrupted)
			}
			if _, err := z.serverPools[1].GetObjectInfo(t.Context(), bucket, "obj", ObjectOptions{NoLock: true}); err == nil || isErrObjectNotFound(err) {
				t.Fatalf("setup: pool 1 must be unreadable (not absent), got %v", err)
			}
			id, parts := r4Upload(t, z, bucket, "obj", 0)
			if _, err := z.CompleteMultipartUpload(t.Context(), bucket, "obj", id, parts, c.opts(stale)); err == nil {
				t.Fatal("completion passed its condition while the pool holding the newest copy was unreadable")
			}
		})
	}
}

// The streaming decoder of the legacy V2 header has no production caller
// today (unmarshalV uses UnmarshalMsg), but it is the same generated code
// with the same EcN/EcM reset; keep it from regressing on its own.
func TestR4LegacyV2HeaderDecodeMsgResetsEc(t *testing.T) {
	var legacy []byte
	legacy = msgp.AppendArrayHeader(legacy, 5)
	legacy = msgp.AppendBytes(legacy, make([]byte, 16))
	legacy = msgp.AppendInt64(legacy, time.Now().Unix())
	legacy = msgp.AppendBytes(legacy, make([]byte, 4))
	legacy = msgp.AppendUint8(legacy, uint8(ObjectType))
	legacy = msgp.AppendUint8(legacy, 0)

	header := xlMetaV2VersionHeader{EcN: 4, EcM: 8}
	x2 := xlMetaV2VersionHeaderV2{xlMetaV2VersionHeader: &header}
	if err := x2.DecodeMsg(msgp.NewReader(bytes.NewReader(legacy))); err != nil {
		t.Fatalf("legacy V2 header DecodeMsg failed: %v", err)
	}
	if header.EcN != 0 || header.EcM != 0 {
		t.Fatalf("stale erasure metadata leaked through DecodeMsg: EcN %d, EcM %d", header.EcN, header.EcM)
	}
}
