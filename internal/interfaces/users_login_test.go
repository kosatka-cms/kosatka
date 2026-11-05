package interfaces

import (
	"context"
	"testing"
)

func TestApplyLoginFlashMessage(t *testing.T) {
	tests := []struct {
		msg      string
		wantErr  bool
		wantText string
	}{
		{msg: "oauth_error", wantErr: true, wantText: "Could not log in with the social network. Try again or use your username and password."},
		{msg: "register_disabled", wantErr: true, wantText: "New user registration is disabled."},
		{msg: "", wantErr: false},
		{msg: "unknown_code", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			data := map[string]any{}
			applyLoginFlashMessage(context.Background(), data, tt.msg)
			got, ok := data["login_error"]
			if ok != tt.wantErr {
				t.Fatalf("login_error present = %v, want %v", ok, tt.wantErr)
			}
			if tt.wantErr && got != tt.wantText {
				t.Errorf("login_error = %q, want %q", got, tt.wantText)
			}
		})
	}
}
