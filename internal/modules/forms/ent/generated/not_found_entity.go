package generated

// Hand-written, not generated: ent's codegen leaves files without its
// "Code generated" header alone.

import "github.com/kosatka-cms/kosatka/modulekit/schemakit"

// Is makes errors.Is(err, schemakit.ErrEntityNotFound) true for ent's
// not-found errors, so the engine recognizes the module's missing record
// as it does the core's (its handlers answer 404).
func (e *NotFoundError) Is(target error) bool {
	return target == schemakit.ErrEntityNotFound
}
