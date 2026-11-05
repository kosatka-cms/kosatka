package service

import (
	"context"
	"errors"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// accessAuth - the view and edit checks: viewable - published nodes,
// editable - the nodes of editor.
type accessAuth struct {
	AuthService
	editor int
}

func (accessAuth) CanViewNode(_ context.Context, n *entity.Node, _ *entity.User) bool {
	return n.IsPublic()
}

func (a accessAuth) CanEditNode(_ context.Context, _ *entity.Node, u *entity.User) bool {
	return u != nil && u.ID == a.editor
}

// TestPrivateFileAccess - the uploader, whoever may see one of the items
// holding the file, and whoever may edit one, may download it; nobody else.
func TestPrivateFileAccess(t *testing.T) {
	nodes := map[int]*entity.Node{
		1: {ID: 1, Status: 0},
		2: {ID: 2, Status: 1},
	}
	holding := map[int][]int{10: {1}, 11: {1, 2}, 12: {9}}
	a := PrivateFileAccess{
		NodesWithFile: func(_ context.Context, id int) ([]int, error) { return holding[id], nil },
		NodeByID: func(_ context.Context, id int) (*entity.Node, error) {
			if n, ok := nodes[id]; ok {
				return n, nil
			}
			return nil, entity.ErrEntityNotFound
		},
		Auth: accessAuth{editor: 5},
	}
	uploader, editor, other := &entity.User{ID: 3}, &entity.User{ID: 5}, &entity.User{ID: 7}
	for name, tc := range map[string]struct {
		file entity.File
		user *entity.User
		want bool
	}{
		"a visitor, unpublished item":       {entity.File{ID: 10}, nil, false},
		"another user, unpublished item":    {entity.File{ID: 10}, other, false},
		"the uploader":                      {entity.File{ID: 10, CreatedBy: uploader}, uploader, true},
		"an editor of the item":             {entity.File{ID: 10}, editor, true},
		"a visitor, one item published":     {entity.File{ID: 11}, nil, true},
		"a file of a deleted item":          {entity.File{ID: 12}, other, false},
		"a file on no item":                 {entity.File{ID: 13}, other, false},
		"the uploader of a file on no item": {entity.File{ID: 13, CreatedBy: uploader}, uploader, true},
	} {
		if got, err := a.CanDownload(context.Background(), tc.user, tc.file); err != nil || got != tc.want {
			t.Errorf("%s: %v %v, want %v", name, got, err, tc.want)
		}
	}
	a.NodesWithFile = func(context.Context, int) ([]int, error) { return nil, errors.New("db down") }
	if ok, err := a.CanDownload(context.Background(), nil, entity.File{ID: 10}); ok || err == nil {
		t.Errorf("a failed lookup: %v %v", ok, err)
	}
}
