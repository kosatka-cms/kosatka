package i18n

import (
	"strings"
	"testing"
)

func TestJSScript(t *testing.T) {
	SetTranslator(testTranslator(t))
	t.Cleanup(func() { SetTranslator(&Translator{chain: map[string][]catalog{}}); SetJSKeys(nil) })
	SetJSKeys([]JSKey{{Source: "Save"}, {Source: "@count view"}, {Source: "Not translated"}})
	s := string(JSScript("ru"))
	if !strings.HasPrefix(s, "window.kosatkaTranslations={") || !strings.Contains(s, "Kosatka.t") && !strings.Contains(s, "K.t =") {
		t.Fatalf("script: %.200s", s)
	}
	if !strings.Contains(s, `"lang":"ru"`) || !strings.Contains(s, `"p":["one","few","many"]`) || !strings.Contains(s, "Intl.PluralRules") || strings.Contains(s, "Not translated") {
		t.Errorf("dictionary: %.300s", s)
	}
	v := JSVersion("ru")
	if len(v) != 12 || JSVersion("ru") != v {
		t.Errorf("version %q", v)
	}
	SetJSKeys([]JSKey{{Source: "Save"}})
	if JSVersion("ru") == v {
		t.Error("version must change with the keys")
	}
}
