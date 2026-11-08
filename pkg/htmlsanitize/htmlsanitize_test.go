package htmlsanitize

import (
	"slices"
	"strings"
	"testing"
)

func TestSanitizeStripsScriptTags(t *testing.T) {
	got := Sanitize(`hello <script>alert(1)</script> world`)
	if strings.Contains(got, "<script") || strings.Contains(got, "alert(1)") {
		t.Fatalf("expected script tag and its content to be stripped, got %q", got)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("expected surrounding text to survive, got %q", got)
	}
}

func TestSanitizeStripsEventHandlers(t *testing.T) {
	got := Sanitize(`<img src="x.png" onerror="alert(1)">`)
	if strings.Contains(got, "onerror") || strings.Contains(got, "alert(1)") {
		t.Fatalf("expected onerror handler to be stripped, got %q", got)
	}
}

func TestSanitizeStripsJavascriptURLs(t *testing.T) {
	got := Sanitize(`<a href="javascript:alert(1)">click me</a>`)
	if strings.Contains(got, "javascript:") {
		t.Fatalf("expected javascript: URL to be stripped, got %q", got)
	}
}

func TestSanitizeStripsIframes(t *testing.T) {
	got := Sanitize(`<iframe src="https://evil.example/"></iframe>text`)
	if strings.Contains(got, "<iframe") {
		t.Fatalf("expected iframe to be stripped, got %q", got)
	}
	if !strings.Contains(got, "text") {
		t.Fatalf("expected surrounding text to survive, got %q", got)
	}
}

