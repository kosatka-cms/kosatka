// Package i18ntest holds a small Russian catalog for the engine's tests
// (pkg/i18n's format): the strings tests check in a language other than
// English - month names, number separators, plural forms, a few labels.
// The engine has no catalogs of its own: a site downloads its languages
// from the translations catalog.
package i18ntest

import "embed"

// FS - the test catalogs.
//
//go:embed *.json
var FS embed.FS
