package generated

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
)

// TestNotFoundErrorIsEntityNotFound - ent's not-found error, wrapped or
// not, is entity.ErrEntityNotFound for errors.Is; other errors aren't.
func TestNotFoundErrorIsEntityNotFound(t *testing.T) {
	err := error(&NotFoundError{label: "node"})
	if !errors.Is(err, entity.ErrEntityNotFound) || !errors.Is(fmt.Errorf("loading: %w", err), entity.ErrEntityNotFound) {
		t.Error("ent's not-found isn't entity.ErrEntityNotFound")
	}
	// The one sentinel: a handler checking apperrors answers 404 too.
	if !apperrors.IsNotFound(fmt.Errorf("voting: %w", err)) {
		t.Error("ent's not-found isn't apperrors.ErrNotFound")
	}
	if errors.Is(&NotSingularError{label: "node"}, entity.ErrEntityNotFound) || errors.Is(errors.New("x"), entity.ErrEntityNotFound) {
		t.Error("another error is entity.ErrEntityNotFound")
	}
}
