package database

import "testing"

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`u42/my_dir/100%\x`); got != `u42/my\_dir/100\%\\x` {
		t.Fatalf("got %q", got)
	}
}
