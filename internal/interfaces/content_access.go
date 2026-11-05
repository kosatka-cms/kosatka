package interfaces

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// guestVisitor - what of the content a guest may not view
// (SetGuestVisitor); nil - nothing.
var guestVisitor func(ctx context.Context) (entity.ContentVisitor, error)

// SetGuestVisitor sets what of the content a guest may not view (wiring:
// service.ContentAccess.Visitor for nil).
func SetGuestVisitor(f func(ctx context.Context) (entity.ContentVisitor, error)) { guestVisitor = f }

// AsGuest - ctx whose node and comment queries see only what a guest may
// view: for what is public by nature and kept by any cache, whoever asks
// (feeds, the sitemap).
func AsGuest(ctx context.Context) (context.Context, error) {
	if guestVisitor == nil {
		return entity.WithContentVisitor(ctx, entity.ContentVisitor{}), nil
	}
	v, err := guestVisitor(ctx)
	if err != nil {
		return nil, fmt.Errorf("the content closed to guests: %w", err)
	}
	return entity.WithContentVisitor(ctx, v), nil
}

// AsGuestHandler serves next as for a guest (AsGuest); a failed check is
// a 500.
func AsGuestHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := AsGuest(r.Context())
		if err != nil {
			logFailure(slog.Default(), err, "serving as for a guest")
			httputil.InternalServerError(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
