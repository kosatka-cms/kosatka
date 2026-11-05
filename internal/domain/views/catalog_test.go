package views

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestValidOperatorsFor(t *testing.T) {
	presence := []entity.ViewFilterOperator{entity.ViewFilterOperatorEmpty, entity.ViewFilterOperatorNotEmpty}

	for _, fieldType := range entity.FieldTypes {
		if !FilterableFieldType(fieldType) {
			continue
		}
		// Reference is Contextual-only (its own operators below), no
		// presence check on a "currently viewed node" relation.
		if fieldType == entity.FieldTypeReference {
			continue
		}
		t.Run(string(fieldType), func(t *testing.T) {
			got := validOperatorsFor(fieldType)
			for _, op := range presence {
				if !slices.Contains(got, op) {
					t.Fatalf("expected %q to allow %q, got %v", fieldType, op, got)
				}
			}
		})
	}

	t.Run("unknown field type has no operators", func(t *testing.T) {
		if got := validOperatorsFor("nonsense"); got != nil {
			t.Fatalf("expected nil for an unknown field type, got %v", got)
		}
	})

	t.Run("image only gets presence operators", func(t *testing.T) {
		got := validOperatorsFor(entity.FieldTypeImage)
		for _, op := range []entity.ViewFilterOperator{entity.ViewFilterOperatorEq, entity.ViewFilterOperatorIn, entity.ViewFilterOperatorNotIn, entity.ViewFilterOperatorContains, entity.ViewFilterOperatorGte, entity.ViewFilterOperatorLte} {
			if slices.Contains(got, op) {
				t.Fatalf("a raw image reference has no meaningful %q comparison, got %v", op, got)
			}
		}
	})

	t.Run("reference gets only the contextual relation operators", func(t *testing.T) {
		got := validOperatorsFor(entity.FieldTypeReference)
		want := []entity.ViewFilterOperator{entity.ViewFilterOperatorReferencedByCurrent, entity.ViewFilterOperatorRelatedToCurrent}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("taxonomy gets not_in alongside in", func(t *testing.T) {
		got := validOperatorsFor(entity.FieldTypeTaxonomy)
		if !slices.Contains(got, entity.ViewFilterOperatorIn) || !slices.Contains(got, entity.ViewFilterOperatorNotIn) {
			t.Fatalf("expected taxonomy to allow both in and not_in, got %v", got)
		}
	})
}

func TestValidOperatorsForBuiltinExcludesPresence(t *testing.T) {
	builtins := []entity.ViewFilterBuiltinField{
		entity.ViewFilterBuiltinStatus,
		entity.ViewFilterBuiltinOnModeration,
		entity.ViewFilterBuiltinCreatedBy,
		entity.ViewFilterBuiltinTitle,
		entity.ViewFilterBuiltinContentType,
	}
	for _, field := range builtins {
		t.Run(string(field), func(t *testing.T) {
			got := validOperatorsForBuiltin(field)
			if slices.Contains(got, entity.ViewFilterOperatorEmpty) || slices.Contains(got, entity.ViewFilterOperatorNotEmpty) {
				t.Fatalf("built-in Node column always has a value, empty/not_empty must not apply to %q, got %v", field, got)
			}
		})
	}
}

// TestValidOperatorsForCommentBuiltin - base_entity=="comment" (see
// entity.ViewBaseEntity's doc comment) builtin fields get the same
// operator treatment as their Content analogs, and (like every other
// builtin) never allow empty/not_empty.
func TestValidOperatorsForCommentBuiltin(t *testing.T) {
	builtins := []entity.ViewFilterBuiltinField{
		entity.ViewFilterBuiltinCommentStatus,
		entity.ViewFilterBuiltinCommentCreatedBy,
		entity.ViewFilterBuiltinCommentSubject,
		entity.ViewFilterBuiltinCommentContent,
		entity.ViewFilterBuiltinCommentNodeType,
		entity.ViewFilterBuiltinCommentIsReply,
	}
	for _, field := range builtins {
		t.Run(string(field), func(t *testing.T) {
			got := validOperatorsForBuiltin(field)
			if len(got) == 0 {
				t.Fatalf("expected %q to be a recognized comment builtin with allowed operators, got none", field)
			}
			if slices.Contains(got, entity.ViewFilterOperatorEmpty) || slices.Contains(got, entity.ViewFilterOperatorNotEmpty) {
				t.Fatalf("built-in Comment column always has a value, empty/not_empty must not apply to %q, got %v", field, got)
			}
		})
	}
	t.Run("comment_created_by allows in", func(t *testing.T) {
		got := validOperatorsForBuiltin(entity.ViewFilterBuiltinCommentCreatedBy)
		if !slices.Contains(got, entity.ViewFilterOperatorIn) {
			t.Fatalf("expected comment_created_by to allow \"in\" (same as created_by), got %v", got)
		}
	})
	t.Run("comment_subject allows contains", func(t *testing.T) {
		got := validOperatorsForBuiltin(entity.ViewFilterBuiltinCommentSubject)
		if !slices.Contains(got, entity.ViewFilterOperatorContains) {
			t.Fatalf("expected comment_subject to allow \"contains\" (same as title), got %v", got)
		}
	})
}

// TestValidOperatorsForBuiltinExcludesStartsWith - starts_with reaches a
// query only through the synthetic glossaryLetterFilter, never through the
// admin "add filter" form; guards against exposing it there by accident.
func TestValidOperatorsForBuiltinExcludesStartsWith(t *testing.T) {
	got := validOperatorsForBuiltin(entity.ViewFilterBuiltinTitle)
	if slices.Contains(got, entity.ViewFilterOperatorStartsWith) {
		t.Fatalf("starts_with must not be admin-selectable in v1, got %v", got)
	}
}

// TestFieldTypesDecidedInViews - every field type is decided for Views:
// it has its entry in viewFieldTypes (one Views can't use - with its
// reason). A new type fails here until it does.
func TestFieldTypesDecidedInViews(t *testing.T) {
	for _, ft := range entity.FieldTypes {
		if _, ok := viewFieldTypes[ft]; !ok {
			t.Errorf("%s: not in viewFieldTypes - decide what Views do with it", ft)
		}
		if SortableFieldType(ft) && !FilterableFieldType(ft) && !DisplayableFieldType(ft) {
			t.Errorf("%s: sortable, but neither filtered nor shown", ft)
		}
	}
}

// TestBuiltinsAccepted - every builtin field, sort and filter a View of
// either base entity offers is one the service accepts (offering one it
// refuses fails the save), and every builtin field has a name.
func TestBuiltinsAccepted(t *testing.T) {
	for _, view := range []entity.ViewDisplayDef{{}, {BaseEntity: entity.ViewBaseEntityComment}} {
		for _, b := range FieldBuiltins(view) {
			if !FieldBuiltinAllowed(b) {
				t.Errorf("field %q offered, refused", b)
			}
			if FieldBuiltinLabel(context.Background(), b) == string(b) {
				t.Errorf("field %q has no name", b)
			}
		}
		for _, b := range SortBuiltins(view) {
			if !SortBuiltinAllowed(b) {
				t.Errorf("sort %q offered, refused", b)
			}
		}
		for _, b := range FilterBuiltins(view) {
			if !FilterBuiltinAllowed(b) {
				t.Errorf("filter %q offered, refused", b)
			}
		}
	}
	if len(FieldBuiltins(entity.ViewDisplayDef{BaseEntity: entity.ViewBaseEntityComment})) == len(FieldBuiltins(entity.ViewDisplayDef{})) {
		t.Error("a comment View offers the content View's fields")
	}
}
