package dao

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// TestUserAvatar - an uploaded avatar: its style's address for pages and
// its stored URI; none - neither (the pages show the placeholder).
func TestUserAvatar(t *testing.T) {
	with := ToUserEntity(&generated.User{ID: 7, Avatar: "public://user_pictures/7-1.jpg"})
	if with.AvatarStored != "public://user_pictures/7-1.jpg" || !strings.Contains(with.Avatar.URI, "styles/user_avatar_list/") {
		t.Errorf("an avatar: %+v %q", with.Avatar, with.AvatarStored)
	}
	if none := ToUserEntity(&generated.User{ID: 8}); none.Avatar.URI != "" || none.AvatarStored != "" {
		t.Errorf("no avatar: %+v %q", none.Avatar, none.AvatarStored)
	}
}
