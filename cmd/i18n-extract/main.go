// Command i18n-extract writes the list of the code's interface strings
// (translations/sources/strings.json, embedded: the admin's interface
// strings page, the checks of a downloaded language):
//
//	go run ./cmd/i18n-extract -sources
//
// The translations themselves are in the translations repository
// (kosatka-cms/translations), checked against this list there
// (app translations index).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	// A subtheme's base theme (paths.ThemeChain) comes from its manifest.
	_ "github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/i18nextract"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

func main() {
	root := flag.String("root", ".", "repository root")
	sources := flag.Bool("sources", false, "write the source strings' list (translations/sources/strings.json)")
	flag.Parse()
	if !*sources {
		flag.Usage()
		os.Exit(2)
	}

	paths.SetThemesDir(filepath.Join(*root, "themes"))
	msgs, err := i18nextract.Extract(*root)
	if err == nil {
		var b []byte
		if b, err = i18nextract.SourcesJSON(i18nextract.Sources(msgs)); err == nil {
			err = os.WriteFile(filepath.Join(*root, "translations", "sources", "strings.json"), b, 0o644)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
