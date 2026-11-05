package i18n

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

// JSKey - a string the site's JavaScript translates with Kosatka.t,
// Kosatka.tc or Kosatka.formatPlural (found at startup by
// internal/i18nextract.ExtractJS).
type JSKey struct {
	Context string
	Source  string
}

var (
	jsMu     sync.Mutex
	jsKeys   []JSKey
	jsScript = map[string][]byte{} // lang → script, built on first use
	jsVer    = map[string]string{} // lang → JSVersion
)

// SetJSKeys sets the strings the JavaScript translates; the per-language
// scripts (JSScript) are rebuilt on next use.
func SetJSKeys(keys []JSKey) {
	jsMu.Lock()
	jsKeys = keys
	jsScript = map[string][]byte{}
	jsVer = map[string]string{}
	jsMu.Unlock()
}

func resetJS() {
	jsMu.Lock()
	jsScript = map[string][]byte{}
	jsVer = map[string]string{}
	jsMu.Unlock()
}

// JSScript - the script served at /i18n/<lang>.js: the translations of
// the JS strings in lang (window.kosatkaTranslations) and the runtime
// (Kosatka.t, Kosatka.tc, Kosatka.formatPlural). Like Go's
// T, the functions return plain text: placeholder values are not escaped.
func JSScript(lang string) []byte {
	jsMu.Lock()
	defer jsMu.Unlock()
	if b, ok := jsScript[lang]; ok {
		return b
	}
	tr := Current()
	type dict struct {
		Lang     string                    `json:"lang"`
		Plural   []string                  `json:"p"`
		Strings  map[string]any            `json:"s"`
		Contexts map[string]map[string]any `json:"c"`
	}
	d := dict{Lang: lang, Plural: PluralCategories(lang), Strings: map[string]any{}, Contexts: map[string]map[string]any{}}
	for _, k := range jsKeys {
		e, ok := tr.lookup(lang, k.Context, k.Source)
		if !ok {
			continue
		}
		var v any = e.text
		if len(e.forms) > 0 {
			v = e.forms
		}
		if k.Context == "" {
			d.Strings[k.Source] = v
			continue
		}
		if d.Contexts[k.Context] == nil {
			d.Contexts[k.Context] = map[string]any{}
		}
		d.Contexts[k.Context][k.Source] = v
	}
	data, _ := json.Marshal(d)
	b := append(append(append([]byte("window.kosatkaTranslations="), data...), ";\n"...), jsRuntime...)
	jsScript[lang] = b
	return b
}

// JSVersion - a short hash of JSScript(lang), for a cache-busting URL.
func JSVersion(lang string) string {
	jsMu.Lock()
	v, ok := jsVer[lang]
	jsMu.Unlock()
	if ok {
		return v
	}
	sum := sha256.Sum256(JSScript(lang))
	v = hex.EncodeToString(sum[:6])
	jsMu.Lock()
	jsVer[lang] = v
	jsMu.Unlock()
	return v
}

const jsRuntime = `(function () {
  var d = window.kosatkaTranslations || {};
  var K = window.Kosatka = window.Kosatka || {};
  function fill(s, args) {
    if (!args) return s;
    var names = Object.keys(args).sort(function (a, b) { return b.length - a.length; });
    for (var i = 0; i < names.length; i++) s = s.split(names[i]).join(String(args[names[i]]));
    return s;
  }
  function find(ctx, s) {
    var m = ctx ? (d.c || {})[ctx] : d.s;
    return m ? m[s] : undefined;
  }
  function one(ctx, s, args) {
    var v = find(ctx, s);
    if (Array.isArray(v)) v = v[0];
    return fill(typeof v === 'string' && v !== '' ? v : s, args);
  }
  K.lang = d.lang || document.documentElement.lang || 'en';
  K.t = function (s, args) { return one('', s, args); };
  K.tc = function (ctx, s, args) { return one(ctx, s, args); };
  // pluralIndex - the CLDR category of n (Intl.PluralRules) among the
  // language's categories in catalog order (d.p, i18n.PluralCategories);
  // without either - 1 is the first form, the rest the second.
  K.pluralIndex = function (n) {
    n = Math.abs(n);
    var cats = d.p || [];
    if (window.Intl && Intl.PluralRules && cats.length) {
      try {
        var i = cats.indexOf(new Intl.PluralRules(K.lang).select(n));
        if (i >= 0) return i;
      } catch (e) {}
    }
    return n === 1 ? 0 : 1;
  };
  K.formatPlural = function (n, singular, plural, args) {
    var v = find('', singular), s = n === 1 ? singular : plural;
    if (Array.isArray(v) && v.length) s = v[Math.min(K.pluralIndex(n), v.length - 1)];
    var a = { '@count': n };
    for (var k in args || {}) a[k] = args[k];
    return fill(s, a);
  };
})();
`
