package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// IntegrationSettingsRepository - counters and codes (one row).
type IntegrationSettingsRepository struct {
	client *generated.Client
}

// NewIntegrationSettingsRepository - the integration settings repository
// on db.
func NewIntegrationSettingsRepository(db *sql.DB) *IntegrationSettingsRepository {
	return &IntegrationSettingsRepository{client: clientFor(db)}
}

// Get - the counters, codes and consent settings from their single row.
func (r *IntegrationSettingsRepository) Get(ctx context.Context) (*entity.IntegrationSettings, error) {
	s, err := GetClient(ctx, r.client).IntegrationSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("IntegrationSettingsRepository.Get: %w", err)
	}
	return &entity.IntegrationSettings{
		VerificationTags: s.VerificationTags, MetrikaID: s.MetrikaID, MetrikaClickmap: s.MetrikaClickmap,
		MetrikaTrackLinks: s.MetrikaTrackLinks, MetrikaAccurateBounce: s.MetrikaAccurateBounce, MetrikaWebvisor: s.MetrikaWebvisor,
		GAID: s.GaID, HeadCode: s.HeadCode, BodyEndCode: s.BodyEndCode, BodyEndSkipServicePages: s.BodyEndSkipServicePages,
		Consent: entity.Consent{
			Enabled: s.ConsentEnabled, Mode: entity.ConsentMode(s.ConsentMode), Text: s.ConsentText, PolicyURL: s.ConsentPolicyURL,
			Version: s.ConsentVersion, MetrikaGroup: entity.ParseConsentGroup(s.MetrikaGroup), GAGroup: entity.ParseConsentGroup(s.GaGroup),
			HeadGroup: entity.ParseConsentGroup(s.HeadCodeGroup), BodyEndGroup: entity.ParseConsentGroup(s.BodyEndGroup),
		},
	}, nil
}

// Update saves s into the integration settings row.
func (r *IntegrationSettingsRepository) Update(ctx context.Context, s entity.IntegrationSettings) error {
	return GetClient(ctx, r.client).IntegrationSettings.UpdateOneID(settingsRowID).
		SetVerificationTags(s.VerificationTags).SetMetrikaID(s.MetrikaID).SetMetrikaClickmap(s.MetrikaClickmap).
		SetMetrikaTrackLinks(s.MetrikaTrackLinks).SetMetrikaAccurateBounce(s.MetrikaAccurateBounce).SetMetrikaWebvisor(s.MetrikaWebvisor).
		SetGaID(s.GAID).SetHeadCode(s.HeadCode).SetBodyEndCode(s.BodyEndCode).SetBodyEndSkipServicePages(s.BodyEndSkipServicePages).
		SetConsentEnabled(s.Consent.Enabled).SetConsentMode(string(s.Consent.Mode)).SetConsentText(s.Consent.Text).
		SetConsentPolicyURL(s.Consent.PolicyURL).SetConsentVersion(s.Consent.Version).
		SetMetrikaGroup(string(s.Consent.MetrikaGroup)).SetGaGroup(string(s.Consent.GAGroup)).
		SetHeadCodeGroup(string(s.Consent.HeadGroup)).SetBodyEndGroup(string(s.Consent.BodyEndGroup)).
		Exec(ctx)
}
