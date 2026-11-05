package database

import (
	"context"

	"entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueblock"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluebool"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluedate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefile"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefilemulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelink"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelist"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelistmulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluenumber"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluereference"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluereferencemulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomy"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomymulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueuser"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
)

// valueTable - what is done the same way to every field value table
// (field_value_<type>, the *_multi ones): rows are picked by their node_id
// and field_id columns (valueOfField, valueOfNodes), whatever the value
// column. Reading and writing values stays per type: their shapes differ.
type valueTable struct {
	// name - the table's.
	name    string
	delete  func(ctx context.Context, c *generated.Client, where ...func(*sql.Selector)) (int, error)
	count   func(ctx context.Context, c *generated.Client, where ...func(*sql.Selector)) (int, error)
	nodeIDs func(ctx context.Context, c *generated.Client, where ...func(*sql.Selector)) ([]int, error)
}

// valueOfField - the rows of one field (FieldStorageDef.ID).
func valueOfField(storageID int) func(*sql.Selector) { return sql.FieldEQ("field_id", storageID) }

// valueOfNodes - the rows of these nodes.
func valueOfNodes(nodeIDs ...int) func(*sql.Selector) { return sql.FieldIn("node_id", nodeIDs...) }

// The ent builders of a value table, as newValueTable uses them: P - its
// predicate type, D - its delete builder, Q - its query, S - its select.
type (
	valueDelete[P ~func(*sql.Selector), D any] interface {
		Where(...P) D
		Exec(context.Context) (int, error)
	}
	valueQuery[P ~func(*sql.Selector), Q, S any] interface {
		Where(...P) Q
		Count(context.Context) (int, error)
		Select(...string) S
	}
	valueSelect interface {
		Ints(context.Context) ([]int, error)
	}
)

// newValueTable - a valueTable over one ent entity (its table name) and
// its delete and query builders.
func newValueTable[P ~func(*sql.Selector), D valueDelete[P, D], Q valueQuery[P, Q, S], S valueSelect](
	name string, deleteOf func(*generated.Client) D, queryOf func(*generated.Client) Q,
) valueTable {
	return valueTable{
		name: name,
		delete: func(ctx context.Context, c *generated.Client, w ...func(*sql.Selector)) (int, error) {
			return deleteOf(c).Where(P(sql.AndPredicates(w...))).Exec(ctx)
		},
		count: func(ctx context.Context, c *generated.Client, w ...func(*sql.Selector)) (int, error) {
			return queryOf(c).Where(P(sql.AndPredicates(w...))).Count(ctx)
		},
		nodeIDs: func(ctx context.Context, c *generated.Client, w ...func(*sql.Selector)) ([]int, error) {
			return queryOf(c).Where(P(sql.AndPredicates(w...))).Select("node_id").Ints(ctx)
		},
	}
}

