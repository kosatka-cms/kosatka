package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// The bot trap's fields: the one hidden from people (bots fill every
// field; its name looks worth filling) and the time the form was shown,
// signed.
const (
	botTrapField     = "address_line_2"
	botTrapTimeField = "form_shown"
)

// ErrBotTrap - a form sent by a bot: the hidden field filled, sent sooner
// than a person could, or without the signed time it was shown.
var ErrBotTrap = errors.New("the form looks sent by a bot")

// BotTrap - the forms' guard against bots (entity.CommentSettings.BotTrap):
// Fields go into a form, Check its submission.
type BotTrap struct {
	secret   []byte
	settings entity.CommentSettingsRepository
	now      func() time.Time
}

// NewBotTrap creates a BotTrap signing with secret (the session secret).
func NewBotTrap(secret string, settings entity.CommentSettingsRepository) *BotTrap {
	return &BotTrap{secret: []byte("bot-trap:" + secret), settings: settings, now: time.Now}
}

// sign - the signature of the time shown.
func (b *BotTrap) sign(shown string) string {
	m := hmac.New(sha256.New, b.secret)
	m.Write([]byte(shown))
	return hex.EncodeToString(m.Sum(nil))[:32]
}

// on - the trap's settings when it is on (false - off, or they can't be
// read: forms go unguarded rather than refused).
func (b *BotTrap) on(ctx context.Context) (entity.CommentSettings, bool) {
	if b == nil {
		return entity.CommentSettings{}, false
	}
	cfg, err := b.settings.Get(ctx)
	if err != nil {
		LogFallback(slog.Default(), err, "loading the bot trap's settings (forms unguarded)")
		return cfg, false
	}
	return cfg, cfg.BotTrap
}

// Fields - the trap's fields for a form ("" - the trap is off).
func (b *BotTrap) Fields(ctx context.Context) template.HTML {
	if _, on := b.on(ctx); !on {
		return ""
	}
	shown := strconv.FormatInt(b.now().Unix(), 10)
	return template.HTML(`<div aria-hidden="true" style="position:absolute;left:-10000px;top:auto;width:1px;height:1px;overflow:hidden">` +
		`<input type="text" name="` + botTrapField + `" tabindex="-1" autocomplete="off" value=""></div>` +
		`<input type="hidden" name="` + botTrapTimeField + `" value="` + shown + `.` + b.sign(shown) + `">`)
}

// Check - ErrBotTrap when r's form is a bot's: the hidden field filled,
// the time it was shown missing or not signed by the site, or sent
// sooner than BotMinSeconds after; nil when the trap is off.
func (b *BotTrap) Check(ctx context.Context, r *http.Request) error {
	cfg, on := b.on(ctx)
	if !on {
		return nil
	}
	if strings.TrimSpace(r.FormValue(botTrapField)) != "" {
		return ErrBotTrap
	}
	shown, sig, ok := strings.Cut(r.FormValue(botTrapTimeField), ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(b.sign(shown))) {
		return ErrBotTrap
	}
	at, err := strconv.ParseInt(shown, 10, 64)
	if err != nil || b.now().Sub(time.Unix(at, 0)) < time.Duration(cfg.BotMinSeconds)*time.Second {
		return ErrBotTrap
	}
	return nil
}

// botTrap - the site's bot trap (SetBotTrap; nil - none).
var botTrap *BotTrap

// SetBotTrap sets the site's bot trap, at startup.
func SetBotTrap(b *BotTrap) { botTrap = b }

// CheckBotTrap - the site's bot trap's Check of r (nil - none set).
func CheckBotTrap(r *http.Request) error { return botTrap.Check(r.Context(), r) }

// botTrapFields - the template func "botTrap": the site's bot trap's
// fields for a form ("" - none, or it is off).
func botTrapFields() template.HTML { return botTrap.Fields(context.Background()) }

// commentCaptcha - the CAPTCHA site key of the comment form for a user
// (SetCommentCaptcha; "" - none for them).
var commentCaptcha func(ctx context.Context, userID int) string

// SetCommentCaptcha sets what gives the comment form's CAPTCHA site key
// for a user, at startup.
func SetCommentCaptcha(fn func(ctx context.Context, userID int) string) { commentCaptcha = fn }

// CommentCaptchaKey - the comment form's CAPTCHA site key for user ("" -
// none: the CAPTCHA is off for comments, or the user has a published
// comment).
func CommentCaptchaKey(ctx context.Context, user *entity.User) string {
	if commentCaptcha == nil || user == nil {
		return ""
	}
	return commentCaptcha(ctx, user.ID)
}
