package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/filestorage"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// namedStore - a store that says where it keeps the files.
type namedStore struct {
	filestore.Store
	name string
}

func (s namedStore) String() string { return s.name }

// fakeSettings - the test driver's section, files.fake.
type fakeSettings struct {
	Bucket    string `yaml:"bucket" env:"KOSATKA_TEST_FAKE_BUCKET"`
	PublicURL string `yaml:"public_url"`
}

// useFakeStorage registers the storage "fake" for the test: its stores
// are directories named after files.fake's bucket; seen - the configs
// its Private got.
func useFakeStorage(t *testing.T, private bool) *[]filestorage.Config {
	t.Helper()
	var seen []filestorage.Config
	open := func(c filestorage.Config) (filestore.Store, fakeSettings, error) {
		var s fakeSettings
		if err := c.Decode("fake", &s); err != nil {
			return nil, s, err
		}
		if s.Bucket == "" {
			return nil, s, errors.New("files.fake: no bucket")
		}
		return namedStore{filestore.NewLocal(t.TempDir()), "fake://" + s.Bucket + "/"}, s, nil
	}
	d := filestorage.Driver{Name: "fake", Public: func(c filestorage.Config) (filestore.Store, string, error) {
		store, s, err := open(c)
		return store, s.PublicURL, err
	}}
	if private {
		d.Private = func(c filestorage.Config) (filestore.Store, error) {
			seen = append(seen, c)
			store, _, err := open(c)
			return store, err
		}
	}
	filestorage.Register(d)
	t.Cleanup(func() { filestorage.Unregister("fake") })
	return &seen
}

// TestPublicStore - files.storage picks the store of a registered
// storage, from its own section; the public address gets a trailing
// slash, a bad one or an unknown storage is an error naming the setting.
func TestPublicStore(t *testing.T) {
	if s, url, err := publicStore(config.FilesConfig{}); s != nil || url != "" || err != nil {
		t.Errorf("local: %v %q %v", s, url, err)
	}
	useFakeStorage(t, false)
	section := func(s map[string]any) map[string]any { return map[string]any{"fake": s} }
	s, url, err := publicStore(config.FilesConfig{Storage: " FAKE ", Drivers: section(map[string]any{"bucket": "site", "public_url": "https://cdn.example.com/blog"})})
	if err != nil || s == nil || url != "https://cdn.example.com/blog/" {
		t.Fatalf("fake: %v %q %v", s, url, err)
	}
	t.Setenv("KOSATKA_TEST_FAKE_BUCKET", "from-env")
	if s, _, err := publicStore(config.FilesConfig{Storage: "fake", Drivers: section(map[string]any{"bucket": "site"})}); err != nil || s.(namedStore).name != "fake://from-env/" {
		t.Errorf("the environment doesn't win: %v %v", s, err)
	}
	for name, tc := range map[string]struct {
		cfg  config.FilesConfig
		want string
	}{
		"unknown storage": {config.FilesConfig{Storage: "ftp"}, `files.storage "ftp"`},
		"bad public url":  {config.FilesConfig{Storage: "fake", Drivers: section(map[string]any{"public_url": "cdn.example.com"})}, "isn't an http(s) address"},
	} {
		if _, _, err := publicStore(tc.cfg); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	t.Setenv("KOSATKA_TEST_FAKE_BUCKET", "")
	if _, _, err := publicStore(config.FilesConfig{Storage: "fake"}); err == nil || !strings.Contains(err.Error(), "files.fake") {
		t.Errorf("the driver's error: %v", err)
	}
}

// TestPrivateStore - files.private_storage picks the store of a
// registered storage, which learns where the public files are; a storage
// without private files or an unknown one is an error.
func TestPrivateStore(t *testing.T) {
	if s, err := privateStore(config.FilesConfig{}); s != nil || err != nil {
		t.Errorf("local: %v %v", s, err)
	}
	seen := useFakeStorage(t, true)
	cfg := config.FilesConfig{Storage: "fake", PrivateStorage: "fake", Drivers: map[string]any{"fake": map[string]any{"bucket": "site"}}}
	if s, err := privateStore(cfg); err != nil || s == nil || len(*seen) != 1 || (*seen)[0].PublicStorage != "fake" {
		t.Fatalf("fake: %v %v %+v", s, err, *seen)
	}
	cfg.Storage = ""
	if _, err := privateStore(cfg); err != nil || (*seen)[1].PublicStorage != filestorage.Local {
		t.Errorf("public files on disk: %v %+v", err, *seen)
	}
	if _, err := privateStore(config.FilesConfig{PrivateStorage: "ftp"}); err == nil || !strings.Contains(err.Error(), "files.private_storage") {
		t.Errorf("an unknown storage: %v", err)
	}
	filestorage.Unregister("fake")
	useFakeStorage(t, false)
	if _, err := privateStore(cfg); err == nil || !strings.Contains(err.Error(), "no private files") {
		t.Errorf("a storage without private files: %v", err)
	}
}

// TestStorageName - app config's storage: local, what the store says, or
// the storage's name when it can't be opened.
func TestStorageName(t *testing.T) {
	useFakeStorage(t, true)
	cfg := config.FilesConfig{Storage: "fake", Drivers: map[string]any{"fake": map[string]any{"bucket": "site"}}}
	info := configInfo(&config.Config{Files: cfg})
	if info.Files.Storage != "fake://site/" || info.Files.PrivateStorage != "local" {
		t.Errorf("storages = %q, %q", info.Files.Storage, info.Files.PrivateStorage)
	}
	cfg.PrivateStorage, cfg.Drivers = "fake", nil
	if info := configInfo(&config.Config{Files: cfg}); info.Files.Storage != "fake" || info.Files.PrivateStorage != "fake" {
		t.Errorf("not opened = %q, %q", info.Files.Storage, info.Files.PrivateStorage)
	}
}
