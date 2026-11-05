package entity

import (
	"context"
	"time"
)

// InterfaceString - an interface string's translation the site keeps
// itself, over the catalogs (pkg/i18n.Entry): changed in the admin. Lang
// - its language (the source language too: the site rewords English),
// Context and Source - the string (a plural string's singular), Text or
// the plural Forms.
type InterfaceString struct {
	Lang, Context, Source string
	Text                  string
	Forms                 []string
	UpdatedAt             time.Time
}

// InterfaceStringRepository - the site's own interface translations.
type InterfaceStringRepository interface {
	// All - every one, of every language.
	All(ctx context.Context) ([]InterfaceString, error)
	// Save writes one (or rewrites it).
	Save(ctx context.Context, s InterfaceString) error
	// Delete removes one: the catalogs' translation is back.
	Delete(ctx context.Context, lang, msgctx, source string) error
}
