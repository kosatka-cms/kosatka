package privatemessages

import "testing"

// TestThreadHasMember - a member counts whether or not they archived the
// thread (Deleted is the archive); someone else doesn't.
func TestThreadHasMember(t *testing.T) {
	thread := Thread{Members: []Member{{UserID: 1}, {UserID: 2, Deleted: true}}}
	for id, want := range map[int]bool{1: true, 2: true, 999: false} {
		if got := thread.HasMember(id); got != want {
			t.Errorf("HasMember(%d) = %v, want %v", id, got, want)
		}
	}
}
