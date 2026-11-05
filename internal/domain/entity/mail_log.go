package entity

import (
	"context"
	"time"
)

// What came of an email (MailLogEntry.Status).
const (
	// MailStatusSent - the transport took it.
	MailStatusSent = "sent"
	// MailStatusFailed - the transport refused it (MailLogEntry.Error).
	MailStatusFailed = "failed"
	// MailStatusUnsent - email is off or not configured: nothing was sent.
	MailStatusUnsent = "unsent"
)

// MailLogEntry - an email the site sent or tried to (the Mail page's
// log): when, which of the site's emails (Key; "" - a message of no
// template, such as a form's notice), to whom, its subject, what came of
// it. Never its text.
type MailLogEntry struct {
	ID      int
	SentAt  time.Time
	Key     string
	To      string
	Subject string
	Status  string
	Error   string
}

// MailLogFilter - the log's entries asked for: To - a part of the
// address, Key and Status - exactly ("" - any), newest first.
type MailLogFilter struct {
	To     string
	Key    string
	Status string
	Limit  int
	Offset int
}

// MailLogRepository keeps the mail log.
type MailLogRepository interface {
	Add(ctx context.Context, e MailLogEntry) error
	// List - the entries f asks for, a page of them, and how many there are.
	List(ctx context.Context, f MailLogFilter) ([]MailLogEntry, int, error)
	// LastSent - when each email (by key) was last sent.
	LastSent(ctx context.Context) (map[string]time.Time, error)
	// DeleteBefore removes the entries older than t.
	DeleteBefore(ctx context.Context, t time.Time) error
}
