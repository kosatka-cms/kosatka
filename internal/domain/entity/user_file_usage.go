package entity

import "context"

// The core's texts that can link to users' files (UserFileUsage.Source);
// modules add theirs (Module.FileSources).
const (
	UserFileSourceNodeField = "node_field"
	UserFileSourceComment   = "comment"
	UserFileSourceSignature = "signature"
	UserFileSourceUserField = "user_field"
	UserFileSourceBlock     = "block"
	UserFileSourceView      = "view"
)

// ModuleFileSourceByKey - a registered module's file source by its key.
func ModuleFileSourceByKey(source string) (ModuleFileSource, bool) {
	for _, m := range Modules {
		for _, s := range m.FileSources {
			if s.Source == source {
				return s, true
			}
		}
	}
	return ModuleFileSource{}, false
}

// UserFileUsage - a text (Source + EntityID) that links to a user's file
// (Path, relative to the files directory: "u42/docs/a.pdf").
type UserFileUsage struct {
	Path     string
	Source   string
	EntityID int
}

// UserFileRef - a place a file is used, for people: what it is and where
// to open it (URL empty when there is no page, e.g. a private message).
type UserFileRef struct {
	Source   string `json:"source"`
	EntityID int    `json:"entity_id"`
	Title    string `json:"title"`
	URL      string `json:"url,omitempty"`
}

// UserFileUsageRepository finds the texts that link to a user's file.
type UserFileUsageRepository interface {
	// ScanTextsFor calls fn for every text that mentions path.
	ScanTextsFor(ctx context.Context, path string, fn func(source string, entityID int, text string)) error
	// Describe gives each usage a title and a link.
	Describe(ctx context.Context, usages []UserFileUsage) ([]UserFileRef, error)
}
