package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/mailsender"
)

// The test's way of sending and its module (unknown to the engine: on).
const (
	testTransport       = "testapi"
	testTransportModule = "testapi_module"
)

// useTestMailTransport registers the transport testapi for the test: a
// required secret "key" (KOSATKA_TEST_MAIL_KEY), an optional "region"
// (KOSATKA_TEST_MAIL_REGION); its sender records the emails, a key "bad"
// fails to build one.
func useTestMailTransport(t *testing.T) *[]*recordingSender {
	t.Helper()
	built := &[]*recordingSender{}
	RegisterMailTransport(MailTransport{
		Name: testTransport, Module: testTransportModule, Title: "Test API",
		Fields: []MailTransportField{
			{Name: "key", Label: "API key", Secret: true, Required: true, Env: "KOSATKA_TEST_MAIL_KEY"},
			{Name: "region", Label: "Region", Env: "KOSATKA_TEST_MAIL_REGION"},
		},
		New: func(values map[string]string, from string, _ time.Duration) (mailsender.EmailSender, error) {
			if values["key"] == "bad" {
				return nil, errors.New("a bad key")
			}
			s := &recordingSender{built: entity.MailSettings{FromAddress: from, TransportSettings: map[string]map[string]string{testTransport: values}}}
			*built = append(*built, s)
			return s, nil
		},
	})
	t.Cleanup(func() { UnregisterMailTransport(testTransport) })
	return built
}

// TestRegisterMailTransportChecks - a mistake in a transport panics: a
// name not allowed or taken, no module or New, a bad field.
func TestRegisterMailTransportChecks(t *testing.T) {
	useTestMailTransport(t)
	newSender := func(map[string]string, string, time.Duration) (mailsender.EmailSender, error) { return nil, nil }
	for name, tr := range map[string]MailTransport{
		"smtp":           {Name: "smtp", Module: "m", Title: "T", New: newSender},
		"upper case":     {Name: "Api", Module: "m", Title: "T", New: newSender},
		"too long":       {Name: "a_very_long_transport", Module: "m", Title: "T", New: newSender},
		"taken":          {Name: testTransport, Module: "m", Title: "T", New: newSender},
		"no module":      {Name: "other", Title: "T", New: newSender},
		"no New":         {Name: "other", Module: "m", Title: "T"},
		"a field's name": {Name: "other", Module: "m", Title: "T", New: newSender, Fields: []MailTransportField{{Name: "a.b", Label: "L"}}},
		"a field twice":  {Name: "other", Module: "m", Title: "T", New: newSender, Fields: []MailTransportField{{Name: "a", Label: "L"}, {Name: "a", Label: "L"}}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: registered", name)
					UnregisterMailTransport(tr.Name)
				}
			}()
			RegisterMailTransport(tr)
		}()
	}
	if names := MailTransportsOfModule(testTransportModule); len(names) != 1 || names[0] != testTransport {
		t.Errorf("the module's transports: %v", names)
	}
	if MailTransportTitle(testTransport) != "Test API" || MailTransportTitle("smtp") != "SMTP" || MailTransportTitle("gone") != "gone" {
		t.Error("titles")
	}
}

// TestConfiguredMailTransport - the server's config sets a module's way
// of sending when its required settings are all in the environment.
func TestConfiguredMailTransport(t *testing.T) {
	useTestMailTransport(t)
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }
	if _, _, ok := ConfiguredMailTransport(getenv); ok {
		t.Error("nothing in the environment")
	}
	env["KOSATKA_TEST_MAIL_REGION"] = "eu"
	if _, _, ok := ConfiguredMailTransport(getenv); ok {
		t.Error("without the required key")
	}
	env["KOSATKA_TEST_MAIL_KEY"] = " k_env "
	name, values, ok := ConfiguredMailTransport(getenv)
	if !ok || name != testTransport || values["key"] != "k_env" || values["region"] != "eu" {
		t.Errorf("%q %v %v", name, values, ok)
	}
}

