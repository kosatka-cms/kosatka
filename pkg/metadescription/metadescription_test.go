package metadescription

import "testing"

func TestGenerate_stripsTagsAndEntities(t *testing.T) {
	got := Generate(`<p>Hello&nbsp;&amp;&nbsp;welcome</p><script>alert(1)</script>`, 0)
	want := "Hello & welcome"
	if got != want {
		t.Fatalf("Generate() = %q, want %q", got, want)
	}
}

func TestGenerate_truncatesAtWordBoundary(t *testing.T) {
	content := "one two three four five six seven eight nine ten"
	got := Generate(content, 20)
	if len(got) == 0 || got[len(got)-3:] != "..." {
		t.Fatalf("expected truncated content ending in ..., got %q", got)
	}
}
