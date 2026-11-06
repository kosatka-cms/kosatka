// Package filestorage - the storages a site's files can be kept in
// besides its directories (files.storage, files.private_storage): drivers
// that packages outside the engine register (modulekit.AddFileStorage),
// like database/sql's drivers. The site's main package imports them.
package filestorage

import (
	"fmt"
	"sort"
	"sync"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// Local - the storage of the site's directories (files.public_dir,
// files.private_dir): the engine's own, no driver.
const Local = "local"

// Driver - a storage the files can be kept in. A store that is a
// fmt.Stringer says where it keeps the files (e.g. s3://bucket/prefix/):
// app config shows it.
type Driver struct {
	// Name - files.storage's (files.private_storage's) value choosing it:
	// lower case, not "local".
	Name string
	// Public opens the store of the site's public files; publicURL - where
	// the store (or a CDN in front of it) serves them itself, an http(s)
	// address ("" - the site serves them from the store).
	Public func(c Config) (store filestore.Store, publicURL string, err error)
	// Private opens the store of the private files: one the public files'
	// address never reaches (c.PublicStorage tells whether they are in
	// this storage too). nil - the driver keeps no private files.
	Private func(c Config) (filestore.Store, error)
}

// Config - what a driver opens its store from.
type Config struct {
	// PublicStorage - the public files' storage (files.storage, "local"
	// when unset): a private store refuses to share their place.
	PublicStorage string
	// CacheControl - the Cache-Control of the public files a store writes
	// (they never change in place).
	CacheControl string
	// decode fills v from the config's files.<section>, then from the
	// environment.
	decode func(section string, v any) error
}

// NewConfig - a Config whose Decode is decode (the engine's config
// loader; tests).
func NewConfig(publicStorage, cacheControl string, decode func(section string, v any) error) Config {
	return Config{PublicStorage: publicStorage, CacheControl: cacheControl, decode: decode}
}

// Decode fills v (a pointer to a struct) from the config file's
// files.<section> (its yaml tags), then from the environment (its env
// tags win, as for the engine's own settings): a driver's settings, e.g.
// Decode("s3", &cfg). Keep secrets out of the file: yaml:"-" and an env
// tag.
func (c Config) Decode(section string, v any) error {
	if c.decode == nil {
		return fmt.Errorf("files.%s: no config to read", section)
	}
	return c.decode(section, v)
}

var (
	mu      sync.RWMutex
	drivers = map[string]Driver{}
)

// Register adds a driver (from the package's init). An empty name,
// "local", a name already taken or a driver without Public panics: it is
// a mistake in the binary, not in a site's settings.
func Register(d Driver) {
	mu.Lock()
	defer mu.Unlock()
	switch {
	case d.Name == "" || d.Name == Local:
		panic(fmt.Sprintf("filestorage: the storage name %q is not allowed", d.Name))
	case d.Public == nil:
		panic(fmt.Sprintf("filestorage: the storage %q has no Public", d.Name))
	}
	if _, ok := drivers[d.Name]; ok {
		panic(fmt.Sprintf("filestorage: the storage %q is registered twice", d.Name))
	}
	drivers[d.Name] = d
}

// Lookup - the driver named name.
func Lookup(name string) (Driver, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := drivers[name]
	return d, ok
}

// Names - the registered drivers' names, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(drivers))
	for n := range drivers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Unregister removes the driver named name (tests).
func Unregister(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(drivers, name)
}
