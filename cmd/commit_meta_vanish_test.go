//go:build linux

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// The staged xl.meta disappears between its write and the commit rename
// inside the real xlStorage.RenameData. An overwrite of a non-inline object
// writes the previous xl.meta as a backup into the old data directory between
// those two steps. A FIFO with a full pipe buffer at that path parks every
// drive's RenameData in write(2); the test removes the staged xl.meta and only
// then drains the pipe.
func stagedMetaFifoOpeners(fifo string) int {
	n := 0
	fds, _ := os.ReadDir("/proc/self/fd")
	for _, fd := range fds {
		if l, err := os.Readlink("/proc/self/fd/" + fd.Name()); err == nil && l == fifo {
			n++
		}
	}
	return n
}

func stagedMetaParkAndVanish(t *testing.T, z *erasureServerPools, bucket, object string, stagedGlob func(drive string) string, op func() error) error {
	t.Helper()
	ctx := t.Context()
	disks := z.serverPools[0].sets[0].getDisks()

	type park struct {
		fifo, drive string
		f           *os.File
	}
	var parks []park
	for _, d := range disks {
		fi, err := d.ReadVersion(ctx, "", bucket, object, "", ReadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if fi.DataDir == "" {
			t.Fatal("setup: the first version must not be inline")
		}
		fifo := filepath.Join(d.Endpoint().Path, bucket, object, fi.DataDir, xlStorageFormatFileBackup)
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(fifo, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), 1031 /* F_SETPIPE_SZ */, 4096); errno != 0 {
			t.Fatal(errno)
		}
		chunk := make([]byte, 512)
		for {
			if _, err := syscall.Write(int(f.Fd()), chunk); err != nil {
				break // EAGAIN: the pipe is full
			}
		}
		parks = append(parks, park{fifo: fifo, drive: d.Endpoint().Path, f: f})
	}

	done := make(chan error, 1)
	go func() { done <- op() }()

	deadline := time.Now().Add(60 * time.Second)
	for _, p := range parks {
		for stagedMetaFifoOpeners(p.fifo) < 2 { // RenameData opened the backup and blocks in write(2)
			if time.Now().After(deadline) {
				t.Fatal("RenameData never reached the backup write on " + p.drive)
			}
			time.Sleep(5 * time.Millisecond)
		}
		staged, _ := filepath.Glob(stagedGlob(p.drive))
		if len(staged) != 1 {
			t.Fatalf("staged xl.meta: %d matches for %s", len(staged), stagedGlob(p.drive))
		}
		if err := os.Remove(staged[0]); err != nil {
			t.Fatal(err)
		}
	}
	buf := make([]byte, 4096)
	for _, p := range parks { // release: drain until the writer has closed
		for stagedMetaFifoOpeners(p.fifo) > 1 {
			if time.Now().After(deadline) {
				t.Fatal("RenameData did not finish the backup write on " + p.drive)
			}
			syscall.Read(int(p.f.Fd()), buf)
			time.Sleep(time.Millisecond)
		}
	}
	select {
	case err := <-done:
		return err
	case <-time.After(60 * time.Second):
		t.Fatal("operation did not return after the drives were released")
	}
	return nil
}

func stagedMetaSetup(t *testing.T) (*erasureServerPools, string) {
	t.Helper()
	objLayer, dirs, err := prepareErasure16(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { objLayer.Shutdown(context.Background()); removeRoots(dirs) })
	z := objLayer.(*erasureServerPools)
	if err = z.MakeBucket(t.Context(), "stagedmeta", MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	return z, "stagedmeta"
}

var stagedMetaPayload = bytes.Repeat([]byte("x"), 8<<20)

func stagedMetaPut(t *testing.T, z *erasureServerPools, bucket, object string) error {
	_, err := z.PutObject(t.Context(), bucket, object,
		mustGetPutObjReader(t, bytes.NewReader(stagedMetaPayload), int64(len(stagedMetaPayload)), "", ""), ObjectOptions{})
	return err
}

func stagedMetaAssertServerError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("the commit cannot succeed after the staged xl.meta vanished")
	}
	var ib IncompleteBody
	if errors.As(err, &ib) {
		t.Fatalf("reported as a client disconnect (IncompleteBody, 400): %v", err)
	}
	if isErrObjectNotFound(err) {
		t.Fatalf("reported as an absent object (NoSuchKey, 404): %v", err)
	}
	var wq InsufficientWriteQuorum
	if !errors.As(err, &wq) {
		t.Fatalf("expected InsufficientWriteQuorum (503), got %T %v", err, err)
	}
}

func TestPutObjectStagedMetaVanishes(t *testing.T) {
	z, bucket := stagedMetaSetup(t)
	if err := stagedMetaPut(t, z, bucket, "obj"); err != nil {
		t.Fatal(err)
	}
	err := stagedMetaParkAndVanish(t, z, bucket, "obj",
		func(drive string) string { return filepath.Join(drive, minioMetaTmpBucket, "*", xlStorageFormatFile) },
		func() error { return stagedMetaPut(t, z, bucket, "obj") })
	stagedMetaAssertServerError(t, err)
}

func TestCompleteMultipartStagedMetaVanishes(t *testing.T) {
	z, bucket := stagedMetaSetup(t)
	if err := stagedMetaPut(t, z, bucket, "obj"); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	mp, err := z.NewMultipartUpload(ctx, bucket, "obj", ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pi, err := z.PutObjectPart(ctx, bucket, "obj", mp.UploadID, 1,
		mustGetPutObjReader(t, bytes.NewReader(stagedMetaPayload), int64(len(stagedMetaPayload)), "", ""), ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = stagedMetaParkAndVanish(t, z, bucket, "obj",
		func(drive string) string {
			return filepath.Join(drive, minioMetaMultipartBucket, "*", "*", xlStorageFormatFile)
		},
		func() error {
			_, err := z.CompleteMultipartUpload(ctx, bucket, "obj", mp.UploadID, []CompletePart{{PartNumber: 1, ETag: pi.ETag}}, ObjectOptions{})
			return err
		})
	stagedMetaAssertServerError(t, err)
}
