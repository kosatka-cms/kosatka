package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToUserEntity converts a generated User to its domain entity, with the
// avatar's list-style address and the role when loaded.
func ToUserEntity(u *generated.User) entity.User {
	user := entity.User{
		ID:            u.ID,
		Username:      u.Username,
		RoleID:        u.RoleID,
		Email:         u.Email,
		PasswordHash:  u.PasswordHash,
		Slug:          u.Slug,
		Timezone:      u.Timezone,
		Metadata:      u.Metadata,
		Signature:     u.Signature,
		Status:        u.Status,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}

	user.LastActivity = u.LastActivity
	user.EmailVerified = u.EmailVerified
	user.SessionVersion = u.SessionVersion

	// No avatar: none (the pages show the placeholder).
	if u.Avatar != "" {
		user.Avatar = imageurl.Styled(entity.File{URI: u.Avatar}, "user_avatar_list")
		user.AvatarStored = u.Avatar
	}

	// Role, if loaded.
	if u.Edges.Role != nil {
		role := ToRoleEntity(u.Edges.Role)
		user.Role = &role
	}

	return user
}
