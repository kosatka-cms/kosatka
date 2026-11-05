package paths

import (
	"strings"
	"sync/atomic"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// Public files (uploads, image style derivatives): the directory on disk
// and the URL path it is served at - config files.public_dir /
// files.public_url. Content stores the
// files' URLs, so a site keeps its URL path once it has content.
const (
	DefaultPublicFilesDir = "./web/uploads"
	DefaultPublicFilesURL = "/uploads/"
)

type publicFiles struct{ dir, url string }

var publicFilesConfig atomic.Value

// SetPublicFiles sets the public files directory and URL path; called once
// at startup from the config. Empty values keep the defaults; the URL
// path gets its leading and trailing slashes.
func SetPublicFiles(dir, url string) {
	dir = strings.TrimRight(strings.TrimSpace(dir), "/")
	if dir == "" {
		dir = DefaultPublicFilesDir
	}
	url = strings.Trim(strings.TrimSpace(url), "/")
	if url == "" {
		url = DefaultPublicFilesURL
	} else {
		url = "/" + url + "/"
	}
	publicFilesConfig.Store(publicFiles{dir: dir, url: url})
}

func currentPublicFiles() publicFiles {
	if c, ok := publicFilesConfig.Load().(publicFiles); ok {
		return c
	}
	return publicFiles{dir: DefaultPublicFilesDir, url: DefaultPublicFilesURL}
}

// PublicFilesDir - the public files directory on disk.
func PublicFilesDir() string { return currentPublicFiles().dir }

// PublicFilesURL - the URL path of the public files directory, with the
// leading and trailing slash (e.g. "/uploads/").
func PublicFilesURL() string { return currentPublicFiles().url }

// PublicFilesRel - the URL path without its leading slash
// ("uploads/"), as stored in some relative values.
func PublicFilesRel() string { return strings.TrimPrefix(PublicFilesURL(), "/") }

type store struct {
	s filestore.Store
	// url - where the store serves the files itself ("" - it doesn't).
	url string
}

var publicStore atomic.Pointer[store]

// SetPublicStore sets the store the public files are kept in and the
// address it serves them at itself ("" - the site serves them); called
// once at startup from the config (nil - the directory PublicFilesDir).
func SetPublicStore(s filestore.Store, url string) {
	if s == nil {
		publicStore.Store(nil)
		return
	}
	publicStore.Store(&store{s: s, url: url})
}

// PublicStore - the store of the public files: the one set, else the
// directory PublicFilesDir on disk. Keys are paths relative to
// PublicFilesURL.
func PublicStore() filestore.Store {
	if s := publicStore.Load(); s != nil {
		return s.s
	}
	return filestore.NewLocal(PublicFilesDir())
}

// PublicStoreURL - the address (ending in "/") the public files' store
// serves them at itself, e.g. a bucket's CDN: the site redirects its files'
// addresses (PublicFilesURL + key) there. "" - the site serves them.
func PublicStoreURL() string {
	if s := publicStore.Load(); s != nil {
		return s.url
	}
	return ""
}

// PrivateFilesURL - the URL path the site serves the private files at,
// after checking the viewer may see a content item a file is attached
// to: PrivateFilesURL + its key.
const PrivateFilesURL = "/files/private/"

// DefaultPrivateFilesDir - the private files directory without a config.
const DefaultPrivateFilesDir = "./private"

var (
	privateFilesDir atomic.Value
	privateStore    atomic.Pointer[filestore.Store]
)

// SetPrivateStore sets the store the private files are kept in; called
// once at startup from the config (nil - the directory PrivateFilesDir).
func SetPrivateStore(s filestore.Store) {
	if s == nil {
		privateStore.Store(nil)
		return
	}
	privateStore.Store(&s)
}

// PrivateStore - the store of the private files: the one set, else the
// directory PrivateFilesDir on disk.
func PrivateStore() filestore.Store {
	if s := privateStore.Load(); s != nil {
		return *s
	}
	return filestore.NewLocal(PrivateFilesDir())
}

// SetPrivateFiles sets the private files directory (config
// files.private_dir; "" - the default): files the application serves only
// after a permission check, never from the web directory.
func SetPrivateFiles(dir string) {
	dir = strings.TrimRight(strings.TrimSpace(dir), "/")
	if dir == "" {
		dir = DefaultPrivateFilesDir
	}
	privateFilesDir.Store(dir)
}

// PrivateFilesDir - the private files directory.
func PrivateFilesDir() string {
	if d, ok := privateFilesDir.Load().(string); ok {
		return d
	}
	return DefaultPrivateFilesDir
}
