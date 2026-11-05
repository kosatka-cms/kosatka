package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

const searchUsage = `usage:
  app search reindex
      writes the search index again: the words of every content item's
      title and text fields (the site writes them as content is saved;
      this is for content changed past the site, or an index to mend)`

// runSearch is "app search <command>": the search index.
func runSearch(args []string) int {
	if len(args) != 1 || args[0] != "reindex" {
		fmt.Fprintln(os.Stderr, searchUsage)
		return 2
	}
	return runSearchReindex(os.Stdout)
}

// runSearchReindex is "app search reindex" (database.RebuildSearchIndex).
func runSearchReindex(out io.Writer) int {
	cfg := loadConfig()
	db, drv, err := NewEntConn(&cfg.DB)
	if err != nil {
		fmt.Fprintln(os.Stderr, "search reindex:", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	n, err := database.RebuildSearchIndex(context.Background(), generated.NewClient(generated.Driver(drv)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "search reindex:", err)
		return 1
	}
	_, _ = fmt.Fprintf(out, "search index: %d content items\n", n)
	return 0
}
