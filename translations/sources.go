package translations

import (
	_ "embed"
)

// Sources - the interface strings of the code (pkg/i18n.Source, JSON):
// the engine's, the modules' and the bundled themes', written by
// go run ./cmd/i18n-extract -sources; TestSourcesUpToDate checks it.
//
//go:embed sources/strings.json
var Sources []byte