func TestSanitizeKeepsCommonFormattingTags(t *testing.T) {
	in := `<p><b>bold</b> and <i>italic</i> and <a href="https://example.com">a link</a></p>`
	got := Sanitize(in)
	for _, want := range []string{"<b>bold</b>", "<i>italic</i>", `href="https://example.com"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q to survive sanitization, got %q", want, got)
		}
	}
}

func TestSanitizeKeepsImages(t *testing.T) {
	got := Sanitize(`<img src="https://example.com/pic.jpg" alt="a picture">`)
	if !strings.Contains(got, `src="https://example.com/pic.jpg"`) {
		t.Fatalf("expected image src to survive, got %q", got)
	}
}

func TestSanitizeIsIdempotent(t *testing.T) {
	in := `<p><b>bold</b></p><script>alert(1)</script>`
	once := Sanitize(in)
	twice := Sanitize(once)
	if once != twice {
		t.Fatalf("expected sanitizing an already-sanitized string to be a no-op: once=%q twice=%q", once, twice)
	}
}

func TestSanitizePlainTextUnchanged(t *testing.T) {
	in := "just plain text, no markup here"
	if got := Sanitize(in); got != in {
		t.Fatalf("expected plain text to pass through unchanged, got %q", got)
	}
}

func TestSanitizeWithFormatEmptyTagsStripsAllFormatting(t *testing.T) {
	got := SanitizeWithFormat(`<p><b>bold</b> text</p>`, "", false)
	if strings.Contains(got, "<") {
		t.Fatalf("expected empty allowedTags to strip every tag, got %q", got)
	}
	if !strings.Contains(got, "bold") || !strings.Contains(got, "text") {
		t.Fatalf("expected surrounding text to survive, got %q", got)
	}
}

func TestSanitizeWithFormatNarrowListOnlyPassesListedTags(t *testing.T) {
	got := SanitizeWithFormat(`<b>bold</b> <i>italic</i> <script>alert(1)</script>`, "b", false)
	if !strings.Contains(got, "<b>bold</b>") {
		t.Fatalf("expected allowed <b> to survive, got %q", got)
	}
	if strings.Contains(got, "<i>") {
		t.Fatalf("expected <i>, not in the allowlist, to be stripped, got %q", got)
	}
	if strings.Contains(got, "<script") || strings.Contains(got, "alert(1)") {
		t.Fatalf("expected <script> to be stripped regardless of allowlist, got %q", got)
	}
}

func TestSanitizeWithFormatAllowClassAttributeToggle(t *testing.T) {
	in := `<p class="lead">text</p>`
	withClass := SanitizeWithFormat(in, "p", true)
	if !strings.Contains(withClass, `class="lead"`) {
		t.Fatalf("expected class to survive when allowClass=true, got %q", withClass)
	}
	withoutClass := SanitizeWithFormat(in, "p", false)
	if strings.Contains(withoutClass, "class=") {
		t.Fatalf("expected class to be stripped when allowClass=false, got %q", withoutClass)
	}
}

// TestSanitizeWithFormatAlwaysStripsAttributeBasedXSSVectors: event
// handlers and javascript: URLs are stripped whatever AllowedTags says;
// AllowUnsafe does not affect attributes.
func TestSanitizeWithFormatAlwaysStripsAttributeBasedXSSVectors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"event handler", `<img src="x.png" onerror="alert(1)">`},
		{"javascript URL", `<a href="javascript:alert(1)">click</a>`},
	}
	const wideAllowlist = "a img script style ul ol li table thead tbody tfoot tr td th p b i em strong"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeWithFormat(tt.in, wideAllowlist, true)
			if strings.Contains(got, "onerror") || strings.Contains(got, "alert(1)") || strings.Contains(got, "javascript:") {
				t.Fatalf("expected XSS vector to be stripped regardless of allowlist, got %q", got)
			}
		})
	}
}

// TestSanitizeWithFormatScriptTagOnlyPassesWithExplicitOptIn - <script> is
// stripped unless the format's AllowedTags names it. This admin-only escape
// hatch is guarded by AuthService.CanUseTextFormat (see
// entity.TextFormatDef.AllowUnsafeContent).
func TestSanitizeWithFormatScriptTagOnlyPassesWithExplicitOptIn(t *testing.T) {
	in := `<div><script>if (1 < 2) { alert("a && b"); }</script></div>`

	t.Run("script not in AllowedTags - stripped like any other unlisted tag", func(t *testing.T) {
		got := SanitizeWithFormat(in, "div a img", true)
		if strings.Contains(got, "<script") || strings.Contains(got, "alert") {
			t.Fatalf("expected <script> to be stripped when not explicitly allowed, got %q", got)
		}
	})

	t.Run("script explicitly in AllowedTags - tag and raw content preserved", func(t *testing.T) {
		got := SanitizeWithFormat(in, "div script", true)
		if !strings.Contains(got, `<script>if (1 < 2) { alert("a && b"); }</script>`) {
			t.Fatalf("expected <script> and its raw, unescaped content to survive an explicit opt-in, got %q", got)
		}
	})
}

// TestSanitizeWithFormatScriptOptInKeepsElementID: with script opted in,
// "id" survives so embedded scripts can find their markup
// (document.getElementById).
func TestSanitizeWithFormatScriptOptInKeepsElementID(t *testing.T) {
	in := `<div id="yandex_rtb_R-A-20046448-2"></div><script>document.getElementById('x');</script>`

	t.Run("script in AllowedTags - id attribute survives", func(t *testing.T) {
		got := SanitizeWithFormat(in, "div script", true)
		if !strings.Contains(got, `id="yandex_rtb_R-A-20046448-2"`) {
			t.Fatalf("expected id attribute to survive alongside an explicit script opt-in, got %q", got)
		}
	})

	t.Run("script NOT in AllowedTags - id still stripped, same as any other format", func(t *testing.T) {
		got := SanitizeWithFormat(in, "div", true)
		if strings.Contains(got, "id=") {
			t.Fatalf("expected id attribute to stay stripped without the script/style opt-in, got %q", got)
		}
	})
}

func TestSanitizeWithFormatImageAndLinkTagsGetURLValidation(t *testing.T) {
	got := SanitizeWithFormat(`<a href="https://example.com">link</a> <img src="https://example.com/pic.jpg" alt="pic">`, "a img", false)
	if !strings.Contains(got, `href="https://example.com"`) {
		t.Fatalf("expected valid href to survive, got %q", got)
	}
	if !strings.Contains(got, `src="https://example.com/pic.jpg"`) {
		t.Fatalf("expected valid img src to survive, got %q", got)
	}
}

// TestSanitizeWithFormatKeepsCombinedImageSizeAndFloatStyle: width, height
// and float in one style value survive together (see imgAppearanceStyle).
func TestSanitizeWithFormatKeepsCombinedImageSizeAndFloatStyle(t *testing.T) {
	in := `<img src="https://example.com/pic.jpg" style="width: 220px; height: 335px; float: right;">`
	got := SanitizeWithFormat(in, "img", false)
	if !strings.Contains(got, `style="width: 220px; height: 335px; float: right;"`) {
		t.Fatalf("expected combined width/height/float style to survive unchanged, got %q", got)
	}
}

// TestSanitizeWithFormatKeepsMargin: hand-typed image margins survive,
// shorthand (1-4 values, including "auto") or longhand, alone or combined
// with width/height/float.
func TestSanitizeWithFormatKeepsMargin(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"shorthand two values", `<img src="https://example.com/pic.jpg" style="margin: 0 15px;">`},
		{"shorthand four values", `<img src="https://example.com/pic.jpg" style="margin: 1px 2px 3px 4px;">`},
		{"shorthand with auto (centering)", `<img src="https://example.com/pic.jpg" style="margin: 0 auto;">`},
		{"longhand margin-left", `<img src="https://example.com/pic.jpg" style="margin-left: 15px;">`},
		{"combined with width/height/float", `<img src="https://example.com/pic.jpg" style="width: 220px; height: 335px; float: left; margin: 0 15px 10px 0;">`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeWithFormat(tt.in, "img", false)
			if !strings.Contains(got, "style=") {
				t.Fatalf("expected style to survive, got %q", got)
			}
			if strings.Contains(got, "margin") == false {
				t.Fatalf("expected margin to survive, got %q", got)
			}
		})
	}
}

func TestSanitizeWithFormatStripsUnsafeOrForeignImageStyle(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"unrelated property", `<img src="https://example.com/pic.jpg" style="color: red;">`},
		{"malicious url in background", `<img src="https://example.com/pic.jpg" style="background: url(javascript:alert(1));">`},
		{"size mixed with unrelated property", `<img src="https://example.com/pic.jpg" style="width: 220px; color: red;">`},
		{"margin with a foreign value", `<img src="https://example.com/pic.jpg" style="margin: url(x);">`},
		{"margin mixed with unrelated property", `<img src="https://example.com/pic.jpg" style="margin: 0 15px; color: red;">`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeWithFormat(tt.in, "img", false)
			if strings.Contains(got, "style=") {
				t.Fatalf("expected the whole style attribute to be stripped, got %q", got)
			}
		})
	}
}

func TestSanitizeAdminKeepsCombinedImageSizeAndFloatStyle(t *testing.T) {
	in := `<img src="https://example.com/pic.jpg" style="width: 220px; height: 335px; float: right;">`
	got := SanitizeAdmin(in)
	if !strings.Contains(got, `style="width: 220px; height: 335px; float: right;"`) {
		t.Fatalf("expected combined width/height/float style to survive unchanged, got %q", got)
	}
}

func TestSanitizeAdminKeepsKbd(t *testing.T) {
	got := SanitizeAdmin(`Press <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>E</kbd>.`)
	if !strings.Contains(got, "<kbd>Ctrl</kbd>") {
		t.Fatalf("expected <kbd> to survive, got %q", got)
	}
}

// TestSanitizeAdminKeepsFooterSocialAttributes: SanitizeAdmin keeps
// target="_blank", aria-label and an informer widget's data-cid/data-lang.
func TestSanitizeAdminKeepsFooterSocialAttributes(t *testing.T) {
	in := `<a href="https://vk.com/x" target="_blank" aria-label="VKontakte">vk</a>` +
		`<img src="https://example.com/informer.png" data-cid="123" data-lang="ru">`
	got := SanitizeAdmin(in)
	for _, want := range []string{`target="_blank"`, `aria-label="VKontakte"`, `data-cid="123"`, `data-lang="ru"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q to survive, got %q", want, got)
		}
	}
}

