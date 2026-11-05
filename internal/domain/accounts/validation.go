package accounts

import (
	"context"
	"net/mail"
	"regexp"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// UsernameRegex - allowed username characters: letters, digits, spaces,
// dots, dashes, apostrophes and underscores (registration, admin user
// creation and profile edit).
var UsernameRegex = regexp.MustCompile(`^[\p{L}\p{N}\s.\-_']+$`)

// UserLookup - the minimal user lookup for username/email uniqueness
// checks; implemented by both service.UserService and entity.UserRepository.
type UserLookup interface {
	FindByUsername(ctx context.Context, username string) (*entity.User, error)
	Find(ctx context.Context, filter entity.UserFilter) (*entity.User, error)
}

// ValidateUsername checks that the username is present, uses allowed
// characters and is unique. excludeUserID skips that user in the
// uniqueness check (own profile edit); pass 0 for none.
func ValidateUsername(ctx context.Context, lookup UserLookup, username string, excludeUserID int) string {
	if username == "" {
		return i18n.T(ctx, "Username is required")
	}
	if !UsernameRegex.MatchString(username) {
		return i18n.T(ctx, "Spaces are allowed; punctuation is not allowed except for periods, hyphens, apostrophes, and underscores")
	}
	if existing, _ := lookup.FindByUsername(ctx, username); existing != nil && existing.ID != excludeUserID {
		return i18n.T(ctx, "This username is already taken")
	}
	return ""
}

// ValidateEmail checks that the email is present, well-formed and unique.
// excludeUserID skips that user in the uniqueness check; pass 0 for none.
func ValidateEmail(ctx context.Context, lookup UserLookup, email string, excludeUserID int) string {
	if email == "" {
		return i18n.T(ctx, "Email address is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return i18n.T(ctx, "Enter a valid email address")
	}
	if existing, _ := lookup.Find(ctx, entity.UserFilter{Email: email}); existing != nil && existing.ID != excludeUserID {
		return i18n.T(ctx, "This email address is already registered")
	}
	return ""
}

// ValidatePassword checks presence, minimum length and the confirmation
// match; an empty string means no error for that field. Messages are in
// ctx's language.
func ValidatePassword(ctx context.Context, password, passwordConfirm string) (passwordErr, confirmErr string) {
	if password == "" {
		passwordErr = i18n.T(ctx, "Password is required")
	} else if utf8.RuneCountInString(password) < entity.MinPasswordLength {
		passwordErr = i18n.Plural(ctx, entity.MinPasswordLength, "Password must be at least @count character", "Password must be at least @count characters")
	}
	if password != passwordConfirm {
		confirmErr = i18n.T(ctx, "Passwords do not match")
	}
	return passwordErr, confirmErr
}
