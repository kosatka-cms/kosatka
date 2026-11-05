package entity

import (
	"context"
	"strings"
	"sync/atomic"
)

// Registration modes (stored in account_settings.registration_mode).
const (
	RegistrationModeAdminOnly        = "admin_only"
	RegistrationModeVisitors         = "visitors"
	RegistrationModeVisitorsApproval = "visitors_approval"
)

// registrationClosed - visitors may not sign up (the account settings'
// mode is admin_only); the zero value - open, the default mode's.
var registrationClosed atomic.Bool

// SetRegistrationMode makes mode the process's (at start and on saving
// the account settings), for the pages' "Register" links.
func SetRegistrationMode(mode string) {
	registrationClosed.Store(mode == RegistrationModeAdminOnly)
}

// RegistrationOpen - whether visitors may sign up (SetRegistrationMode).
func RegistrationOpen() bool { return !registrationClosed.Load() }

// AccountSettings holds registration policy and all email templates (single DB row).
type AccountSettings struct {
	RegistrationMode         string
	RequireEmailVerification bool
	// UserTimezones - users may set their own time zone in the profile; off -
	// every page uses the site's zone.
	UserTimezones              bool
	RegistrationBlockedDomains string
	// AvatarAllowedExtensions/AvatarMaxFileSizeBytes - the avatar upload
	// policy, independent of the sitewide UploadSettings. nil/0 means unset -
	// see ResolveAvatarUploadPolicy. The resize target is the fixed
	// MaxAvatarDimension constant (a layout concern, not validation).
	AvatarAllowedExtensions []string
	AvatarMaxFileSizeBytes  int
	// TwoFactorRoles - the roles (ids) whose users must use two-factor
	// sign-in; TwoFactorIssuer - the site's name in the authenticator
	// ("" - the site's name).
	TwoFactorRoles  []int
	TwoFactorIssuer string
	// CancelContent - what a user deleting their own account (the
	// permission "cancel own account") leaves: CancelKeepContent - their
	// content, without an author (their comments go with the account);
	// CancelDeleteContent - none.
	CancelContent      string
	SubjWelcomeAdmin   string
	BodyWelcomeAdmin   string
	SubjWelcomePending string
	BodyWelcomePending string
	SubjWelcomeOpen    string
	BodyWelcomeOpen    string
	SubjActivation     string
	BodyActivation     string
	SubjPasswordReset  string
	BodyPasswordReset  string
	SubjEmailConfirm   string
	BodyEmailConfirm   string
	SubjAdminNotify    string
	BodyAdminNotify    string
}

// AccountSettingsRepository persists account / email template settings.
type AccountSettingsRepository interface {
	Get(ctx context.Context) (*AccountSettings, error)
	Update(ctx context.Context, s *AccountSettings) error
}

// AvatarAllowedExtensionsString - space-separated, the format the admin form
// posts back (same as FieldInstanceDef.AllowedFileExtensionsString).
func (s AccountSettings) AvatarAllowedExtensionsString() string {
	return strings.Join(s.AvatarAllowedExtensions, " ")
}

// AvatarMaxFileSizeMB - AvatarMaxFileSizeBytes in MB, for the admin form's
// megabyte-denominated input. 0 when unset.
func (s AccountSettings) AvatarMaxFileSizeMB() float64 {
	return float64(s.AvatarMaxFileSizeBytes) / 1024 / 1024
}

// BlockedDomains - RegistrationBlockedDomains as host suffixes: one domain
// per line, # starts a comment line, a leading @ is stripped, empty lines
// are skipped; lowercase.
func (s AccountSettings) BlockedDomains() []string {
	var out []string
	for _, line := range strings.Split(s.RegistrationBlockedDomains, "\n") {
		d := strings.TrimSpace(line)
		if d == "" || strings.HasPrefix(d, "#") {
			continue
		}
		if d = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(d, "@"))); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// EmailDomainBlocked reports whether the email's domain may not register:
// a blocked domain or its subdomain (blocked "foo.com" matches "foo.com"
// and "x.foo.com").
func (s AccountSettings) EmailDomainBlocked(email string) bool {
	local, host, ok := strings.Cut(strings.TrimSpace(strings.ToLower(email)), "@")
	if !ok || local == "" || host == "" {
		return false
	}
	for _, b := range s.BlockedDomains() {
		if host == b || strings.HasSuffix(host, "."+b) {
			return true
		}
	}
	return false
}

// What a user's own account deletion does with their content
// (AccountSettings.CancelContent).
const (
	CancelKeepContent   = "keep"
	CancelDeleteContent = "delete"
)
