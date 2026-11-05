package schema

import (
	"context"

	"entgo.io/ent"

	gen "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/hook"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Hooks of the Node: a new node without a language gets ctx's - the
// request's (its address's prefix), else the site's (i18n.FromContext) -
// whoever creates it: a form, a module, the demo.
func (Node) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(func(next ent.Mutator) ent.Mutator {
			return hook.NodeFunc(func(ctx context.Context, m *gen.NodeMutation) (ent.Value, error) {
				if code, ok := m.Langcode(); !ok || code == "" {
					m.SetLangcode(i18n.FromContext(ctx))
				}
				return next.Mutate(ctx, m)
			})
		}, ent.OpCreate),
	}
}