func TestSanitizeAdminStripsUnsafeTargetAndDataAttributeValues(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"target other than _blank", `<a href="https://example.com" target="_self">x</a>`, `target=`},
		{"data-cid with unsafe characters", `<img src="https://example.com/i.png" data-cid="javascript:alert(1)">`, `data-cid=`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeAdmin(tt.in)
			if strings.Contains(got, tt.want) {
				t.Fatalf("expected %q to be stripped, got %q", tt.want, got)
			}
		})
	}
}

// TestSanitizeAdminKeepsBorderZeroOnImageStyle: an informer widget's
// style="width:88px; height:31px; border:0;" survives.
func TestSanitizeAdminKeepsBorderZeroOnImageStyle(t *testing.T) {
	in := `<img src="https://example.com/i.png" style="width:88px; height:31px; border:0;">`
	got := SanitizeAdmin(in)
	if !strings.Contains(got, "border:0") {
		t.Fatalf("expected border:0 to survive, got %q", got)
	}
}

func TestSanitizeWithFormatKeepsBreakMarker(t *testing.T) {
	in := `<p>Teaser</p><!--break--><p>Body</p><!-- other comment -->`
	got := SanitizeWithFormat(in, "p", false)
	if got != `<p>Teaser</p><!--break--><p>Body</p>` {
		t.Errorf("got %q", got)
	}
	if got := SanitizeWithFormat(in, "", false); strings.Contains(got, "<!--") {
		t.Errorf("plain text format must strip the marker too: %q", got)
	}
	if again := SanitizeWithFormat(got, "p", false); again != got {
		t.Errorf("not idempotent: %q → %q", got, again)
	}
}