// TestMailServiceModuleTransport - a module's transport sends through
// its sender while its module is enabled; disabled, nothing is sent and
// the status says so; a saved transport the binary lacks is reported; a
// sender that can't be built sends nothing.
func TestMailServiceModuleTransport(t *testing.T) {
	built := useTestMailTransport(t)
	t.Cleanup(func() { SetModules(map[string]bool{testTransportModule: true}) })
	ctx := context.Background()
	saved := entity.MailSettings{Enabled: true, Transport: testTransport, FromAddress: "site@example.com", TransportSettings: map[string]map[string]string{testTransport: {"key": "k"}}}
	repo := &stubMailRepo{s: saved}
	svc := NewMailService(repo, entity.MailSettings{}, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Source != MailSourceAdmin || st.TransportTitle != "Test API" || st.ModuleOff || st.Missing != "" {
		t.Fatalf("status: %+v", st)
	}
	if err := svc.SendTest("a@example.com", "s", "b"); err != nil || len(*built) != 1 || len((*built)[0].sent) != 1 || (*built)[0].built.FromAddress != "site@example.com" {
		t.Fatalf("sent: %v %+v", err, *built)
	}

	SetModules(map[string]bool{testTransportModule: false})
	if st := svc.Status(); !st.ModuleOff {
		t.Errorf("module off: %+v", st)
	}
	if err := svc.SendTest("a@example.com", "s", "b"); !errors.Is(err, ErrMailOff) || len((*built)[0].sent) != 1 {
		t.Errorf("sent with the module off: %v", err)
	}
	SetModules(map[string]bool{testTransportModule: true})
	if err := svc.SendTest("a@example.com", "s", "b"); err != nil || len((*built)[0].sent) != 2 {
		t.Errorf("the module on again: %v", err)
	}

	repo.s.TransportSettings[testTransport]["key"] = "bad"
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendTest("a@example.com", "s", "b"); !errors.Is(err, ErrMailOff) {
		t.Errorf("a sender that can't be built: %v", err)
	}

	repo.s.Transport = "gone"
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Missing != "gone" || st.Source != MailSourceNone {
		t.Errorf("a transport the binary lacks: %+v", st)
	}
}

// TestMailServiceTransportEnv - a key in the environment without the
// config's sender address: no server transport, the admin's choice of the
// transport and a sender sends with the environment's key (it wins over a
// saved one); the saved settings aren't changed; EnvTransport names the
// transport the environment sets, EnvSetting its settings.
func TestMailServiceTransportEnv(t *testing.T) {
	built := useTestMailTransport(t)
	ctx := context.Background()
	env := MailTransportEnv(func(k string) string {
		return map[string]string{"KOSATKA_TEST_MAIL_KEY": " k_env "}[k]
	})
	if env[testTransport]["key"] != "k_env" || len(env[testTransport]) != 1 {
		t.Fatalf("MailTransportEnv: %v", env)
	}

	repo := &stubMailRepo{s: entity.MailSettings{Enabled: true}}
	svc := NewMailService(repo, entity.MailSettings{}, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	if svc.EnvTransport() != "" {
		t.Error("EnvTransport before SetTransportEnv")
	}
	svc.SetTransportEnv(env)
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Source != MailSourceNone {
		t.Fatalf("nothing chosen: %+v", st)
	}
	if svc.EnvTransport() != testTransport || !svc.EnvSetting(testTransport, "key") || svc.EnvSetting(testTransport, "region") {
		t.Errorf("EnvTransport %q, EnvSetting key %v region %v", svc.EnvTransport(), svc.EnvSetting(testTransport, "key"), svc.EnvSetting(testTransport, "region"))
	}

	repo.s = entity.MailSettings{Enabled: true, Transport: testTransport, FromAddress: "site@example.com",
		TransportSettings: map[string]map[string]string{testTransport: {"key": "k_saved", "region": "eu"}}}
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Source != MailSourceAdmin || st.Transport != testTransport {
		t.Fatalf("the admin's transport: %+v", st)
	}
	if err := svc.SendTest("a@example.com", "s", "b"); err != nil || len(*built) == 0 {
		t.Fatalf("sent: %v", err)
	}
	got := (*built)[len(*built)-1].built.TransportSettings[testTransport]
	if got["key"] != "k_env" || got["region"] != "eu" {
		t.Errorf("the sender's settings: %v, want the environment's key and the saved region", got)
	}
	if repo.s.TransportSettings[testTransport]["key"] != "k_saved" {
		t.Error("the saved settings changed")
	}

	repo.s.TransportSettings = nil
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.Status(); st.Source != MailSourceAdmin {
		t.Errorf("no key saved, the environment's: %+v", st)
	}
}
