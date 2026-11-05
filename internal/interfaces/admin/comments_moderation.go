package admin

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// commentsPath - the admin's comments.
const commentsPath = "/admin/content/comment"

// commentTab - a status tab of the comments: its key (the status query),
// label and count.
type commentTab struct {
	Key, Label string
	Status     int8
	Count      int
	Current    bool
}

// commentStatusKeys - the tabs' keys by status, in their order.
var commentStatusKeys = []struct {
	Key    string
	Label  string
	Status int8
}{
	{"published", i18n.N("Published"), entity.CommentPublished},
	{"pending", i18n.N("Waiting for approval"), entity.CommentPending},
	{"spam", i18n.N("Spam"), entity.CommentSpam},
}

// commentTabs - the tabs with their counts; current - the one of key, else
// the waiting comments when there are any, else the published.
func (c *Controller) commentTabs(ctx context.Context, key string) ([]commentTab, commentTab, error) {
	tabs := make([]commentTab, len(commentStatusKeys))
	for i, k := range commentStatusKeys {
		status := k.Status
		n, err := c.commentsRepo.TotalCount(ctx, entity.CommentsFilter{Status: &status})
		if err != nil {
			return nil, commentTab{}, err
		}
		tabs[i] = commentTab{Key: k.Key, Label: i18n.T(ctx, k.Label), Status: k.Status, Count: n}
	}
	current := 0
	switch {
	case key != "":
		for i, t := range tabs {
			if t.Key == key {
				current = i
			}
		}
	case tabs[1].Count > 0:
		current = 1
	}
	tabs[current].Current = true
	return tabs, tabs[current], nil
}

// adminCommentRow - a comment of the admin's list: why it waits or is
// spam (Reason, translated), how many published comments its author has.
type adminCommentRow struct {
	entity.Comment
	Reason          string
	AuthorPublished int
}

// commentReasonLabel - reason (entity.CommentReason*) as shown.
func commentReasonLabel(ctx context.Context, reason string) string {
	switch reason {
	case "":
		return ""
	case entity.CommentReasonNewAuthor:
		return i18n.T(ctx, "new author")
	case entity.CommentReasonMarked:
		return i18n.T(ctx, "marked by hand")
	}
	switch code, detail := entity.SplitCommentReason(reason); code {
	case entity.CommentReasonLinks:
		n, _ := strconv.Atoi(detail)
		return i18n.Plural(ctx, n, "@count link", "@count links")
	case entity.CommentReasonStopWord:
		return i18n.T(ctx, "stop word \"@word\"", "@word", detail)
	case entity.CommentReasonReported:
		n, _ := strconv.Atoi(detail)
		return i18n.Plural(ctx, n, "@count complaint", "@count complaints")
	}
	return reason
}

// commentRows - comments as the list shows them; the authors' published
// counts only for a tab but the published one.
func (c *Controller) commentRows(ctx context.Context, comments entity.Comments, tab commentTab) []adminCommentRow {
	rows := make([]adminCommentRow, len(comments))
	counts := map[int]int{}
	published := entity.CommentPublished
	for i, cm := range comments {
		rows[i] = adminCommentRow{Comment: cm, Reason: commentReasonLabel(ctx, cm.HoldReason)}
		if tab.Status == entity.CommentPublished {
			continue
		}
		n, ok := counts[cm.CreatedBy]
		if !ok {
			var err error
			if n, err = c.commentsRepo.TotalCount(ctx, entity.CommentsFilter{CreatedBy: &cm.CreatedBy, Status: &published}); err != nil {
				service.LogFallback(c.logger, err, "counting an author's published comments (0 shown)", "user_id", cm.CreatedBy)
			}
			counts[cm.CreatedBy] = n
		}
		rows[i].AuthorPublished = n
	}
	return rows
}

