package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/interfacestring"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
)

// interfaceStringSite - the origin of the translations changed in the
// admin.
const interfaceStringSite = "site"

// InterfaceStringRepository - entity.InterfaceStringRepository on ent.
type InterfaceStringRepository struct {
	client *generated.Client
}

var _ entity.InterfaceStringRepository = (*InterfaceStringRepository)(nil)

// NewInterfaceStringRepository - the repository on db.
func NewInterfaceStringRepository(db *sql.DB) *InterfaceStringRepository {
	return &InterfaceStringRepository{client: clientFor(db)}
}

// interfaceStringKey - the key_hash of a string: SHA-256 of its context
// and source, hex.
func interfaceStringKey(msgctx, source string) string {
	sum := sha256.Sum256([]byte(msgctx + "\x04" + source))
	return hex.EncodeToString(sum[:])
}

// All - see entity.InterfaceStringRepository.All.
func (r *InterfaceStringRepository) All(ctx context.Context) ([]entity.InterfaceString, error) {
	rows, err := GetClient(ctx, r.client).InterfaceString.Query().
		Where(interfacestring.Origin(interfaceStringSite)).
		Order(generated.Asc(interfacestring.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the interface strings: %w", err)
	}
	out := make([]entity.InterfaceString, len(rows))
	for i, row := range rows {
		out[i] = entity.InterfaceString{Lang: row.Lang, Context: row.Context, Source: row.Source,
			Text: row.Text, Forms: row.Forms, UpdatedAt: row.UpdatedAt}
	}
	return out, nil
}

// Save - see entity.InterfaceStringRepository.Save.
func (r *InterfaceStringRepository) Save(ctx context.Context, s entity.InterfaceString) error {
	client := GetClient(ctx, r.client)
	hash := interfaceStringKey(s.Context, s.Source)
	where := []predicate.InterfaceString{interfacestring.Lang(s.Lang), interfacestring.Origin(interfaceStringSite), interfacestring.KeyHash(hash)}
	exists, err := client.InterfaceString.Query().Where(where...).Exist(ctx)
	switch {
	case err != nil:
	case exists:
		err = client.InterfaceString.Update().Where(where...).
			SetText(s.Text).SetForms(s.Forms).SetUpdatedAt(s.UpdatedAt).
			Exec(ctx)
	default:
		err = client.InterfaceString.Create().
			SetLang(s.Lang).SetOrigin(interfaceStringSite).SetContext(s.Context).SetSource(s.Source).SetKeyHash(hash).
			SetText(s.Text).SetForms(s.Forms).SetUpdatedAt(s.UpdatedAt).
			Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the interface string %q (%s): %w", s.Source, s.Lang, err)
	}
	return nil
}

// Delete - see entity.InterfaceStringRepository.Delete.
func (r *InterfaceStringRepository) Delete(ctx context.Context, lang, msgctx, source string) error {
	_, err := GetClient(ctx, r.client).InterfaceString.Delete().
		Where(interfacestring.Lang(lang), interfacestring.Origin(interfaceStringSite), interfacestring.KeyHash(interfaceStringKey(msgctx, source))).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting the interface string %q (%s): %w", source, lang, err)
	}
	return nil
}
