package accounts

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// stubAccountRepo - the account settings row in memory.
type stubAccountRepo struct{ acc entity.AccountSettings }

func (r *stubAccountRepo) Get(context.Context) (*entity.AccountSettings, error) {
	a := r.acc
	return &a, nil
}

func (r *stubAccountRepo) Update(_ context.Context, a *entity.AccountSettings) error {
	r.acc = *a
	return nil
}

// TestRegisterMails - the account emails on the Mail page: the
// invitation's text the page's (a default), each other one's its own
// pair of account settings fields, a blank one the default's; saving one
// leaves the others; the account settings page's save keeps the texts it
// no longer has.
func TestRegisterMails(t *testing.T) {
	ctx := context.Background()
	service.ResetMailDefs()
	t.Cleanup(service.ResetMailDefs)
	repo := &stubAccountRepo{acc: entity.AccountSettings{SubjActivation: "Active!", BodyPasswordReset: "Reset body"}}
	RegisterMails(repo)

	if n := len(service.MailDefs()); n != 8 {
		t.Fatalf("%d account emails, want 8", n)
	}
	if invite, ok := service.MailDefByKey(MailInvite); !ok || invite.Optional || invite.Load != nil || invite.Default.Body == "" {
		t.Errorf("the invitation: %+v", invite)
	}
	activation, _ := service.MailDefByKey(MailActivation)
	got, err := activation.Load(ctx)
	if err != nil || got.Subject != "Active!" || got.Body != DefaultAccountSettings().BodyActivation {
		t.Errorf("activation: %+v %v", got, err)
	}
	reset, _ := service.MailDefByKey(MailPasswordReset)
	if reset.Optional || !activation.Optional {
		t.Errorf("optional: reset %v, activation %v", reset.Optional, activation.Optional)
	}
	if err := reset.Save(ctx, entity.MailTemplate{Subject: "New", Body: ""}); err != nil {
		t.Fatal(err)
	}
	if repo.acc.SubjPasswordReset != "New" || repo.acc.BodyPasswordReset != "" || repo.acc.SubjActivation != "Active!" {
		t.Errorf("after saving the reset e-mail: %+v", repo.acc)
	}

	// The account settings page: its fields saved, the texts kept.
	if err := NewAccountPolicyService(repo).Save(ctx, &entity.AccountSettings{RegistrationMode: entity.RegistrationModeAdminOnly}); err != nil {
		t.Fatal(err)
	}
	if repo.acc.RegistrationMode != entity.RegistrationModeAdminOnly || repo.acc.SubjActivation != "Active!" || repo.acc.SubjPasswordReset != "New" {
		t.Errorf("after the account settings: %+v", repo.acc)
	}
	entity.SetRegistrationMode(entity.RegistrationModeVisitors)
}
