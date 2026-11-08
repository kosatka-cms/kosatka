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

// TestGenerate_leavesOutCode - a description is the text's prose: its code
// blocks left out.
func TestGenerate_leavesOutCode(t *testing.T) {
	got := Generate(`<p>Small images:</p><pre><code>FROM golang AS build</code></pre><p>build, then copy.</p>`, 0)
	if got != "Small images: build, then copy." {
		t.Errorf("Generate = %q", got)
	}
}
