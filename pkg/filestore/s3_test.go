package filestore_test

import (
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func TestNewS3Checks(t *testing.T) {
	for _, cfg := range []filestore.S3Config{
		{Endpoint: "storage.example.com", Bucket: "b", AccessKey: "k", SecretKey: "s"},
		{Endpoint: "https://storage.example.com", AccessKey: "k", SecretKey: "s"},
		{Endpoint: "https://storage.example.com", Bucket: "b"},
		{Endpoint: "https://storage.example.com", Bucket: "b", AccessKey: "k", SecretKey: "s", Prefix: "../x"},
	} {
		if _, err := filestore.NewS3(cfg); err == nil {
			t.Errorf("%+v: accepted", cfg)
		}
	}
}