var (
	textValues = newValueTable[predicate.FieldValueText, *generated.FieldValueTextDelete, *generated.FieldValueTextQuery, *generated.FieldValueTextSelect](
		fieldvaluetext.Table,
		func(c *generated.Client) *generated.FieldValueTextDelete { return c.FieldValueText.Delete() },
		func(c *generated.Client) *generated.FieldValueTextQuery { return c.FieldValueText.Query() })
	numberValues = newValueTable[predicate.FieldValueNumber, *generated.FieldValueNumberDelete, *generated.FieldValueNumberQuery, *generated.FieldValueNumberSelect](
		fieldvaluenumber.Table,
		func(c *generated.Client) *generated.FieldValueNumberDelete { return c.FieldValueNumber.Delete() },
		func(c *generated.Client) *generated.FieldValueNumberQuery { return c.FieldValueNumber.Query() })
	dateValues = newValueTable[predicate.FieldValueDate, *generated.FieldValueDateDelete, *generated.FieldValueDateQuery, *generated.FieldValueDateSelect](
		fieldvaluedate.Table,
		func(c *generated.Client) *generated.FieldValueDateDelete { return c.FieldValueDate.Delete() },
		func(c *generated.Client) *generated.FieldValueDateQuery { return c.FieldValueDate.Query() })
	boolValues = newValueTable[predicate.FieldValueBool, *generated.FieldValueBoolDelete, *generated.FieldValueBoolQuery, *generated.FieldValueBoolSelect](
		fieldvaluebool.Table,
		func(c *generated.Client) *generated.FieldValueBoolDelete { return c.FieldValueBool.Delete() },
		func(c *generated.Client) *generated.FieldValueBoolQuery { return c.FieldValueBool.Query() })
	fileValues = newValueTable[predicate.FieldValueFile, *generated.FieldValueFileDelete, *generated.FieldValueFileQuery, *generated.FieldValueFileSelect](
		fieldvaluefile.Table,
		func(c *generated.Client) *generated.FieldValueFileDelete { return c.FieldValueFile.Delete() },
		func(c *generated.Client) *generated.FieldValueFileQuery { return c.FieldValueFile.Query() })
	fileMultiValues = newValueTable[predicate.FieldValueFileMulti, *generated.FieldValueFileMultiDelete, *generated.FieldValueFileMultiQuery, *generated.FieldValueFileMultiSelect](
		fieldvaluefilemulti.Table,
		func(c *generated.Client) *generated.FieldValueFileMultiDelete { return c.FieldValueFileMulti.Delete() },
		func(c *generated.Client) *generated.FieldValueFileMultiQuery { return c.FieldValueFileMulti.Query() })
	taxonomyValues = newValueTable[predicate.FieldValueTaxonomy, *generated.FieldValueTaxonomyDelete, *generated.FieldValueTaxonomyQuery, *generated.FieldValueTaxonomySelect](
		fieldvaluetaxonomy.Table,
		func(c *generated.Client) *generated.FieldValueTaxonomyDelete { return c.FieldValueTaxonomy.Delete() },
		func(c *generated.Client) *generated.FieldValueTaxonomyQuery { return c.FieldValueTaxonomy.Query() })
	taxonomyMultiValues = newValueTable[predicate.FieldValueTaxonomyMulti, *generated.FieldValueTaxonomyMultiDelete, *generated.FieldValueTaxonomyMultiQuery, *generated.FieldValueTaxonomyMultiSelect](
		fieldvaluetaxonomymulti.Table,
		func(c *generated.Client) *generated.FieldValueTaxonomyMultiDelete {
			return c.FieldValueTaxonomyMulti.Delete()
		},
		func(c *generated.Client) *generated.FieldValueTaxonomyMultiQuery {
			return c.FieldValueTaxonomyMulti.Query()
		})
	listValues = newValueTable[predicate.FieldValueList, *generated.FieldValueListDelete, *generated.FieldValueListQuery, *generated.FieldValueListSelect](
		fieldvaluelist.Table,
		func(c *generated.Client) *generated.FieldValueListDelete { return c.FieldValueList.Delete() },
		func(c *generated.Client) *generated.FieldValueListQuery { return c.FieldValueList.Query() })
	listMultiValues = newValueTable[predicate.FieldValueListMulti, *generated.FieldValueListMultiDelete, *generated.FieldValueListMultiQuery, *generated.FieldValueListMultiSelect](
		fieldvaluelistmulti.Table,
		func(c *generated.Client) *generated.FieldValueListMultiDelete { return c.FieldValueListMulti.Delete() },
		func(c *generated.Client) *generated.FieldValueListMultiQuery { return c.FieldValueListMulti.Query() })
	referenceValues = newValueTable[predicate.FieldValueReference, *generated.FieldValueReferenceDelete, *generated.FieldValueReferenceQuery, *generated.FieldValueReferenceSelect](
		fieldvaluereference.Table,
		func(c *generated.Client) *generated.FieldValueReferenceDelete { return c.FieldValueReference.Delete() },
		func(c *generated.Client) *generated.FieldValueReferenceQuery { return c.FieldValueReference.Query() })
	linkValues = newValueTable[predicate.FieldValueLink, *generated.FieldValueLinkDelete, *generated.FieldValueLinkQuery, *generated.FieldValueLinkSelect](
		fieldvaluelink.Table,
		func(c *generated.Client) *generated.FieldValueLinkDelete { return c.FieldValueLink.Delete() },
		func(c *generated.Client) *generated.FieldValueLinkQuery { return c.FieldValueLink.Query() })
	blockValues = newValueTable[predicate.FieldValueBlock, *generated.FieldValueBlockDelete, *generated.FieldValueBlockQuery, *generated.FieldValueBlockSelect](
		fieldvalueblock.Table,
		func(c *generated.Client) *generated.FieldValueBlockDelete { return c.FieldValueBlock.Delete() },
		func(c *generated.Client) *generated.FieldValueBlockQuery { return c.FieldValueBlock.Query() })
	userValues = newValueTable[predicate.FieldValueUser, *generated.FieldValueUserDelete, *generated.FieldValueUserQuery, *generated.FieldValueUserSelect](
		fieldvalueuser.Table,
		func(c *generated.Client) *generated.FieldValueUserDelete { return c.FieldValueUser.Delete() },
		func(c *generated.Client) *generated.FieldValueUserQuery { return c.FieldValueUser.Query() })
	referenceMultiValues = newValueTable[predicate.FieldValueReferenceMulti, *generated.FieldValueReferenceMultiDelete, *generated.FieldValueReferenceMultiQuery, *generated.FieldValueReferenceMultiSelect](
		fieldvaluereferencemulti.Table,
		func(c *generated.Client) *generated.FieldValueReferenceMultiDelete {
			return c.FieldValueReferenceMulti.Delete()
		},
		func(c *generated.Client) *generated.FieldValueReferenceMultiQuery {
			return c.FieldValueReferenceMulti.Query()
		})
)

// storageValueTable - valueTableOf for a field storage.
func storageValueTable(s entity.FieldStorageDef) (valueTable, bool) {
	return valueTableOf(s.FieldType, s.Multiple())
}

// NodeIDsWithField - the nodes having any value of the field, a node with
// several values repeated; false for a field type without a value table.
func NodeIDsWithField(ctx context.Context, client *generated.Client, fs entity.FieldStorageDef) ([]int, bool, error) {
	t, ok := storageValueTable(fs)
	if !ok {
		return nil, false, nil
	}
	ids, err := t.nodeIDs(ctx, client, valueOfField(fs.ID))
	return ids, true, err
}
