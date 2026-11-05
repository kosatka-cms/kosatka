package app

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
)

// TestPublicStore - files.storage picks the store; an S3 one checks its
// settings and gets its public address with a trailing slash.
func TestPublicStore(t *testing.T) {
	if s, url, err := publicStore(config.FilesConfig{}); s != nil || url != "" || err != nil {
		t.Errorf("local: %v %q %v", s, url, err)
	}
	s3 := config.S3Config{Endpoint: "https://storage.example.com", Bucket: "site", Prefix: "/blog/", AccessKey: "k", SecretKey: "s", PublicURL: "https://cdn.example.com/blog"}
	s, url, err := publicStore(config.FilesConfig{Storage: "S3", S3: s3})
	if err != nil || s == nil || url != "https://cdn.example.com/blog/" {
		t.Fatalf("s3: %v %q %v", s, url, err)
	}
	if got := storageName("s3", s3.Bucket, s3.Prefix); got != "s3://site/blog/" || got != s.(interface{ String() string }).String() {
		t.Errorf("storageName = %q, store %v", got, s)
	}
	for name, cfg := range map[string]config.FilesConfig{
		"unknown storage": {Storage: "ftp"},
		"no keys":         {Storage: "s3", S3: config.S3Config{Endpoint: s3.Endpoint, Bucket: "b"}},
		"bad public url":  {Storage: "s3", S3: func() config.S3Config { c := s3; c.PublicURL = "cdn.example.com"; return c }()},
	} {
		if _, _, err := publicStore(cfg); err == nil || !strings.Contains(err.Error(), "files.") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestPrivateStore - the private files' bucket is reached with files.s3's
// connection and never where the public files' address reaches.
func TestPrivateStore(t *testing.T) {
	if s, err := privateStore(config.FilesConfig{}); s != nil || err != nil {
		t.Errorf("local: %v %v", s, err)
	}
	s3 := config.S3Config{Endpoint: "https://storage.example.com", Bucket: "site", Prefix: "public", AccessKey: "k", SecretKey: "s"}
	for name, tc := range map[string]struct {
		storage string
		priv    config.PrivateS3Config
		ok      bool
	}{
		"another bucket":           {"s3", config.PrivateS3Config{Bucket: "site-private"}, true},
		"a prefix beside":          {"s3", config.PrivateS3Config{Bucket: "site", Prefix: "private"}, true},
		"public files on disk":     {"local", config.PrivateS3Config{Bucket: "site"}, true},
		"the same prefix":          {"s3", config.PrivateS3Config{Bucket: "site", Prefix: "/public/"}, false},
		"inside the public prefix": {"s3", config.PrivateS3Config{Bucket: "site", Prefix: "public/forms"}, false},
		"the whole bucket":         {"s3", config.PrivateS3Config{Bucket: "site"}, false},
		"no bucket":                {"s3", config.PrivateS3Config{}, false},
	} {
		s, err := privateStore(config.FilesConfig{Storage: tc.storage, S3: s3, PrivateStorage: "s3", PrivateS3: tc.priv})
		if (err == nil) != tc.ok || (tc.ok && s == nil) {
			t.Errorf("%s: %v %v", name, s, err)
		}
	}
	if _, err := privateStore(config.FilesConfig{PrivateStorage: "ftp"}); err == nil {
		t.Error("an unknown storage accepted")
	}
}

// TestAzureStores - files.storage and files.private_storage "azure": the
// container's store, its public address, and no private container where
// the public files' address reaches.
func TestAzureStores(t *testing.T) {
	az := config.AzureConfig{Account: "acc", Key: "a2V5", Container: "site", Prefix: "public", PublicURL: "https://cdn.example.com/site"}
	s, url, err := publicStore(config.FilesConfig{Storage: "azure", Azure: az})
	if err != nil || s == nil || url != "https://cdn.example.com/site/" || s.(interface{ String() string }).String() != "azure://site/public/" {
		t.Fatalf("public: %v %q %v", s, url, err)
	}
	for name, tc := range map[string]struct {
		priv config.PrivateAzureConfig
		ok   bool
	}{
		"another container": {config.PrivateAzureConfig{Container: "site-private"}, true},
		"a prefix beside":   {config.PrivateAzureConfig{Container: "site", Prefix: "private"}, true},
		"the same prefix":   {config.PrivateAzureConfig{Container: "site", Prefix: "public"}, false},
		"no container":      {config.PrivateAzureConfig{}, false},
	} {
		s, err := privateStore(config.FilesConfig{Storage: "azure", Azure: az, PrivateStorage: "azure", PrivateAzure: tc.priv})
		if (err == nil) != tc.ok || (tc.ok && s == nil) {
			t.Errorf("%s: %v %v", name, s, err)
		}
	}
	if got := storageName("azure", "site", "/public/"); got != "azure://site/public/" {
		t.Errorf("storageName = %q", got)
	}
}
