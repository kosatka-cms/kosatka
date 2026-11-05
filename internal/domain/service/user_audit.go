package service

import (
	"context"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// userObject - the user u as an action log's object; withURL - the
// account still exists.
func userObject(u entity.User, withURL bool) audit.Object {
	o := audit.Object{Kind: audit.KindUser, ID: strconv.Itoa(u.ID), Label: u.Username}
	if withURL {
		o.URL = "/user/" + strconv.Itoa(u.ID) + "/edit"
	}
	return o
}

// AuditUserStatus records a user's new status for the action log: blocked,
// unblocked, or activated (a registration approved); the same status -
// nothing.
func AuditUserStatus(ctx context.Context, u entity.User, from, to int) {
	var t string
	switch {
	case from == to:
		return
	case to == entity.UserStatusBlocked:
		t = audit.UserBlock
	case from == entity.UserStatusPending:
		t = audit.UserActivate
	default:
		t = audit.UserUnblock
	}
	audit.Record(ctx, audit.Action{Type: t, Object: userObject(u, true)})
}

// AuditUserRole records a user's new role for the action log (from, to -
// the roles' names as shown).
func AuditUserRole(ctx context.Context, u entity.User, from, to string) {
	if from == to {
		return
	}
	audit.Record(ctx, audit.Action{Type: audit.UserRole, Object: userObject(u, true), Changes: []audit.Change{
		{Field: i18n.T(ctx, "Role"), From: from, To: to},
	}})
}

// AuditUserDeleted records a deleted account for the action log.
func AuditUserDeleted(ctx context.Context, u entity.User) {
	audit.Record(ctx, audit.Action{Type: audit.UserDelete, Object: userObject(u, false)})
}
