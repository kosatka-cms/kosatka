package imagestyle

import (
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// failedOriginals remembers the originals that can't be rendered
// (ErrNotRenderable), by key, size and modification time: anyone may
// request a derivative, and a failing original isn't decoded again on
// each request until it changes or failedFor passes. The zero value is
// ready to use.
type failedOriginals struct {
	mu   sync.Mutex
	seen map[failedOriginal]time.Time
}

type failedOriginal struct {
	key     string
	size    int64
	modTime int64
}

const (
	failedFor = 10 * time.Minute
	// failedKeep - entries at most; when full, the expired ones go, and
	// all of them when none has expired.
	failedKeep = 1024
)

func failedOriginalOf(info filestore.Info) failedOriginal {
	return failedOriginal{key: info.Key, size: info.Size, modTime: info.ModTime.UnixNano()}
}

func (f *failedOriginals) has(info filestore.Info) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	at, ok := f.seen[failedOriginalOf(info)]
	return ok && time.Since(at) < failedFor
}

func (f *failedOriginals) add(info filestore.Info) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seen == nil {
		f.seen = map[failedOriginal]time.Time{}
	}
	if len(f.seen) >= failedKeep {
		for k, at := range f.seen {
			if time.Since(at) >= failedFor {
				delete(f.seen, k)
			}
		}
		if len(f.seen) >= failedKeep {
			clear(f.seen)
		}
	}
	f.seen[failedOriginalOf(info)] = time.Now()
}
