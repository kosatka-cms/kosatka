package themes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/kosatka-cms/kosatka/internal/paths"
)

// Lock - the themes installed from their repositories (app theme
// add, internal/themeinstall), by directory name: themes/themes.lock, like
// go.sum or composer.lock. A theme not in it is one shipped with the
// engine or the site.
type Lock struct {
	Themes map[string]LockEntry `json:"themes"`
}

// LockEntry - an installed theme: Source - its repository (git
// URL), Version - the tag installed ("v1.2.0"), Commit - the tag's
// commit, Hash - its files' checksum (dirhash "h1:…", the minified
// files left out), checked by app theme verify.
type LockEntry struct {
	Source  string `json:"source"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Hash    string `json:"hash"`
}

// ReadLock - themes/themes.lock (empty when there is none).
func ReadLock() (Lock, error) {
	lock := Lock{Themes: map[string]LockEntry{}}
	p, err := paths.ThemesLockFile()
	if err != nil {
		return lock, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return lock, nil
	}
	if err != nil {
		return lock, err
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return lock, fmt.Errorf("parsing %s: %w", p, err)
	}
	if lock.Themes == nil {
		lock.Themes = map[string]LockEntry{}
	}
	return lock, nil
}
