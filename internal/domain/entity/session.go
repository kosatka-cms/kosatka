package entity

import "github.com/kosatka-cms/kosatka/pkg/csrf"

// SessionName - the site's session cookie: who is signed in, the 2FA
// step, the theme preview, the CSRF token (pkg/csrf keeps it there).
const SessionName = csrf.SessionName
