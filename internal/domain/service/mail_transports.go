package service

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/mailsender"
)

// MailTransport - a way of sending the site's email that a module adds
// besides the engine's SMTP (modulekit.AddMailTransport): an HTTP API of
// a mail service, say. The Mail page offers it while its module is
// enabled, with its settings.
type MailTransport struct {
	// Name - the saved choice of it: lower case letters, digits and _, at
	// most 16, not "smtp".
	Name string
	// Module - the module adding it: the transport is offered and sends
	// while the module is enabled.
	Module string
	// Title - its name on the Mail page (a service's name, as it is).
	Title string
	// Fields - its settings, in the page's order.
	Fields []MailTransportField
	// New - the sender of the settings (values by field name, every
	// required one set) from the sender address from; timeout bounds a
	// send.
	New func(values map[string]string, from string, timeout time.Duration) (mailsender.EmailSender, error)
}

// MailTransportField - a setting of a MailTransport.
type MailTransportField struct {
	// Name - the setting's key: lower case letters, digits and _.
	Name string
	// Label and Help - shown on the Mail page, translated (i18n.N).
	Label, Help string
	// Secret - a key or a password: never shown back, an empty field keeps
	// the saved one.
	Secret bool
	// Required - the transport sends only with it set.
	Required bool
	// Env - the environment variable setting it from the server's config:
	// a transport whose required settings all are set so, with the
	// config's sender address, is the site's, and the admin can't change
	// it (as the config's SMTP server). Without that address the admin
	// chooses the transport and the sender, and the environment's value
	// is used (MailService.SetTransportEnv).
	Env string
}

var (
	mailTransportsMu sync.RWMutex
	mailTransports   []MailTransport
	mailNameRe       = regexp.MustCompile(`^[a-z0-9_]+$`)
)

// RegisterMailTransport adds a module's way of sending (from its init): a
// mistake in it - a bad or taken name, no New, a bad field - panics.
func RegisterMailTransport(t MailTransport) {
	mailTransportsMu.Lock()
	defer mailTransportsMu.Unlock()
	switch {
	case t.Name == entity.MailTransportSMTP || len(t.Name) > 16 || !mailNameRe.MatchString(t.Name):
		panic(fmt.Sprintf("mail transport %q: the name is not allowed", t.Name))
	case t.Module == "" || t.Title == "" || t.New == nil:
		panic(fmt.Sprintf("mail transport %q: no module, title or New", t.Name))
	}
	for _, other := range mailTransports {
		if other.Name == t.Name {
			panic(fmt.Sprintf("mail transport %q is registered twice", t.Name))
		}
	}
	seen := map[string]bool{}
	for _, f := range t.Fields {
		if len(f.Name) > 64 || !mailNameRe.MatchString(f.Name) || seen[f.Name] || f.Label == "" {
			panic(fmt.Sprintf("mail transport %q: the field %q is not allowed", t.Name, f.Name))
		}
		seen[f.Name] = true
	}
	mailTransports = append(mailTransports, t)
}

// UnregisterMailTransport removes a way of sending (tests).
func UnregisterMailTransport(name string) {
	mailTransportsMu.Lock()
	defer mailTransportsMu.Unlock()
	for i, t := range mailTransports {
		if t.Name == name {
			mailTransports = append(mailTransports[:i:i], mailTransports[i+1:]...)
			return
		}
	}
}

// MailTransports - the modules' ways of sending, in the order they
// registered.
func MailTransports() []MailTransport {
	mailTransportsMu.RLock()
	defer mailTransportsMu.RUnlock()
	return append([]MailTransport(nil), mailTransports...)
}

// MailTransportByName - the module's way of sending named name.
func MailTransportByName(name string) (MailTransport, bool) {
	mailTransportsMu.RLock()
	defer mailTransportsMu.RUnlock()
	for _, t := range mailTransports {
		if t.Name == name {
			return t, true
		}
	}
	return MailTransport{}, false
}

// MailTransportsOfModule - the names of the module's ways of sending.
func MailTransportsOfModule(module string) []string {
	var names []string
	for _, t := range MailTransports() {
		if t.Module == module {
			names = append(names, t.Name)
		}
	}
	return names
}

// mailTransportOn - whether the transport may send now: SMTP, or a
// module's whose module is enabled.
func mailTransportOn(name string) bool {
	if name == entity.MailTransportSMTP {
		return true
	}
	t, ok := MailTransportByName(name)
	return ok && ModuleEnabled(t.Module)
}

// MailTransportTitle - the transport's name on the Mail page: SMTP, a
// module's title, or the saved name itself when the binary lacks it.
func MailTransportTitle(name string) string {
	if name == entity.MailTransportSMTP {
		return "SMTP"
	}
	if t, ok := MailTransportByName(name); ok {
		return t.Title
	}
	return name
}

// ConfiguredMailTransport - the module's way of sending the server's
// config sets: the first whose required settings all are in the
// environment (getenv), with the values of its settings found there.
func ConfiguredMailTransport(getenv func(string) string) (string, map[string]string, bool) {
	for _, t := range MailTransports() {
		values := map[string]string{}
		complete, found := true, false
		for _, f := range t.Fields {
			v := ""
			if f.Env != "" {
				v = strings.TrimSpace(getenv(f.Env))
			}
			if v != "" {
				values[f.Name], found = v, true
			} else if f.Required {
				complete = false
			}
		}
		if complete && found {
			return t.Name, values, true
		}
	}
	return "", nil, false
}

// MailTransportEnv - the settings the server's environment (getenv) gives
// the modules' ways of sending (MailTransportField.Env): transport →
// setting → value, the ones set only.
func MailTransportEnv(getenv func(string) string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, t := range MailTransports() {
		for _, f := range t.Fields {
			if f.Env == "" {
				continue
			}
			if v := strings.TrimSpace(getenv(f.Env)); v != "" {
				if out[t.Name] == nil {
					out[t.Name] = map[string]string{}
				}
				out[t.Name][f.Name] = v
			}
		}
	}
	return out
}

// mailTransportReady - whether the module's transport t has its required
// settings in values.
func mailTransportReady(t MailTransport, values map[string]string) bool {
	for _, f := range t.Fields {
		if f.Required && strings.TrimSpace(values[f.Name]) == "" {
			return false
		}
	}
	return true
}
