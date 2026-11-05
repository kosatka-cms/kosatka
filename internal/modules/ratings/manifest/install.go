package manifest

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/demokit"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// SettingsRowID - the id of rating_settings' single row.
const SettingsRowID = 1

// RegisterSteps registers the module's own ent schema (its tables and
// migration files), its install step and its demo step
// (manifests.Register).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	// Installed: its settings row (5 stars, a vote can be changed and
	// taken back), and signed-in users may rate.
	manifestkit.RegisterInstall(Name, func(ctx context.Context, env manifestkit.InstallEnv) error {
		c := generated.NewClient(generated.Driver(manifestkit.Driver(env)))
		if err := c.RatingSettings.Create().SetID(SettingsRowID).Exec(ctx); err != nil {
			return fmt.Errorf("the ratings' settings row: %w", err)
		}
		return manifestkit.GrantPermission(ctx, env, manifestkit.RoleUser, PermRate)
	})
	demokit.Register(Name, createDemo)
}
