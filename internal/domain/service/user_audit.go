package service

import (
	"context"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
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

// AccountChanges - what a save of the profile form changed of the account
// (before, after), as the action log shows it: the name, the e-mail and
// the time zone (empty - the site's) from and to; the password
// (passwordSet - a new one typed), the signature and the picture without
// their values (the words in context: "changed" of a password and of a
// signature read differently in some languages). Its status and role are
// actions of their own
// (AuditUserStatus, AuditUserRole).
func AccountChanges(ctx context.Context, before, after entity.User, passwordSet bool) []audit.Change {
	var out []audit.Change
	add := func(field, from, to string) {
		if from != to {
			out = append(out, audit.Change{Field: i18n.T(ctx, field), From: from, To: to})
		}
	}
	add("Username", before.Username, after.Username)
	add("Email address", before.Email, after.Email)
	zone := func(z string) string {
		if z == "" {
			return i18n.T(ctx, "Site default")
		}
		return z
	}
	add("Time zone", zone(before.Timezone), zone(after.Timezone))
	if passwordSet {
		out = append(out, audit.Change{Field: i18n.T(ctx, "Password"), To: i18n.TC(ctx, "a password", "changed")})
	}
	if before.Signature != after.Signature {
		out = append(out, audit.Change{Field: i18n.T(ctx, "Signature"), To: i18n.TC(ctx, "a signature", "changed")})
	}
	switch {
	case after.AvatarUpdateHint == "clear":
		if before.Avatar.URI != "" {
			out = append(out, audit.Change{Field: i18n.T(ctx, "Picture"), To: i18n.TC(ctx, "a picture", "deleted")})
		}
	case after.AvatarUpdateHint != "keep" && after.Avatar.URI != before.Avatar.URI:
		out = append(out, audit.Change{Field: i18n.T(ctx, "Picture"), To: i18n.TC(ctx, "a picture", "uploaded")})
	}
	return out
}

// UserFieldChanges - the profile fields' values a save changed (before:
// the account's values with their fields; after: the form's, by field
// ID - a field the form hasn't is unchanged), from and to, cut to 100
// characters.
func UserFieldChanges(before []entity.UserFieldValue, after map[int]string) []audit.Change {
	var out []audit.Change
	old := map[int]string{}
	for _, v := range before {
		old[v.Field.ID] = v.Value
	}
	for _, v := range before {
		to, ok := after[v.Field.ID]
		if !ok || to == old[v.Field.ID] {
			continue
		}
		out = append(out, audit.Change{Field: v.Field.Label, From: textcut.Ellipsis(old[v.Field.ID], 100), To: textcut.Ellipsis(to, 100)})
	}
	return out
}

// AuditUserUpdate records the account's changes (AccountChanges,
// UserFieldChanges) for the action log; none - nothing.
func AuditUserUpdate(ctx context.Context, u entity.User, changes []audit.Change) {
	if len(changes) == 0 {
		return
	}
	audit.Record(ctx, audit.Action{Type: audit.UserUpdate, Object: userObject(u, true), Changes: changes})
}

// AuditUserPasswordReset records the password of u set by a reset link
// for the action log. Its owner did it - the link came to the account's
// address - while not signed in: the actor is u.
func AuditUserPasswordReset(ctx context.Context, u entity.User) {
	audit.Record(audit.WithUser(ctx, u.ID), audit.Action{Type: audit.UserPasswordReset, Object: userObject(u, true)})
}

// AuditUserDeleted records a deleted account for the action log.
func AuditUserDeleted(ctx context.Context, u entity.User) {
	audit.Record(ctx, audit.Action{Type: audit.UserDelete, Object: userObject(u, false)})
}
