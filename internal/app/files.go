package app

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/filestorage"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// storeCacheControl - how long browsers and CDNs keep a file the bucket
// serves itself (as the site's own uploads: they don't change in place).
const storeCacheControl = "public, max-age=31536000"

// setupFiles sets where the public and private files are (paths) from
// the config: the directories, the public files' URL path, the stores.
func setupFiles(cfg config.FilesConfig) error {
	paths.SetPublicFiles(cfg.PublicDir, cfg.PublicURL)
	paths.SetPrivateFiles(cfg.PrivateDir)
	store, publicURL, err := publicStore(cfg)
	if err != nil {
		return err
	}
	private, err := privateStore(cfg)
	if err != nil {
		return err
	}
	paths.SetPublicStore(store, publicURL)
	paths.SetPrivateStore(private)
	return nil
}

// storageOf - the storage a setting (files.storage, files.private_storage)
// names, normalized ("" - local).
func storageOf(setting string) string {
	if s := strings.ToLower(strings.TrimSpace(setting)); s != "" {
		return s
	}
	return filestorage.Local
}

// storageDriver - the driver of storage (not local); setting - the
// config's key, for the error.
func storageDriver(setting, storage string) (filestorage.Driver, error) {
	d, ok := filestorage.Lookup(storage)
	if !ok {
		known := append([]string{filestorage.Local}, filestorage.Names()...)
		return d, fmt.Errorf("%s %q: this binary has no such storage (%s); a storage from its own repository is added to the site's binary like a module (app module add)", setting, storage, strings.Join(known, ", "))
	}
	return d, nil
}

// storageConfig - what a driver opens its store from.
func storageConfig(cfg config.FilesConfig) filestorage.Config {
	return filestorage.NewConfig(storageOf(cfg.Storage), storeCacheControl, cfg.DecodeDriver)
}

// privateStore - the private files' store of files.private_storage; nil
// for "local" (the directory).
func privateStore(cfg config.FilesConfig) (filestore.Store, error) {
	storage := storageOf(cfg.PrivateStorage)
	if storage == filestorage.Local {
		return nil, nil
	}
	d, err := storageDriver("files.private_storage", storage)
	if err != nil {
		return nil, err
	}
	if d.Private == nil {
		return nil, fmt.Errorf("files.private_storage %q: the storage keeps no private files", storage)
	}
	return d.Private(storageConfig(cfg))
}

// publicStore - the public files' store of files.storage and the address
// it serves them at itself; nil for "local" (the directory).
func publicStore(cfg config.FilesConfig) (filestore.Store, string, error) {
	storage := storageOf(cfg.Storage)
	if storage == filestorage.Local {
		return nil, "", nil
	}
	d, err := storageDriver("files.storage", storage)
	if err != nil {
		return nil, "", err
	}
	store, publicURL, err := d.Public(storageConfig(cfg))
	if err != nil {
		return nil, "", err
	}
	publicURL = strings.TrimSpace(publicURL)
	if publicURL != "" {
		u, err := url.Parse(publicURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, "", fmt.Errorf("files.storage %q: the public address %q isn't an http(s) address", storage, publicURL)
		}
		publicURL = strings.TrimRight(publicURL, "/") + "/"
	}
	return store, publicURL, nil
}

// storageName - where the files of a setting (files.storage,
// files.private_storage) are, for app config: "local" (the directory),
// what the store says (s3://bucket/prefix/) or the storage's name.
func storageName(storage string, open func() (filestore.Store, error)) string {
	if storage == filestorage.Local {
		return storage
	}
	if s, err := open(); err == nil {
		if str, ok := s.(fmt.Stringer); ok {
			return str.String()
		}
	}
	return storage
}
