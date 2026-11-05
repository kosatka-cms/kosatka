package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// CaptchaSettingsRepository implements entity.CaptchaSettingsRepository.
type CaptchaSettingsRepository struct {
	client *generated.Client
}

var _ entity.CaptchaSettingsRepository = (*CaptchaSettingsRepository)(nil)

// NewCaptchaSettingsRepository - the CAPTCHA settings repository on db.
func NewCaptchaSettingsRepository(db *sql.DB) *CaptchaSettingsRepository {
	return &CaptchaSettingsRepository{client: clientFor(db)}
}

// Get - the CAPTCHA settings from their single row; Forms holds the
// switches of the core's forms and of the modules' ones that were saved.
func (r *CaptchaSettingsRepository) Get(ctx context.Context) (*entity.CaptchaSettings, error) {
	c, err := GetClient(ctx, r.client).CaptchaSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("CaptchaSettingsRepository.Get: %w", err)
	}
	forms := map[string]bool{}
	// The modules' forms (a form missing - its default, see CaptchaService).
	if c.ModuleForms != "" {
		if err := json.Unmarshal([]byte(c.ModuleForms), &forms); err != nil {
			return nil, fmt.Errorf("captcha_settings.module_forms: %w", err)
		}
	}
	forms[entity.CaptchaFormLogin] = c.OnLogin
	forms[entity.CaptchaFormRegister] = c.OnRegister
	forms[entity.CaptchaFormPassword] = c.OnPassword
	return &entity.CaptchaSettings{
		Enabled:   c.Enabled,
		SiteKey:   c.SiteKey,
		SecretKey: openSecret(c.SecretKey),
		Forms:     forms,
	}, nil
}

// moduleForms - the forms of s that aren't the core's, as stored in
// module_forms.
func moduleForms(s *entity.CaptchaSettings) (string, error) {
	out := map[string]bool{}
	for name, on := range s.Forms {
		switch name {
		case entity.CaptchaFormLogin, entity.CaptchaFormRegister, entity.CaptchaFormPassword:
		default:
			out[name] = on
		}
	}
	if len(out) == 0 {
		return "", nil
	}
	b, err := json.Marshal(out)
	return string(b), err
}

// Update saves the settings row (the secret key sealed), creating it if
// it isn't there.
func (r *CaptchaSettingsRepository) Update(ctx context.Context, s *entity.CaptchaSettings) error {
	client := GetClient(ctx, r.client)
	modules, err := moduleForms(s)
	if err != nil {
		return err
	}
	secret, err := sealSecret(s.SecretKey)
	if err != nil {
		return err
	}
	err = client.CaptchaSettings.UpdateOneID(settingsRowID).
		SetEnabled(s.Enabled).SetSiteKey(s.SiteKey).SetSecretKey(secret).
		SetOnLogin(s.Forms[entity.CaptchaFormLogin]).SetOnRegister(s.Forms[entity.CaptchaFormRegister]).
		SetOnPassword(s.Forms[entity.CaptchaFormPassword]).SetModuleForms(modules).
		Exec(ctx)
	if !generated.IsNotFound(err) {
		return err
	}
	return client.CaptchaSettings.Create().SetID(settingsRowID).
		SetEnabled(s.Enabled).SetSiteKey(s.SiteKey).SetSecretKey(secret).
		SetOnLogin(s.Forms[entity.CaptchaFormLogin]).SetOnRegister(s.Forms[entity.CaptchaFormRegister]).
		SetOnPassword(s.Forms[entity.CaptchaFormPassword]).SetModuleForms(modules).
		Exec(ctx)
}
