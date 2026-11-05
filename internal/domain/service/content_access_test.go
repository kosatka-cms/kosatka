package service

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestContentAccess - a type with view roles is hidden from visitors
// without one of them, unless they may edit its content or bypass the
// access; a change is seen after Invalidate; a failed load denies.
func TestContentAccess(t *testing.T) {
	types := entity.ContentTypeDefs{
		{MachineName: "article"},
		{MachineName: "members", ViewRoles: []string{entity.RoleUser}},
		{MachineName: "staff", ViewRoles: []string{"editor"}},
		{MachineName: "teaser", ViewRoles: []string{entity.RoleAnonymous, "editor"}},
	}
	grants := map[string]map[string]bool{
		"editor":  {},
		"writer":  {PermEditAny("staff"): true},
		"auditor": {PermBypassContentAccess: true},
	}
	loads := 0
	var loadErr error
	a := &ContentAccess{
		Types: func(context.Context) (entity.ContentTypeDefs, error) {
			loads++
			return types, loadErr
		},
		HasPermission: func(_ context.Context, u *entity.User, key string) bool {
			return u != nil && u.Role != nil && grants[u.Role.Name][key]
		},
	}
	ctx := context.Background()
	user := func(role string) *entity.User { return &entity.User{ID: 1, Role: &entity.Role{Name: role}} }
	for name, tc := range map[string]struct {
		user *entity.User
		want []string
	}{
		"a guest":           {nil, []string{"members", "staff"}},
		"a signed-in user":  {user(entity.RoleUser), []string{"staff", "teaser"}},
		"an editor":         {user("editor"), nil},
		"a writer of staff": {user("writer"), []string{"teaser"}},
		"an auditor":        {user("auditor"), nil},
	} {
		got, err := a.HiddenTypes(ctx, tc.user)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v %v, want %v", name, got, err, tc.want)
		}
	}
	if loads != 1 {
		t.Errorf("the types loaded %d times, want once (cached)", loads)
	}
	if ok, _ := a.CanViewType(ctx, nil, "article"); !ok {
		t.Error("an open type hidden from a guest")
	}
	if ok, _ := a.CanViewType(ctx, nil, "members"); ok {
		t.Error("a members' type shown to a guest")
	}

	types[1].ViewRoles = nil
	a.Invalidate()
	if ok, _ := a.CanViewType(ctx, nil, "members"); !ok {
		t.Error("a type opened isn't seen after Invalidate")
	}

	a.Invalidate()
	loadErr = errors.New("db down")
	if ok, err := a.CanViewType(ctx, nil, "article"); ok || err == nil {
		t.Errorf("a failed load: %v %v, want denied with the error", ok, err)
	}
}

// TestCanViewNodeByType - a published node of a type closed to a guest
// is hidden from them; its author who may edit it still sees it.
func TestCanViewNodeByType(t *testing.T) {
	access := &ContentAccess{
		Types: func(context.Context) (entity.ContentTypeDefs, error) {
			return entity.ContentTypeDefs{{MachineName: "members", ViewRoles: []string{entity.RoleUser}}}, nil
		},
		HasPermission: func(context.Context, *entity.User, string) bool { return false },
	}
	s := &DefaultAuthService{access: access}
	node := &entity.Node{Type: "members", Status: 1, CreatedBy: entity.User{ID: 7}}
	if s.CanViewNode(context.Background(), node, nil) {
		t.Error("a guest sees a members' node")
	}
	if !s.CanViewNode(context.Background(), node, &entity.User{ID: 3}) {
		t.Error("a signed-in user doesn't see a members' node")
	}
	open := &entity.Node{Type: "article", Status: 1, CreatedBy: entity.User{ID: 7}}
	if !s.CanViewNode(context.Background(), open, nil) {
		t.Error("a guest doesn't see an open type's node")
	}
}

// TestContentVisitor - what of the content a visitor may not view: the
// closed types; while some item has view roles of its own, the visitor's
// roles, their ID and the types whose every item they view (or all of
// them); the key the same for visitors who view the same.
func TestContentVisitor(t *testing.T) {
	itemRoles := false
	a := &ContentAccess{
		Types: func(context.Context) (entity.ContentTypeDefs, error) {
			return entity.ContentTypeDefs{{MachineName: "article"}, {MachineName: "staff", ViewRoles: []string{"editor"}}}, nil
		},
		ItemRoles: func(context.Context) (bool, error) { return itemRoles, nil },
		HasPermission: func(_ context.Context, u *entity.User, key string) bool {
			if u == nil || u.Role == nil {
				return false
			}
			return u.Role.Name == "writer" && key == PermEditAny("article") || u.Role.Name == "auditor" && key == PermBypassContentAccess
		},
	}
	ctx := context.Background()
	user := func(id int, role string) *entity.User { return &entity.User{ID: id, Role: &entity.Role{Name: role}} }

	if v, err := a.Visitor(ctx, user(3, entity.RoleUser)); err != nil || v.ItemRoles || v.Roles != nil || v.Key() != "staff" {
		t.Errorf("no item with view roles: %+v %v", v, err)
	}
	itemRoles = true
	a.Invalidate()
	for name, tc := range map[string]struct {
		user *entity.User
		want entity.ContentVisitor
	}{
		"a guest":    {nil, entity.ContentVisitor{HiddenTypes: []string{"staff"}, ItemRoles: true, Roles: []string{entity.RoleAnonymous}}},
		"an editor":  {user(4, "editor"), entity.ContentVisitor{ItemRoles: true, Roles: []string{"editor", entity.RoleUser}, UserID: 4}},
		"a writer":   {user(5, "writer"), entity.ContentVisitor{HiddenTypes: []string{"staff"}, ItemRoles: true, Roles: []string{entity.RoleUser, "writer"}, UserID: 5, ItemTypes: []string{"article"}}},
		"an auditor": {user(6, "auditor"), entity.ContentVisitor{ItemRoles: true, AllItems: true}},
	} {
		got, err := a.Visitor(ctx, tc.user)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %+v %v, want %+v", name, got, err, tc.want)
		}
	}
	one, _ := a.Visitor(ctx, user(7, "editor"))
	other, _ := a.Visitor(ctx, user(8, "editor"))
	if one.Key() != other.Key() || one.Shared().UserID != 0 {
		t.Errorf("two editors: keys %q, %q", one.Key(), other.Key())
	}
}

// TestCanViewNodeByItemRoles - a node with view roles of its own is
// hidden from those without one of them, but for its author and those who
// view every content.
func TestCanViewNodeByItemRoles(t *testing.T) {
	access := &ContentAccess{
		Types: func(context.Context) (entity.ContentTypeDefs, error) {
			return entity.ContentTypeDefs{{MachineName: "article"}}, nil
		},
		HasPermission: func(context.Context, *entity.User, string) bool { return false },
	}
	s := &DefaultAuthService{access: access}
	node := &entity.Node{Type: "article", Status: 1, CreatedBy: entity.User{ID: 7}, ViewRoles: []string{"staff"}}
	ctx := context.Background()
	for name, tc := range map[string]struct {
		user *entity.User
		want bool
	}{
		"a guest":               {nil, false},
		"a signed-in user":      {&entity.User{ID: 3}, false},
		"one with the role":     {&entity.User{ID: 4, Role: &entity.Role{Name: "staff"}}, true},
		"the author without it": {&entity.User{ID: 7}, true},
	} {
		if got := s.CanViewNode(ctx, node, tc.user); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
}
