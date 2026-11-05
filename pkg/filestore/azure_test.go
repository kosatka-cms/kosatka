package filestore_test

import (
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func TestNewAzureChecks(t *testing.T) {
	for _, cfg := range []filestore.AzureConfig{
		{Container: "c", Key: "a2V5"},
		{Account: "acc", Key: "a2V5"},
		{Account: "acc", Key: "a2V5", Container: "c", Endpoint: "storage.example.com"},
		{Account: "acc", Key: "not base64!", Container: "c"},
		{Account: "acc", Key: "a2V5", Container: "c", Prefix: "../x"},
	} {
		if _, err := filestore.NewAzure(cfg); err == nil {
			t.Errorf("%+v: accepted", cfg)
		}
	}
	s, err := filestore.NewAzure(filestore.AzureConfig{Account: "acc", Key: "a2V5", Container: "site", Prefix: "/blog/"})
	if err != nil || s.String() != "azure://site/blog/" {
		t.Errorf("%v %v", s, err)
	}
}
