package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

var (
	mailDefsMu sync.RWMutex
	mailDefs   []entity.MailDef
)

// RegisterMail adds one of the site's emails: the core's at start, a
// module's from its Wire (modulekit.AddMail). A key added again replaces
// the earlier.
func RegisterMail(d entity.MailDef) {
	mailDefsMu.Lock()
	defer mailDefsMu.Unlock()
	if i := slices.IndexFunc(mailDefs, func(m entity.MailDef) bool { return m.Key == d.Key }); i >= 0 {
		mailDefs[i] = d
		return
	}
	mailDefs = append(mailDefs, d)
}

// MailDefs - the site's emails: the core's first, then the modules' by
// module; each group lightest first, then as added.
func MailDefs() []entity.MailDef {
	mailDefsMu.RLock()
	defer mailDefsMu.RUnlock()
	out := slices.Clone(mailDefs)
	slices.SortStableFunc(out, func(a, b entity.MailDef) int {
		if c := strings.Compare(a.Module, b.Module); c != 0 {
			return c
		}
		return a.Weight - b.Weight
	})
	return out
}

// MailDefByKey - the email key.
func MailDefByKey(key string) (entity.MailDef, bool) {
	mailDefsMu.RLock()
	defer mailDefsMu.RUnlock()
	i := slices.IndexFunc(mailDefs, func(m entity.MailDef) bool { return m.Key == key })
	if i < 0 {
		return entity.MailDef{}, false
	}
	return mailDefs[i], true
}

// ResetMailDefs forgets the emails (tests; the modules wire again).
func ResetMailDefs() {
	mailDefsMu.Lock()
	defer mailDefsMu.Unlock()
	mailDefs = nil
}

// SetTemplates gives the service what the Mail page changed of the
// emails (without it every email is on, with its default text).
func (s *MailService) SetTemplates(repo entity.MailTemplateRepository) { s.templates = repo }

// Stored - what the Mail page changed of the emails, by key.
func (s *MailService) Stored(ctx context.Context) (map[string]entity.StoredMail, error) {
	if s.templates == nil {
		return map[string]entity.StoredMail{}, nil
	}
	return s.templates.All(ctx)
}

// Template - def's template in effect: its own text (def.Load, or the
// mail_templates row), a blank subject or body the default's in the
// site's language.
func (s *MailService) Template(ctx context.Context, def entity.MailDef) (entity.MailTemplate, error) {
	if def.Load != nil {
		t, err := def.Load(ctx)
		return entity.Wrapped(t, err, "loading the email %s", def.Key)
	}
	stored, err := s.Stored(ctx)
	if err != nil {
		return entity.MailTemplate{}, err
	}
	t := stored[def.Key].MailTemplate
	lang := i18n.Default()
	if strings.TrimSpace(t.Subject) == "" {
		t.Subject = i18n.Current().Text(lang, "", def.Default.Subject)
	}
	if strings.TrimSpace(t.Body) == "" {
		t.Body = i18n.Current().Text(lang, "", def.Default.Body)
	}
	return t, nil
}

// SaveTemplate stores def's text (blank - back to the default).
func (s *MailService) SaveTemplate(ctx context.Context, def entity.MailDef, t entity.MailTemplate) error {
	if def.Save != nil {
		if err := def.Save(ctx, t); err != nil {
			return fmt.Errorf("saving the email %s: %w", def.Key, err)
		}
		return nil
	}
	if s.templates == nil {
		return fmt.Errorf("saving the email %s: no storage", def.Key)
	}
	return s.templates.SaveText(ctx, def.Key, t)
}

// SetMailEnabled switches the email key on or off (an optional one: the
// others are always sent).
func (s *MailService) SetMailEnabled(ctx context.Context, key string, on bool) error {
	if s.templates == nil {
		return fmt.Errorf("switching the email %s: no storage", key)
	}
	return s.templates.SetEnabled(ctx, key, on)
}

// Send sends the site's email key to `to`: its template with data's
// placeholders filled in, recorded in the mail log. An optional email
// switched off on the Mail page isn't sent; with email off nothing is.
func (s *MailService) Send(ctx context.Context, key, to string, data mailtemplate.TokenData) error {
	def, ok := MailDefByKey(key)
	if !ok {
		return fmt.Errorf("sending the email %s: no such email", key)
	}
	if def.Optional {
		stored, err := s.Stored(ctx)
		if err != nil {
			return fmt.Errorf("sending the email %s: %w", key, err)
		}
		if st, ok := stored[key]; ok && !st.Enabled {
			return nil
		}
	}
	t, err := s.Template(ctx, def)
	if err != nil {
		return fmt.Errorf("sending the email %s: %w", key, err)
	}
	return s.deliver(key, to, mailtemplate.ReplacePlaceholders(t.Subject, data), mailtemplate.ReplacePlaceholders(t.Body, data), false)
}

// SendSample sends def's template with data to `to` whether it is
// switched on or not: the Mail page's test. ErrMailOff - nothing would be
// sent.
func (s *MailService) SendSample(ctx context.Context, def entity.MailDef, to string, data mailtemplate.TokenData) error {
	t, err := s.Template(ctx, def)
	if err != nil {
		return err
	}
	return s.deliver(def.Key, to, mailtemplate.ReplacePlaceholders(t.Subject, data), mailtemplate.ReplacePlaceholders(t.Body, data), true)
}
