//go:build dbintegration

package filestore_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/filestore/filestoretest"
)

// s3Config - the test storage from KOSATKA_TEST_S3
// (http://<key>:<secret>@host:port/<bucket>, scripts/test_db.sh), its
// bucket made if needed; the test is skipped without it.
func s3Config(t *testing.T) filestore.S3Config {
	t.Helper()
	raw := os.Getenv("KOSATKA_TEST_S3")
	if raw == "" {
		t.Skip("KOSATKA_TEST_S3 not set (scripts/test_db.sh sets it)")
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		t.Fatalf("KOSATKA_TEST_S3 = %q: want http://key:secret@host:port/bucket", raw)
	}
	secret, _ := u.User.Password()
	cfg := filestore.S3Config{
		Endpoint:  u.Scheme + "://" + u.Host,
		Bucket:    strings.Trim(u.Path, "/"),
		AccessKey: u.User.Username(),
		SecretKey: secret,
		PathStyle: true,
	}
	client, err := minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: u.Scheme == "https"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if ok, err := client.BucketExists(ctx, cfg.Bucket); err != nil {
		t.Fatalf("the test bucket: %v", err)
	} else if !ok {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("making the test bucket: %v", err)
		}
	}
	return cfg
}

// TestS3 - the store's contract on an S3-compatible storage, each test
// under a prefix of its own in the shared bucket.
func TestS3(t *testing.T) {
	cfg := s3Config(t)
	filestoretest.Run(t, func(t *testing.T) filestore.Store {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		c := cfg
		c.Prefix = "test-" + hex.EncodeToString(b)
		s, err := filestore.NewS3(c)
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

// TestS3ContentHeaders - what the bucket serves itself: the file's media
// type, and a download for anything but images and PDF.
func TestS3ContentHeaders(t *testing.T) {
	cfg := s3Config(t)
	cfg.Prefix = "headers"
	cfg.CacheControl = "public, max-age=60"
	s, err := filestore.NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	u, _ := url.Parse(os.Getenv("KOSATKA_TEST_S3"))
	secret, _ := u.User.Password()
	client, err := minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(u.User.Username(), secret, ""), Secure: u.Scheme == "https"})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string][2]string{
		"a.jpg":   {"image/jpeg", ""},
		"doc.pdf": {"application/pdf", ""},
		"x.html":  {"text/html; charset=utf-8", "attachment"},
	} {
		if _, err := s.Put(ctx, key, strings.NewReader("x"), filestore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		oi, err := client.StatObject(ctx, cfg.Bucket, "headers/"+key, minio.StatObjectOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if oi.ContentType != want[0] || oi.Metadata.Get("Content-Disposition") != want[1] || oi.Metadata.Get("Cache-Control") != cfg.CacheControl {
			t.Errorf("%s: %q %q %q", key, oi.ContentType, oi.Metadata.Get("Content-Disposition"), oi.Metadata.Get("Cache-Control"))
		}
	}
	if s.String() != "s3://"+cfg.Bucket+"/headers/" {
		t.Errorf("String() = %q", s.String())
	}
	for _, key := range []string{"a.jpg", "doc.pdf", "x.html"} {
		_ = s.Delete(ctx, key)
	}
}

// TestCopyToS3 - files copied from a directory into the bucket, a second
// run finding them all there.
func TestCopyToS3(t *testing.T) {
	cfg := s3Config(t)
	cfg.Prefix = "copy"
	dst, err := filestore.NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	src := filestore.NewLocal(t.TempDir())
	for _, key := range []string{"a.jpg", "d/b.pdf", "d/e/c.txt"} {
		if _, err := src.Put(ctx, key, strings.NewReader("body of "+key), filestore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = dst.DeleteDir(context.Background(), "d"); _ = dst.Delete(context.Background(), "a.jpg") })
	if stats, err := filestore.Copy(ctx, src, dst, filestore.CopyOptions{}, nil); err != nil || stats.Copied != 3 {
		t.Fatalf("copy: %+v %v", stats, err)
	}
	if got, err := filestore.ReadFile(ctx, dst, "d/e/c.txt"); err != nil || string(got) != "body of d/e/c.txt" {
		t.Errorf("copied file = %q, %v", got, err)
	}
	if stats, err := filestore.Copy(ctx, src, dst, filestore.CopyOptions{}, nil); err != nil || stats.Existing != 3 || stats.Copied != 0 {
		t.Errorf("second run: %+v %v", stats, err)
	}
}
