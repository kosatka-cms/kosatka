package interfaces

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeTagTermsUpdater struct {
	updateErr error
	fieldErr  error

	updatedNodeID  int
	updatedTermIDs []int

	fieldTermIDs []int // what FieldTermIDsForNode returns
}

func (f *fakeTagTermsUpdater) FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error) {
	return f.fieldTermIDs, f.fieldErr
}

func (f *fakeTagTermsUpdater) UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error {
	f.updatedNodeID = nodeID
	f.updatedTermIDs = termIDs
	return f.updateErr
}

func TestSaveContentTaxonomy_NoTerms(t *testing.T) {
	fake := &fakeTagTermsUpdater{}

	if err := saveContentTaxonomy(context.Background(), fake, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.updatedNodeID != 1 || len(fake.updatedTermIDs) != 0 {
		t.Errorf("updated %d with %v, want node 1 with no terms", fake.updatedNodeID, fake.updatedTermIDs)
	}
}

// TestSaveContentTaxonomy_Errors - a failed update is returned; field terms
// failing to load are returned too and the index is left as it is (written
// without them, the node would lose its terms).
func TestSaveContentTaxonomy_Errors(t *testing.T) {
	fake := &fakeTagTermsUpdater{updateErr: errors.New("db is on fire")}
	if err := saveContentTaxonomy(context.Background(), fake, 1); !errors.Is(err, fake.updateErr) {
		t.Errorf("a failed update: %v, want it returned", err)
	}

	fake = &fakeTagTermsUpdater{fieldErr: errors.New("db is on fire")}
	if err := saveContentTaxonomy(context.Background(), fake, 1); !errors.Is(err, fake.fieldErr) {
		t.Errorf("failed field terms: %v, want it returned", err)
	}
	if fake.updatedNodeID != 0 {
		t.Errorf("the index was rewritten (node %d, terms %v) though the field terms failed to load", fake.updatedNodeID, fake.updatedTermIDs)
	}
}

// TestSaveContentTaxonomy_IndexesFieldTerms - the node's taxonomy index
// covers every taxonomy field value.
func TestSaveContentTaxonomy_IndexesFieldTerms(t *testing.T) {
	fake := &fakeTagTermsUpdater{fieldTermIDs: []int{40, 41}}

	if err := saveContentTaxonomy(context.Background(), fake, 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.updatedNodeID != 42 {
		t.Errorf("updatedNodeID = %d, want 42", fake.updatedNodeID)
	}
	if want := []int{40, 41}; !reflect.DeepEqual(fake.updatedTermIDs, want) {
		t.Errorf("updatedTermIDs = %v, want %v", fake.updatedTermIDs, want)
	}
}