func TestSanitizeWithFormatIframe(t *testing.T) {
	embed := `<iframe width="560" height="315" src="https://www.youtube.com/embed/abc123" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe>`

	got := SanitizeWithFormat(embed, "p iframe", false)
	for _, want := range []string{`src="https://www.youtube.com/embed/abc123"`, `width="560"`, `height="315"`, `allowfullscreen`, `allow="accelerometer; autoplay;`, `referrerpolicy="strict-origin-when-cross-origin"`, `title="YouTube video player"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}

	if got := SanitizeWithFormat(embed, "p", false); strings.Contains(got, "iframe") {
		t.Errorf("iframe must be stripped when the format doesn't list it: %q", got)
	}

	for _, bad := range []string{
		`<iframe src="http://example.com/x"></iframe>`,
		`<iframe src="javascript:alert(1)"></iframe>`,
		`<iframe src="data:text/html,<script>alert(1)</script>"></iframe>`,
	} {
		if got := SanitizeWithFormat(bad, "iframe", false); strings.Contains(got, "src=") {
			t.Errorf("unsafe src kept: %q → %q", bad, got)
		}
	}
	if got := SanitizeWithFormat(`<iframe src="https://example.com/x" onload="alert(1)" srcdoc="<b>x</b>"></iframe>`, "iframe", false); strings.Contains(got, "onload") || strings.Contains(got, "srcdoc") {
		t.Errorf("scriptable attributes kept: %q", got)
	}
}

// TestSanitizeAdminRowTemplate - the {{url}} token survives in href and
// text, field tokens stay, internal links get no nofollow (external
// ones do), scripts go.
func TestSanitizeAdminRowTemplate(t *testing.T) {
	in := `<div class="card"><a href="{{url}}">{{field:cover_image}}</a><h3><a href="{{url}}">{{field:title}}</a></h3><a href="https://example.com/x">ext</a><script>alert(1)</script></div>`
	out := SanitizeAdminRowTemplate(in)
	for _, want := range []string{`<a href="{{url}}">{{field:cover_image}}</a>`, `<a href="{{url}}">{{field:title}}</a>`, `class="card"`, `href="https://example.com/x" rel="nofollow"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "script") || strings.Contains(out, "cmsrowurl") {
		t.Fatalf("unexpected output: %s", out)
	}
}

// TestStripToText - blocks keep their texts apart, inline tags don't,
// entities are decoded, white space collapses.
func TestStripToText(t *testing.T) {
	for in, want := range map[string]string{
		"<p>One.</p><p>Two.</p>":                        "One. Two.",
		"<h2>Title</h2><ul><li>a</li><li>b</li></ul>":   "Title a b",
		"line<br>next<BR/>last":                         "line next last",
		"<p>A <strong>bold</strong>word &amp; more</p>": "A boldword & more",
		"<pre><code>go test</code></pre>Done":           "go test Done",
		"  spaced\n\ttext  ":                            "spaced text",
	} {
		if got := StripToText(in); got != want {
			t.Errorf("StripToText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripToLines(t *testing.T) {
	for in, want := range map[string][]string{
		"<p>One.</p><p>Two  <em>more</em>.</p>":       {"One.", "Two more."},
		"<h2>Title</h2><ul><li>a</li><li>b</li></ul>": {"Title", "a", "b"},
		"line<br>next":       {"line", "next"},
		"plain &amp; simple": {"plain & simple"},
		"<p> </p>":           nil,
	} {
		if got := StripToLines(in); !slices.Equal(got, want) {
			t.Errorf("StripToLines(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSanitizeEditor - the legacy editor's HTML: scripts, handlers and
// http embeds go; the admin allowlist (classes, image styles), https
// embeds and the "read more" cut stay.
func TestSanitizeEditor(t *testing.T) {
	in := `<p class="lead">Hi<script>alert(1)</script></p><img src="/a.jpg" onerror="alert(1)" style="float: left;">` +
		BreakMarker + `<iframe src="https://video.example.com/embed/1" width="560" height="315" allowfullscreen></iframe>` +
		`<iframe src="http://video.example.com/embed/2"></iframe><a href="javascript:alert(1)">x</a>`
	got := SanitizeEditor(in)
	for _, bad := range []string{"<script", "onerror", "http://video", "javascript:"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q kept in %s", bad, got)
		}
	}
	for _, want := range []string{`<p class="lead">Hi</p>`, `style="float: left;"`, BreakMarker, `<iframe src="https://video.example.com/embed/1" width="560" height="315" allowfullscreen`} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in %s", want, got)
		}
	}
}

// TestWithoutCode - the code blocks out of a text (any case, with
// attributes, several), the text around them kept; a text without code or
// all code stays as it is.
func TestWithoutCode(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`<p>Run:</p><pre><code class="language-go">x := 1</code></pre><p>done</p>`, `<p>Run:</p> <p>done</p>`},
		{"<p>a</p><PRE data-lang=\"sql\">SELECT 1\n</PRE><p>b</p><pre>c</pre>", "<p>a</p> <p>b</p> "},
		{`<p>No code</p>`, `<p>No code</p>`},
		{`<pre>only code</pre>`, `<pre>only code</pre>`},
		{`<p>a preformatted <pretty> word</p>`, `<p>a preformatted <pretty> word</p>`},
	} {
		if got := WithoutCode(tc.in); got != tc.want {
			t.Errorf("WithoutCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
