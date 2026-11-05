package app

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/config"
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

// privateStore - the private files' store of files.private_storage; nil
// for "local" (the directory). Its bucket (container) is reached with
// files.s3's endpoint and keys (files.azure's account), and must keep the
// files out of the public ones' reach: another bucket, or a prefix
// neither inside theirs nor holding it.
func privateStore(cfg config.FilesConfig) (filestore.Store, error) {
	switch storage := strings.ToLower(strings.TrimSpace(cfg.PrivateStorage)); storage {
	case "", "local":
		return nil, nil
	case "s3":
		s3, priv := cfg.S3, cfg.PrivateS3
		if publicStorage(cfg) == storage && priv.Bucket == s3.Bucket && prefixesOverlap(s3.Prefix, priv.Prefix) {
			return nil, fmt.Errorf("files.private_s3: the bucket %q and prefix %q reach the public files (files.s3): use another bucket or a prefix outside theirs", priv.Bucket, priv.Prefix)
		}
		store, err := filestore.NewS3(filestore.S3Config{
			Endpoint: s3.Endpoint, Region: s3.Region, Bucket: priv.Bucket, Prefix: priv.Prefix,
			AccessKey: s3.AccessKey, SecretKey: s3.SecretKey, PathStyle: s3.PathStyle,
		})
		if err != nil {
			return nil, fmt.Errorf("files.private_s3: %w", err)
		}
		return store, nil
	case "azure":
		az, priv := cfg.Azure, cfg.PrivateAzure
		if publicStorage(cfg) == storage && priv.Container == az.Container && prefixesOverlap(az.Prefix, priv.Prefix) {
			return nil, fmt.Errorf("files.private_azure: the container %q and prefix %q reach the public files (files.azure): use another container or a prefix outside theirs", priv.Container, priv.Prefix)
		}
		store, err := filestore.NewAzure(filestore.AzureConfig{
			Account: az.Account, Key: az.Key, Endpoint: az.Endpoint,
			Container: priv.Container, Prefix: priv.Prefix,
		})
		if err != nil {
			return nil, fmt.Errorf("files.private_azure: %w", err)
		}
		return store, nil
	default:
		return nil, fmt.Errorf("files.private_storage %q: local, s3 or azure", cfg.PrivateStorage)
	}
}

// publicStorage - files.storage, normalized ("" - local).
func publicStorage(cfg config.FilesConfig) string {
	if s := strings.ToLower(strings.TrimSpace(cfg.Storage)); s != "" {
		return s
	}
	return "local"
}

// prefixesOverlap reports whether one bucket prefix holds the other
// ("" - the whole bucket - holds every one).
func prefixesOverlap(a, b string) bool {
	a, b = strings.Trim(a, "/"), strings.Trim(b, "/")
	return a == "" || b == "" || a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// publicStore - the public files' store of files.storage and the address
// it serves them at itself; nil for "local" (the directory).
func publicStore(cfg config.FilesConfig) (filestore.Store, string, error) {
	var store filestore.Store
	var publicURL, setting string
	switch publicStorage(cfg) {
	case "local":
		return nil, "", nil
	case "s3":
		s3 := cfg.S3
		s, err := filestore.NewS3(filestore.S3Config{
			Endpoint: s3.Endpoint, Region: s3.Region, Bucket: s3.Bucket, Prefix: s3.Prefix,
			AccessKey: s3.AccessKey, SecretKey: s3.SecretKey, PathStyle: s3.PathStyle,
			CacheControl: storeCacheControl,
		})
		if err != nil {
			return nil, "", fmt.Errorf("files.s3: %w", err)
		}
		store, publicURL, setting = s, s3.PublicURL, "files.s3.public_url"
	case "azure":
		az := cfg.Azure
		s, err := filestore.NewAzure(filestore.AzureConfig{
			Account: az.Account, Key: az.Key, Endpoint: az.Endpoint,
			Container: az.Container, Prefix: az.Prefix,
			CacheControl: storeCacheControl,
		})
		if err != nil {
			return nil, "", fmt.Errorf("files.azure: %w", err)
		}
		store, publicURL, setting = s, az.PublicURL, "files.azure.public_url"
	default:
		return nil, "", fmt.Errorf("files.storage %q: local, s3 or azure", cfg.Storage)
	}
	publicURL = strings.TrimSpace(publicURL)
	if publicURL != "" {
		u, err := url.Parse(publicURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, "", fmt.Errorf("%s %q isn't an http(s) address", setting, publicURL)
		}
		publicURL = strings.TrimRight(publicURL, "/") + "/"
	}
	return store, publicURL, nil
}

// storageName - where the files of storage (files.storage,
// files.private_storage) are, for app config: "local" (the directory),
// or the bucket (s3://bucket/prefix, azure://container/prefix).
func storageName(storage, bucket, prefix string) string {
	switch s := strings.ToLower(strings.TrimSpace(storage)); s {
	case "s3", "azure":
		prefix = strings.Trim(prefix, "/")
		if prefix != "" {
			prefix += "/"
		}
		return s + "://" + bucket + "/" + prefix
	}
	return "local"
}