// CommentAction handles POST /admin/content/comment/{id}/{action}: a
// comment approved or marked as spam from its row ("administer
// comments"), then back to the list (the form's "back").
func (c *Controller) CommentAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if user, _ := c.GetCurrentUser(r); !c.authService.HasPermission(ctx, user, service.PermAdministerComments) {
		c.RenderForbidden(w, r)
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		c.RenderNotFound(w, r)
		return
	}
	switch r.PathValue("action") {
	case "approve":
		err = c.CommentsService().Approve(ctx, id)
	case "spam":
		err = c.CommentsService().MarkSpam(ctx, id)
	default:
		c.RenderNotFound(w, r)
		return
	}
	back := service.SafeRedirectPath(r.FormValue("back"))
	if back == "" {
		back = commentsPath
	}
	if err != nil {
		c.LogFailure(err, "changing a comment's status", "comment_id", id)
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// spamAndBlock marks comment id as spam and blocks its author, unless they
// administer comments or are user: false - the author is kept.
func (c *Controller) spamAndBlock(ctx context.Context, user *entity.User, id int) (bool, error) {
	cm, err := c.commentsRepo.ByID(ctx, id)
	if err != nil {
		return false, err
	}
	if err := c.CommentsService().MarkSpam(ctx, id); err != nil {
		return false, err
	}
	author, err := c.userRepo.ByID(ctx, cm.CreatedBy)
	if err != nil {
		return false, err
	}
	if author == nil || (user != nil && author.ID == user.ID) || c.authService.HasPermission(ctx, author, service.PermAdministerComments) ||
		author.Status == entity.UserStatusBlocked {
		return false, nil
	}
	if _, errs := c.adminBulkService.BulkUpdateUserStatus(ctx, []int{author.ID}, entity.UserStatusBlocked); len(errs) > 0 {
		return false, i18n.Errorf("Blocking the author: @error", "@error", errs[0])
	}
	return true, nil
}

// commentSettingsPath - the page of what is done with suspicious
// comments.
const commentSettingsPath = "/admin/config/comments"

// The settings' choices (entity.CommentSettings): how many of a new
// author's comments wait for approval, the links allowed, how long the
// same text is refused (hours), how long spam is kept (days).
var (
	newAuthorChoices  = []int{1, 3, -1, 0}
	linksChoices      = []int{0, 1, 2, 3, 5, -1}
	duplicateChoices  = []int{1, 24, 168}
	spamKeepChoices   = []int{7, 30, 90, 0}
	botSecondsChoices = []int{2, 3, 5, 10}
)

// settingOption - a choice of a setting as the page shows it.
type settingOption struct {
	Value int
	Label string
	On    bool
}

// settingOptions - choices with their labels, current chosen.
func settingOptions(choices []int, current int, label func(int) string) []settingOption {
	out := make([]settingOption, len(choices))
	for i, v := range choices {
		out[i] = settingOption{Value: v, Label: label(v), On: v == current}
	}
	return out
}

// formChoice - the form's field name as one of choices (false - another).
func formChoice(r *http.Request, name string, choices []int) (int, bool) {
	n, err := strconv.Atoi(r.FormValue(name))
	return n, err == nil && slices.Contains(choices, n)
}

// commentSettingsFromForm - the settings the page sent (false - a value
// not among its choices).
func commentSettingsFromForm(r *http.Request) (entity.CommentSettings, bool) {
	var s entity.CommentSettings
	var ok [5]bool
	s.NewAuthorChecks, ok[0] = formChoice(r, "new_author_checks", newAuthorChoices)
	s.LinksLimit, ok[1] = formChoice(r, "links_limit", linksChoices)
	s.DuplicateHours, ok[2] = formChoice(r, "duplicate_hours", duplicateChoices)
	s.SpamKeepDays, ok[3] = formChoice(r, "spam_keep_days", spamKeepChoices)
	s.BotMinSeconds, ok[4] = formChoice(r, "bot_min_seconds", botSecondsChoices)
	s.BotTrap = r.FormValue("bot_trap") == "1"
	s.LoadWithPage = r.FormValue("comments_load") != "ajax"
	if r.FormValue("duplicate") != "1" {
		s.DuplicateHours = 0
	}
	s.StopWords = strings.Join(stopWordLines(r.FormValue("stop_words")), "\n")
	s.StopWordsAction = entity.StopWordsHold
	if r.FormValue("stop_words_action") == entity.StopWordsReject {
		s.StopWordsAction = entity.StopWordsReject
	}
	return s, ok[0] && ok[1] && ok[2] && ok[3] && ok[4]
}

// stopWordLines - the text's non-blank lines, trimmed.
func stopWordLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if w := strings.TrimSpace(line); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// CommentSettingsPage handles GET/POST /admin/config/comments: what is
// done with suspicious comments ("administer comments").
func (c *Controller) CommentSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if user, _ := c.GetCurrentUser(r); !c.authService.HasPermission(ctx, user, service.PermAdministerComments) {
		c.RenderForbidden(w, r)
		return
	}
	if r.Method == http.MethodPost {
		next, ok := commentSettingsFromForm(r)
		if !ok {
			http.Redirect(w, r, commentSettingsPath+"?error=1", http.StatusSeeOther)
			return
		}
		if err := c.commentSettings.Update(ctx, next); err != nil {
			c.LogFailure(err, "saving the comment settings")
			http.Redirect(w, r, commentSettingsPath+"?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, commentSettingsPath+"?saved=1", http.StatusSeeOther)
		return
	}
	settings, err := c.commentSettings.Get(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "loading the comment settings")
		return
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Comments")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["settings"] = settings
	data["newAuthorOptions"] = settingOptions(newAuthorChoices, settings.NewAuthorChecks, func(v int) string {
		return map[int]string{1: i18n.T(ctx, "the first comment"), 3: i18n.T(ctx, "the first 3 comments"), -1: i18n.T(ctx, "every comment"), 0: i18n.T(ctx, "no check")}[v]
	})
	data["linksOptions"] = settingOptions(linksChoices, settings.LinksLimit, func(v int) string {
		if v < 0 {
			return i18n.T(ctx, "no limit")
		}
		return i18n.Plural(ctx, v, "more than @count link", "more than @count links")
	})
	duplicateHours := settings.DuplicateHours
	if duplicateHours == 0 {
		duplicateHours = entity.DefaultCommentSettings().DuplicateHours
	}
	data["duplicateOptions"] = settingOptions(duplicateChoices, duplicateHours, func(v int) string {
		return map[int]string{1: i18n.T(ctx, "an hour"), 24: i18n.T(ctx, "a day"), 168: i18n.T(ctx, "a week")}[v]
	})
	data["spamKeepOptions"] = settingOptions(spamKeepChoices, settings.SpamKeepDays, func(v int) string {
		if v == 0 {
			return i18n.T(ctx, "never")
		}
		return i18n.Plural(ctx, v, "after @count day", "after @count days")
	})
	data["botSecondsOptions"] = settingOptions(botSecondsChoices, settings.BotMinSeconds, func(v int) string {
		return i18n.Plural(ctx, v, "@count second", "@count seconds")
	})
	data["commentCaptcha"] = c.captchaService.SiteKey(entity.CaptchaFormComment) != ""
	c.RenderOrFail(w, r, "admin/config/comments.html", data)
}
