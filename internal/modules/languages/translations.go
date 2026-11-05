package languages

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Version - a content item of a set of translations as shown: the item
// (its own columns: type, title, address, status, language) and its
// place in the set.
type Version struct {
	Member
	Node modulekit.Node
}

// Public - anyone may see it: published and not on moderation.
func (v Version) Public() bool { return v.Node.Status == 1 && v.Node.OnModeration == 0 }

// nodeRows - the core's nodes the module reads (their own columns).
type nodeRows interface {
	ByIDs(ctx context.Context, ids []int) (map[int]modulekit.Node, error)
}

// Translations - the sets of translations and which content types have
// them.
type Translations struct {
	repo  *TranslationRepo
	nodes nodeRows

	mu       sync.Mutex
	settings Settings
	known    bool
}

// NewTranslations creates Translations.
func NewTranslations(repo *TranslationRepo, nodes nodeRows) *Translations {
	return &Translations{repo: repo, nodes: nodes}
}

// Versions - the items of nid's set of translations ctx's visitor may
// see (the trash's left out), the original first; none - nid has no
// translations.
func (t *Translations) Versions(ctx context.Context, nid int) ([]Version, error) {
	members, err := t.repo.Set(ctx, nid)
	if err != nil || len(members) == 0 {
		return nil, err
	}
	ids := make([]int, len(members))
	for i, m := range members {
		ids[i] = m.Nid
	}
	nodes, err := t.nodes.ByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("loading the translations of node %d: %w", nid, err)
	}
	var out []Version
	for _, m := range members {
		if n, ok := nodes[m.Nid]; ok {
			out = append(out, Version{Member: m, Node: n})
		}
	}
	return out, nil
}

// ByLanguage - versions by language: the first of each.
func ByLanguage(versions []Version) map[string]Version {
	out := make(map[string]Version, len(versions))
	for _, v := range versions {
		if _, ok := out[v.Node.Langcode]; !ok {
			out[v.Node.Langcode] = v
		}
	}
	return out
}

// Settings - the module's settings (read once, kept until saved).
func (t *Translations) Settings(ctx context.Context) (Settings, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.known {
		st, err := t.repo.Settings(ctx)
		if err != nil {
			return Settings{}, err
		}
		t.settings, t.known = st, true
	}
	st := t.settings
	st.TranslatableTypes = slices.Clone(st.TranslatableTypes)
	return st, nil
}

// Translatable - the items of nodeType are translated.
func (t *Translations) Translatable(ctx context.Context, nodeType string) (bool, error) {
	st, err := t.Settings(ctx)
	return slices.Contains(st.TranslatableTypes, nodeType), err
}

// SaveSettings saves the module's settings.
func (t *Translations) SaveSettings(ctx context.Context, st Settings) error {
	if err := t.repo.SaveSettings(ctx, st); err != nil {
		return err
	}
	t.mu.Lock()
	t.settings, t.known = st, true
	t.mu.Unlock()
	return nil
}

// Translate creates node's translation into lang by user - a copy of it
// in lang, its title kept, unpublished - in node's set: its id.
func (t *Translations) Translate(ctx context.Context, db *sql.DB, node modulekit.Node, lang string, user *modulekit.User) (int, error) {
	var id int
	err := modulekit.InTx(ctx, db, func(ctx context.Context) error {
		var err error
		if id, err = modulekit.CopyNode(i18n.WithLang(ctx, lang), node.ID, user, node.Title); err != nil {
			return fmt.Errorf("copying node %d into %s: %w", node.ID, lang, err)
		}
		return t.repo.Add(ctx, node.ID, id)
	})
	return id, err
}
