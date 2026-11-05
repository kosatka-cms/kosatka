package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/interfacepack"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/interfacestring"
)

// interfaceStringPack - the origin of a downloaded translation's strings.
const interfaceStringPack = "pack"

// interfacePackBatch - the strings of a pack written a statement.
const interfacePackBatch = 500

// InterfacePackRepository - entity.InterfacePackRepository on ent: the
// packs in interface_packs, their strings in interface_strings (origin
// "pack").
type InterfacePackRepository struct {
	client *generated.Client
}

var _ entity.InterfacePackRepository = (*InterfacePackRepository)(nil)

// NewInterfacePackRepository - the repository on db.
func NewInterfacePackRepository(db *sql.DB) *InterfacePackRepository {
	return &InterfacePackRepository{client: clientFor(db)}
}

// NewInterfacePackRepositoryOn - the repository on client: for a caller
// with a client of its database (its dialect), not the app's.
func NewInterfacePackRepositoryOn(client *generated.Client) *InterfacePackRepository {
	return &InterfacePackRepository{client: client}
}

// Packs - see entity.InterfacePackRepository.Packs.
func (r *InterfacePackRepository) Packs(ctx context.Context) ([]entity.InterfacePack, error) {
	rows, err := GetClient(ctx, r.client).InterfacePack.Query().
		Order(generated.Asc(interfacepack.FieldLang)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the interface packs: %w", err)
	}
	out := make([]entity.InterfacePack, len(rows))
	for i, row := range rows {
		out[i] = entity.InterfacePack{Lang: row.Lang, Version: row.Version, SHA256: row.Sha256,
			Strings: row.Strings, Draft: row.Draft, InstalledAt: row.InstalledAt}
	}
	return out, nil
}

// Strings - see entity.InterfacePackRepository.Strings.
func (r *InterfacePackRepository) Strings(ctx context.Context) ([]entity.InterfaceString, error) {
	rows, err := GetClient(ctx, r.client).InterfaceString.Query().
		Where(interfacestring.Origin(interfaceStringPack)).
		Order(generated.Asc(interfacestring.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the interface packs' strings: %w", err)
	}
	out := make([]entity.InterfaceString, len(rows))
	for i, row := range rows {
		out[i] = entity.InterfaceString{Lang: row.Lang, Context: row.Context, Source: row.Source,
			Text: row.Text, Forms: row.Forms, UpdatedAt: row.UpdatedAt}
	}
	return out, nil
}

// Install - see entity.InterfacePackRepository.Install.
func (r *InterfacePackRepository) Install(ctx context.Context, p entity.InterfacePack, strings []entity.InterfaceString) error {
	err := inTx(ctx, r.client, func(c *generated.Client) error {
		if err := deletePack(ctx, c, p.Lang); err != nil {
			return err
		}
		rows := make([]*generated.InterfaceStringCreate, len(strings))
		for i, s := range strings {
			rows[i] = c.InterfaceString.Create().
				SetLang(p.Lang).SetOrigin(interfaceStringPack).SetContext(s.Context).SetSource(s.Source).
				SetKeyHash(interfaceStringKey(s.Context, s.Source)).
				SetText(s.Text).SetForms(s.Forms).SetUpdatedAt(p.InstalledAt)
		}
		for len(rows) > 0 {
			n := min(len(rows), interfacePackBatch)
			if err := c.InterfaceString.CreateBulk(rows[:n]...).Exec(ctx); err != nil {
				return err
			}
			rows = rows[n:]
		}
		return c.InterfacePack.Create().
			SetLang(p.Lang).SetVersion(p.Version).SetSha256(p.SHA256).
			SetStrings(p.Strings).SetDraft(p.Draft).SetInstalledAt(p.InstalledAt).
			Exec(ctx)
	})
	if err != nil {
		return fmt.Errorf("installing the %s interface pack: %w", p.Lang, err)
	}
	return nil
}

// Delete - see entity.InterfacePackRepository.Delete.
func (r *InterfacePackRepository) Delete(ctx context.Context, lang string) error {
	if err := inTx(ctx, r.client, func(c *generated.Client) error { return deletePack(ctx, c, lang) }); err != nil {
		return fmt.Errorf("deleting the %s interface pack: %w", lang, err)
	}
	return nil
}

// deletePack removes lang's pack and its strings.
func deletePack(ctx context.Context, c *generated.Client, lang string) error {
	if _, err := c.InterfaceString.Delete().Where(interfacestring.Lang(lang), interfacestring.Origin(interfaceStringPack)).Exec(ctx); err != nil {
		return err
	}
	_, err := c.InterfacePack.Delete().Where(interfacepack.Lang(lang)).Exec(ctx)
	return err
}
