package filestore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// CopyOptions - what Copy copies and how.
type CopyOptions struct {
	// Skip - files not to copy (nil - none).
	Skip func(Info) bool
	// Jobs - files copied at once (0 - 4).
	Jobs int
	// DryRun - count what would be copied, copy nothing.
	DryRun bool
}

// CopyStats - what Copy did.
type CopyStats struct {
	// Copied - files copied (with DryRun: to copy), Bytes - their size.
	Copied int
	Bytes  int64
	// Existing - files the destination has already, of the same size.
	Existing int
	// Skipped - files CopyOptions.Skip left out.
	Skipped int
	// Failed - files that couldn't be copied (each reported).
	Failed int
}

// Copy copies every file of src to the same key in dst, except those the
// destination has already with the same size: a copy cut short is
// resumed by running it again. It never removes anything. report (nil -
// none) is called after each file copied or failed, from the copying
// goroutines one at a time.
func Copy(ctx context.Context, src, dst Store, opts CopyOptions, report func(Info, error)) (CopyStats, error) {
	var stats CopyStats
	var files []Info
	if err := src.Walk(ctx, "", func(in Info) error {
		if opts.Skip != nil && opts.Skip(in) {
			stats.Skipped++
			return nil
		}
		files = append(files, in)
		return nil
	}); err != nil {
		return stats, fmt.Errorf("listing the files: %w", err)
	}

	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = 4
	}
	var mu sync.Mutex
	done := func(in Info, existing bool, err error) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err != nil:
			stats.Failed++
		case existing:
			stats.Existing++
			return
		default:
			stats.Copied++
			stats.Bytes += in.Size
		}
		if report != nil {
			report(in, err)
		}
	}
	work := make(chan Info)
	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for in := range work {
				existing, err := copyFile(ctx, src, dst, in, opts.DryRun)
				done(in, existing, err)
			}
		}()
	}
	for _, in := range files {
		if ctx.Err() != nil {
			break
		}
		work <- in
	}
	close(work)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return stats, err
	}
	if stats.Failed > 0 {
		return stats, fmt.Errorf("%d of %d files not copied", stats.Failed, len(files))
	}
	return stats, nil
}

// copyFile copies one file unless dst has it with the same size
// (existing).
func copyFile(ctx context.Context, src, dst Store, in Info, dryRun bool) (existing bool, err error) {
	if have, err := dst.Stat(ctx, in.Key); err == nil && !have.IsDir && have.Size == in.Size {
		return true, nil
	} else if err != nil && !errors.Is(err, ErrNotExist) {
		return false, err
	}
	if dryRun {
		return false, nil
	}
	f, _, err := src.Open(ctx, in.Key)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	if _, err := dst.Put(ctx, in.Key, f, PutOptions{}); err != nil {
		return false, err
	}
	return false, nil
}

// HasDotSegment reports whether a key has an element starting with a dot
// (.htaccess, a store's temporary file): never a public file.
func HasDotSegment(key string) bool {
	for _, seg := range strings.Split(key, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}
