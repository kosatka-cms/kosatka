package htmlimages

import "testing"

// TestTextCache - past its budget the least recently used text goes; one
// larger than the budget isn't kept; clear empties it; a nil one keeps
// nothing.
func TestTextCache(t *testing.T) {
	c := newTextCache(10)
	a, b, d := [32]byte{1}, [32]byte{2}, [32]byte{3}
	c.put(a, "aaaa")
	c.put(b, "bbbb")
	if _, ok := c.get(a); !ok { // a is now the most recent
		t.Fatal("a")
	}
	c.put(d, "dddd") // 12 bytes: b, the least recent, goes
	if _, ok := c.get(b); ok {
		t.Error("b kept past the budget")
	}
	if out, ok := c.get(a); !ok || out != "aaaa" {
		t.Errorf("a: %q %v", out, ok)
	}
	c.put([32]byte{4}, "eleven byte")
	if _, ok := c.get([32]byte{4}); ok {
		t.Error("a text larger than the budget was kept")
	}
	c.clear()
	if _, ok := c.get(a); ok || c.size != 0 {
		t.Errorf("after clear: size %d", c.size)
	}
	var none *textCache
	none.put(a, "x")
	if _, ok := none.get(a); ok {
		t.Error("nil cache")
	}
	none.clear()
}
