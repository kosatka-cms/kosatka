package accounts

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// fakeUserLookup is an in-memory UserLookup for testing ValidateUsername/ValidateEmail.
type fakeUserLookup struct {
	byUsername map[string]*entity.User
	byEmail    map[string]*entity.User
}

func newFakeUserLookup(users ...entity.User) *fakeUserLookup {
	f := &fakeUserLookup{
		byUsername: map[string]*entity.User{},
		byEmail:    map[string]*entity.User{},
	}
	for i := range users {
		u := users[i]
		f.byUsername[u.Username] = &u
		f.byEmail[u.Email] = &u
	}
	return f
}

func (f *fakeUserLookup) FindByUsername(_ context.Context, username string) (*entity.User, error) {
	if u, ok := f.byUsername[username]; ok {
		return u, nil
	}
	return nil, nil
}

func (f *fakeUserLookup) Find(_ context.Context, filter entity.UserFilter) (*entity.User, error) {
	if u, ok := f.byEmail[filter.Email]; ok {
		return u, nil
	}
	return nil, nil
}

func TestValidateUsername(t *testing.T) {
	lookup := newFakeUserLookup(entity.User{ID: 1, Username: "taken", Email: "taken@example.com"})

	tests := []struct {
		name          string
		username      string
		excludeUserID int
		wantErr       bool
	}{
		{"empty", "", 0, true},
		{"invalid characters", "bad@name!", 0, true},
		{"already taken", "taken", 0, true},
		{"taken but excluded (self edit)", "taken", 1, false},
		{"valid new username", "new-user_2", 0, false},
		{"valid with spaces and apostrophe", "O'Brien Jr.", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateUsername(context.Background(), lookup, tt.username, tt.excludeUserID)
			if (got != "") != tt.wantErr {
				t.Errorf("ValidateUsername(%q, exclude=%d) = %q, wantErr=%v", tt.username, tt.excludeUserID, got, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	lookup := newFakeUserLookup(entity.User{ID: 1, Username: "taken", Email: "taken@example.com"})

	tests := []struct {
		name          string
		email         string
		excludeUserID int
		wantErr       bool
	}{
		{"empty", "", 0, true},
		{"malformed", "not-an-email", 0, true},
		{"already registered", "taken@example.com", 0, true},
		{"already registered but excluded (self edit)", "taken@example.com", 1, false},
		{"valid new email", "fresh@example.com", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateEmail(context.Background(), lookup, tt.email, tt.excludeUserID)
			if (got != "") != tt.wantErr {
				t.Errorf("ValidateEmail(%q, exclude=%d) = %q, wantErr=%v", tt.email, tt.excludeUserID, got, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name            string
		password        string
		confirm         string
		wantPasswordErr bool
		wantConfirmErr  bool
	}{
		{"empty password", "", "", true, false},
		{"too short", "abc12", "abc12", true, false},
		{"mismatch", "longenough", "different", false, true},
		{"valid", "longenough", "longenough", false, false},
		{"minimum length exactly", "123456", "123456", false, false},
		// Characters, not bytes: three Cyrillic letters are six bytes.
		{"short in Cyrillic", "абв", "абв", true, false},
		{"six Cyrillic letters", "пароль", "пароль", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passwordErr, confirmErr := ValidatePassword(context.Background(), tt.password, tt.confirm)
			if (passwordErr != "") != tt.wantPasswordErr {
				t.Errorf("ValidatePassword(%q, %q) passwordErr = %q, want error=%v", tt.password, tt.confirm, passwordErr, tt.wantPasswordErr)
			}
			if (confirmErr != "") != tt.wantConfirmErr {
				t.Errorf("ValidatePassword(%q, %q) confirmErr = %q, want error=%v", tt.password, tt.confirm, confirmErr, tt.wantConfirmErr)
			}
		})
	}
}

func TestUsernameRegex(t *testing.T) {
	// Cyrillic on purpose: Cyrillic usernames are allowed.
	valid := []string{"john", "John Doe", "jane_doe", "o'brien", "a-b.c_d", "Пользователь 42"}
	invalid := []string{"bad@name", "hi!", "user#1", "<script>"}

	for _, v := range valid {
		if !UsernameRegex.MatchString(v) {
			t.Errorf("expected %q to be a valid username", v)
		}
	}
	for _, v := range invalid {
		if UsernameRegex.MatchString(v) {
			t.Errorf("expected %q to be an invalid username", v)
		}
	}
}
