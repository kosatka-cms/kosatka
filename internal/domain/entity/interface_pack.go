package entity

import (
	"context"
	"time"
)

// InterfacePack - a language's interface translation downloaded from the
// translations catalog: its language, the catalog's version and the
// file's checksum, how many of its strings were kept, whether the
// catalog marks it a draft (machine translated), when it was installed.
type InterfacePack struct {
	Lang, Version, SHA256 string
	Strings               int
	Draft                 bool
	InstalledAt           time.Time
}

// InterfacePackRepository - the downloaded interface translations.
type InterfacePackRepository interface {
	// Packs - the installed ones, by language.
	Packs(ctx context.Context) ([]InterfacePack, error)
	// Strings - the strings of every installed one.
	Strings(ctx context.Context) ([]InterfaceString, error)
	// Install writes a language's pack and its strings in place of what
	// the language had.
	Install(ctx context.Context, p InterfacePack, strings []InterfaceString) error
	// Delete removes a language's pack and its strings.
	Delete(ctx context.Context, lang string) error
}
