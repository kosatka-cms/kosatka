package generated

// Hand-written, not generated: ent's codegen leaves files without its
// "Code generated" header alone.

import "github.com/kosatka-cms/kosatka/internal/domain/entity"

// Is makes errors.Is(err, entity.ErrEntityNotFound) true for ent's
// not-found errors, so the domain recognizes a missing record without
// knowing about ent (service.IsNotFound, handlers' 404s).
func (e *NotFoundError) Is(target error) bool {
	return target == entity.ErrEntityNotFound
}
