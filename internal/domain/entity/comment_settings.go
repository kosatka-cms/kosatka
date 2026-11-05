package entity

import (
	"context"
	"strings"
)

// What a stop word in a comment does (CommentSettings.StopWordsAction).
const (
	StopWordsHold   = "hold"
	StopWordsReject = "reject"
)

// CommentSettings - what is done with suspicious comments (one row, the
// page /admin/config/comments).
type CommentSettings struct {
	// NewAuthorChecks - how many of an author's first comments wait for
	// approval (counting their published ones): 0 - none, -1 - all.
	NewAuthorChecks int
	// LinksLimit - a comment with more links waits for approval (-1 - no
	// limit).
	LinksLimit int
	// StopWords - one per line; a comment with one (in its text or a
	// link's address, case aside) waits or is refused: StopWordsAction.
	StopWords       string
	StopWordsAction string
	// DuplicateHours - the same text by the same author again within that
	// is refused (0 - allowed).
	DuplicateHours int
	// SpamKeepDays - spam is deleted that long after it was marked (0 -
	// kept).
	SpamKeepDays int
	// BotTrap - the forms (a comment's, registration, web forms) carry a
	// field hidden from people that bots fill, and the time they took: one
	// sent sooner than BotMinSeconds is a bot's.
	BotTrap       bool
	BotMinSeconds int
	// LoadWithPage - a content page has its comments in its HTML (search
	// engines read them, they show without JavaScript); false - they load
	// once the visitor scrolls to them (ajax).
	LoadWithPage bool
}

// DefaultCommentSettings - the settings until they are saved.
func DefaultCommentSettings() CommentSettings {
	return CommentSettings{NewAuthorChecks: 1, LinksLimit: 2, StopWordsAction: StopWordsHold, DuplicateHours: 24, SpamKeepDays: 30,
		BotTrap: true, BotMinSeconds: 3, LoadWithPage: true}
}

// StopWordList - StopWords' words, lower-cased, without blank lines.
func (s CommentSettings) StopWordList() []string {
	var out []string
	for _, line := range strings.Split(s.StopWords, "\n") {
		if w := strings.Join(strings.Fields(strings.ToLower(line)), " "); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// CommentSettingsRepository stores the comment settings row.
type CommentSettingsRepository interface {
	// Get - the settings; DefaultCommentSettings when none are saved.
	Get(ctx context.Context) (CommentSettings, error)
	Update(ctx context.Context, s CommentSettings) error
}
