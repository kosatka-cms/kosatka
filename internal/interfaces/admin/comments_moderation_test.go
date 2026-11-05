package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestCommentSettingsFromForm - the settings as the page sends them: the
// comments with the page unless "ajax"; the same text again off with its
// box unchecked; stop words a line each, blank ones left out; a value not
// among the choices refused.
func TestCommentSettingsFromForm(t *testing.T) {
	parse := func(v url.Values) (entity.CommentSettings, bool) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return commentSettingsFromForm(r)
	}
	form := url.Values{"new_author_checks": {"3"}, "links_limit": {"-1"}, "duplicate_hours": {"168"}, "spam_keep_days": {"0"},
		"stop_words": {" casino \r\n\r\nquick money\r\n"}, "stop_words_action": {"reject"}, "bot_trap": {"1"}, "bot_min_seconds": {"5"}}
	got, ok := parse(form)
	want := entity.CommentSettings{NewAuthorChecks: 3, LinksLimit: -1, StopWords: "casino\nquick money", StopWordsAction: entity.StopWordsReject,
		BotTrap: true, BotMinSeconds: 5, LoadWithPage: true}
	if !ok || got != want {
		t.Errorf("got %+v %v, want %+v", got, ok, want)
	}
	form.Set("comments_load", "ajax")
	if got, _ := parse(form); got.LoadWithPage {
		t.Error("the comments with the page, ajax chosen")
	}
	form.Set("duplicate", "1")
	if got, _ := parse(form); got.DuplicateHours != 168 {
		t.Errorf("duplicates refused for %d hours", got.DuplicateHours)
	}
	form.Set("links_limit", "100")
	if _, ok := parse(form); ok {
		t.Error("a links limit not among the choices accepted")
	}
}

// TestCommentReasonLabel - the reasons as the list shows them.
func TestCommentReasonLabel(t *testing.T) {
	ctx := context.Background()
	for reason, want := range map[string]string{"": "", "new_author": "new author", "marked": "marked by hand", "links:3": "3 links", "word:casino": "stop word \"casino\""} {
		if got := commentReasonLabel(ctx, reason); got != want {
			t.Errorf("%q: %q, want %q", reason, got, want)
		}
	}
}
