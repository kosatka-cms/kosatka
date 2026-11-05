package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

const filesUsage = `usage:
  app files migrate [-private] [-to-local] [-derivatives] [-jobs 8] [-dry-run]
      copies the public files from the files directory (files.public_dir)
      to the store of files.storage (a bucket, a container); -private - the private
      files (files.private_dir, files.private_storage); -to-local - back`

// runFiles is "app files <command>": the public files' store.
func runFiles(args []string) int {
	if len(args) == 0 || args[0] != "migrate" {
		fmt.Fprintln(os.Stderr, filesUsage)
		return 2
	}
	return runFilesMigrate(args[1:], os.Stdout)
}

// derivativeRoots - the folders of the files the site makes again from
// the originals: image style derivatives and previews.
var derivativeRoots = []string{"styles/", "resize/"}

// runFilesMigrate is "app files migrate": the public files copied
// between the files directory and the configured store, those the
// destination has already (the same size) skipped - so it resumes - and
// nothing removed.
func runFilesMigrate(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("files migrate", flag.ContinueOnError)
	private := fs.Bool("private", false, "the private files (files.private_dir to files.private_storage)")
	toLocal := fs.Bool("to-local", false, "copy from the store back to the files directory")
	derivatives := fs.Bool("derivatives", false, "copy the image styles' derivatives and previews too (the site makes them again)")
	jobs := fs.Int("jobs", 8, "files copied at once")
	dryRun := fs.Bool("dry-run", false, "count what would be copied, copy nothing")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, filesUsage)
		return 2
	}
	cfg := loadConfig()
	if err := setupFiles(cfg.Files); err != nil {
		fmt.Fprintln(os.Stderr, "files migrate:", err)
		return 1
	}
	store, local := paths.PublicStore(), paths.PublicFilesDir()
	setting := "files.storage is local - set files.storage (s3 or azure) and its bucket first"
	if *private {
		store, local = paths.PrivateStore(), paths.PrivateFilesDir()
		setting = "files.private_storage is local - set files.private_storage (s3 or azure) and its bucket first"
	}
	if _, onDisk := store.(*filestore.Local); onDisk {
		fmt.Fprintln(os.Stderr, "files migrate:", setting)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	src, dst := filestore.Store(filestore.NewLocal(local)), store
	if *toLocal {
		src, dst = dst, src
	}
	_, err := migrateFiles(ctx, src, dst, filestore.CopyOptions{Jobs: *jobs, DryRun: *dryRun, Skip: migrateSkip(*derivatives || *private)}, out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "files migrate:", err)
		return 1
	}
	return 0
}

// migrateSkip - the files migrate leaves: dotfiles (never public
// files), and the derivatives unless asked for.
func migrateSkip(derivatives bool) func(filestore.Info) bool {
	return func(in filestore.Info) bool {
		if filestore.HasDotSegment(in.Key) {
			return true
		}
		if !derivatives {
			for _, root := range derivativeRoots {
				if strings.HasPrefix(in.Key, root) {
					return true
				}
			}
		}
		return false
	}
}

// progressEvery - files between two progress lines.
const progressEvery = 500

// migrateFiles copies src to dst (filestore.Copy), printing what it does
// to out: where from and to, the failures, progress, a summary.
func migrateFiles(ctx context.Context, src, dst filestore.Store, opts filestore.CopyOptions, out io.Writer) (filestore.CopyStats, error) {
	verb := "copied"
	if opts.DryRun {
		verb = "to copy"
	}
	_, _ = fmt.Fprintf(out, "files: %s -> %s\n", src, dst)
	n := 0
	stats, err := filestore.Copy(ctx, src, dst, opts, func(in filestore.Info, err error) {
		if err != nil {
			_, _ = fmt.Fprintf(out, "  failed: %s: %v\n", in.Key, err)
			return
		}
		if n++; n%progressEvery == 0 {
			_, _ = fmt.Fprintf(out, "  %s %d…\n", verb, n)
		}
	})
	_, _ = fmt.Fprintf(out, "files: %s %d (%s), already there %d, left out %d (derivatives, dotfiles), failed %d\n",
		verb, stats.Copied, service.FormatHumanFileSize(int(stats.Bytes)), stats.Existing, stats.Skipped, stats.Failed)
	return stats, err
}
