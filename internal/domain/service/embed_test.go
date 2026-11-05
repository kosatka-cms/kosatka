package service

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/embedtag"
)

// registerTestEmbeds - the kinds of the embed tests: echo (the name and
// the visitor's closed types), nest (embeds the kind and name its name
// says, "echo-x" - [[echo:x]]), self (embeds itself), gone (missing).
func registerTestEmbeds() {
	RegisterEmbedKind(EmbedKind{Name: "echo", Render: func(ctx context.Context, name string) (template.HTML, error) {
		return template.HTML("<b>" + name + "|" + strings.Join(entity.HiddenNodeTypes(ctx), ",") + "</b>"), nil
	}})
	RegisterEmbedKind(EmbedKind{Name: "nest", Render: func(_ context.Context, name string) (template.HTML, error) {
		kind, inner, _ := strings.Cut(name, "-")
		return template.HTML("(" + markEmbeds(context.Background(), "[["+kind+":"+inner+"]]") + ")"), nil
	}})
	RegisterEmbedKind(EmbedKind{Name: "self", Render: func(_ context.Context, name string) (template.HTML, error) {
		return template.HTML("(" + markEmbeds(context.Background(), "[[self:"+name+"]]") + ")"), nil
	}})
	RegisterEmbedKind(EmbedKind{Name: "gone", Render: func(context.Context, string) (template.HTML, error) {
		return "", entity.ErrEntityNotFound
	}})
}

// TestMarkEmbeds - the filter marks the tags of known kinds in the text, a
// paragraph of its own whole; a tag in an attribute or of an unknown kind
// stays.
func TestMarkEmbeds(t *testing.T) {
	registerTestEmbeds()
	ctx := context.Background()
	in := `<p>[[echo:a]]</p><p>See [[echo:b]] and [[other:c]].</p><a href="/x?[[echo:d]]">[[echo:e]]</a>`
	out := markEmbeds(ctx, in)
	for _, want := range []string{embedMarker(tagOf("echo", "a")) + "<p>See " + embedMarker(tagOf("echo", "b")) + " and [[other:c]].</p>", `href="/x?[[echo:d]]"`, ">" + embedMarker(tagOf("echo", "e")) + "</a>"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
	if got := markEmbeds(ctx, "<p>plain</p>"); got != "<p>plain</p>" {
		t.Errorf("a text without tags: %q", got)
	}
}

// TestResolveEmbeds - the marks are filled in with what their kinds
// render in the request's context; a missing thing, a self-embedding
// and one too deep leave nothing; the template function marks too.
func TestResolveEmbeds(t *testing.T) {
	registerTestEmbeds()
	ctx := entity.WithContentVisitor(context.Background(), entity.ContentVisitor{HiddenTypes: []string{"members"}})
	page := []byte("<main>" + markEmbeds(ctx, "<p>[[echo:a]]</p>[[gone:b]]") + string(EmbedHTML("echo:c")) + "</main>")
	if got := string(ResolveEmbeds(ctx, page)); got != "<main><b>a|members</b><b>c|members</b></main>" {
		t.Errorf("resolved: %s", got)
	}
	nested := []byte(markEmbeds(ctx, "[[nest:echo-x]]"))
	if got := string(ResolveEmbeds(ctx, nested)); got != "(<b>x|members</b>)" {
		t.Errorf("nested: %s", got)
	}
	self := []byte(markEmbeds(ctx, "[[self:x]]"))
	if got := string(ResolveEmbeds(ctx, self)); got != "()" {
		t.Errorf("self-embedding: %s", got)
	}
	if EmbedHTML("bad tag") != "" || EmbedHTML("unknown:x") != "" {
		t.Error("an invalid tag or an unknown kind marks")
	}
	forged := []byte("<!--kosatka-embed:0000000000000000:echo:a-->")
	if got := string(ResolveEmbeds(ctx, forged)); got != string(forged) {
		t.Errorf("a mark without the process's nonce: %s", got)
	}
}

// TestEmbedRenderError - a failing kind leaves nothing.
func TestEmbedRenderError(t *testing.T) {
	RegisterEmbedKind(EmbedKind{Name: "broken", Render: func(context.Context, string) (template.HTML, error) {
		return "", errors.New("db down")
	}})
	if got := ResolveEmbeds(context.Background(), []byte(markEmbeds(context.Background(), "[[broken:x]]"))); len(got) != 0 {
		t.Errorf("a failure: %s", got)
	}
}

func tagOf(kind, name string) embedtag.Tag { return embedtag.Tag{Kind: kind, Name: name} }

// TestEmbedDepth - embeds embed up to maxEmbedDepth deep.
func TestEmbedDepth(t *testing.T) {
	RegisterEmbedKind(EmbedKind{Name: "deep", Render: func(_ context.Context, name string) (template.HTML, error) {
		n, _ := strconv.Atoi(name)
		return template.HTML(name + "(" + markEmbeds(context.Background(), fmt.Sprintf("[[deep:%d]]", n+1)) + ")"), nil
	}})
	page := []byte(markEmbeds(context.Background(), "[[deep:1]]"))
	if got := string(ResolveEmbeds(context.Background(), page)); got != "1(2(3()))" {
		t.Errorf("deep: %s", got)
	}
}
