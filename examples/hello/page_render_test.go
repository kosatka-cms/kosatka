package hello

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestPageRender - the module's page renders from its embedded templates
// (testkit.UseEngineRoot serves a registered module's assets, as a site
// does): its greetings, or the empty text without them.
func TestPageRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	out := testkit.RenderPageContent(t, "hello/page.html", map[string]any{"greetings": []Greeting{{Text: "Ahoy"}, {Text: "Hi"}}})
	for _, want := range []string{`<ul class="hello-list">`, "<li>Ahoy</li>", "<li>Hi</li>", "Greetings from a module of its own repository."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if out := testkit.RenderPageContent(t, "hello/page.html", map[string]any{}); !strings.Contains(out, "No greetings yet.") {
		t.Errorf("without greetings:\n%s", out)
	}
}
