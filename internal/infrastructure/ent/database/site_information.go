package database

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/siteinformation"
)

// SiteInformationRepository - the site information row. Read on every
// page, it is kept after the first read until the repository writes it
// (the site runs one process; app migrate, which writes it too, is
// followed by a restart).
type SiteInformationRepository struct {
	client *generated.Client
	cached atomic.Pointer[entity.SiteInformation]
}

// NewSiteInformationRepository - the site information repository on db.
func NewSiteInformationRepository(db *sql.DB) *SiteInformationRepository {
	return &SiteInformationRepository{client: clientFor(db)}
}

// Get - the site information; outside a transaction, the one kept since
// the last write (a copy).
func (r *SiteInformationRepository) Get(ctx context.Context) (*entity.SiteInformation, error) {
	_, inTx := ctx.Value(txKey{}).(*generated.Client)
	if c := r.cached.Load(); c != nil && !inTx {
		cp := *c
		cp.Languages = slices.Clone(c.Languages)
		cp.SchemaOrg.SameAs = slices.Clone(c.SchemaOrg.SameAs)
		return &cp, nil
	}
	site, err := r.load(ctx)
	if err != nil {
		return nil, err
	}
	if !inTx {
		kept := *site
		kept.Languages = slices.Clone(site.Languages)
		kept.SchemaOrg.SameAs = slices.Clone(site.SchemaOrg.SameAs)
		r.cached.Store(&kept)
	}
	return site, nil
}

// Cached - the site information kept in memory, nil when none is (not
// read since it was written): for a page made without the database (it is
// gone).
func (r *SiteInformationRepository) Cached() *entity.SiteInformation {
	c := r.cached.Load()
	if c == nil {
		return nil
	}
	cp := *c
	cp.Languages = slices.Clone(c.Languages)
	cp.SchemaOrg.SameAs = slices.Clone(c.SchemaOrg.SameAs)
	return &cp
}

// forget drops the kept site information: it was written (afterWrite).
func (r *SiteInformationRepository) forget() { r.cached.Store(nil) }

func (r *SiteInformationRepository) load(ctx context.Context) (*entity.SiteInformation, error) {
	s, err := GetClient(ctx, r.client).SiteInformation.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("SiteInformationRepository.Get: %w", err)
	}
	return &entity.SiteInformation{
		SiteName:        s.SiteName,
		Slogan:          s.Slogan,
		SiteEmail:       s.SiteEmail,
		Logo:            entity.File{URI: deref(s.Logo)},
		Icon:            entity.File{URI: deref(s.Icon)},
		FrontFeedView:   s.FrontFeedView,
		Timezone:        deref(s.Timezone),
		DefaultLanguage: deref(s.DefaultLanguage),
		Page404:         deref(s.Page404),
		Page403:         deref(s.Page403),
		Theme:           s.Theme,
		BaseURL:         deref(s.BaseURL),
		RobotsExtra:     deref(s.RobotsExtra),
		Languages:       s.Languages,
		SchemaOrg:       s.SchemaOrg,
	}, nil
}

// RenameFrontFeedView - the front page feed's View (FrontFeedView) from
// display oldName to newName, if it is oldName: the display was renamed.
func (r *SiteInformationRepository) RenameFrontFeedView(ctx context.Context, oldName, newName string) error {
	defer afterWrite(ctx, r.forget)
	if _, err := GetClient(ctx, r.client).SiteInformation.Update().
		Where(siteinformation.FrontFeedView(oldName)).SetFrontFeedView(newName).Save(ctx); err != nil {
		return fmt.Errorf("renaming the front page feed's View %q: %w", oldName, err)
	}
	return nil
}

// Update saves the site's name, slogan, e-mail, front page feed, timezone,
// default language, error pages, base URL, logo and icon; the theme, the
// robots.txt rules, the languages and the schema.org data have their own
// setters and stay.
func (r *SiteInformationRepository) Update(ctx context.Context, s *entity.SiteInformation) error {
	defer afterWrite(ctx, r.forget)
	u := GetClient(ctx, r.client).SiteInformation.UpdateOneID(settingsRowID).
		SetSiteName(s.SiteName).
		SetSlogan(s.Slogan).
		SetSiteEmail(s.SiteEmail).
		SetFrontFeedView(s.FrontFeedView).
		SetTimezone(s.Timezone).
		SetDefaultLanguage(s.DefaultLanguage).
		SetPage404(s.Page404).
		SetPage403(s.Page403).
		SetBaseURL(s.BaseURL)
	if s.Logo.URI != "" {
		u.SetLogo(s.Logo.URI)
	} else {
		u.ClearLogo()
	}
	if s.Icon.URI != "" {
		u.SetIcon(s.Icon.URI)
	} else {
		u.ClearIcon()
	}
	return u.Exec(ctx)
}

// SetRobotsExtra stores the site's own robots.txt rules (Access blocking
// page); Update leaves them as is.
func (r *SiteInformationRepository) SetRobotsExtra(ctx context.Context, rules string) error {
	defer afterWrite(ctx, r.forget)
	return GetClient(ctx, r.client).SiteInformation.UpdateOneID(settingsRowID).SetRobotsExtra(rules).Exec(ctx)
}

// SetTheme stores the active theme (Appearance page); Update leaves it as is.
func (r *SiteInformationRepository) SetTheme(ctx context.Context, theme string) error {
	defer afterWrite(ctx, r.forget)
	return GetClient(ctx, r.client).SiteInformation.UpdateOneID(settingsRowID).SetTheme(theme).Exec(ctx)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// SetLanguages stores the site's languages besides the default one
// (entity.SiteInformation.Languages).
func (r *SiteInformationRepository) SetLanguages(ctx context.Context, languages []entity.SiteLanguage) error {
	defer afterWrite(ctx, r.forget)
	if err := GetClient(ctx, r.client).SiteInformation.UpdateOneID(settingsRowID).SetLanguages(languages).Exec(ctx); err != nil {
		return fmt.Errorf("storing the site's languages: %w", err)
	}
	return nil
}

// SetSchemaOrg stores who runs the site (entity.SiteInformation.SchemaOrg).
func (r *SiteInformationRepository) SetSchemaOrg(ctx context.Context, o entity.SiteSchemaOrg) error {
	defer afterWrite(ctx, r.forget)
	if err := GetClient(ctx, r.client).SiteInformation.UpdateOneID(settingsRowID).SetSchemaOrg(o).Exec(ctx); err != nil {
		return fmt.Errorf("saving who runs the site: %w", err)
	}
	return nil
}
