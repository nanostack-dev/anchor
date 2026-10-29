package api

import (
	"context"
	"net/http"

	"anchor/internal/authz/mirror"
)

// AuthzReplayer syncs what a request marked, once its transaction committed.
type AuthzReplayer interface {
	Replay(ctx context.Context, marked []mirror.Scope)
}

// NewAuthzReplayMiddleware returns a StrictMiddlewareFunc that collects what a
// request's writes marked and, once the handler has returned and its
// transaction committed, syncs it into the relationship store before the
// response is written, so the next request already sees the change (ADR-0022
// E5).
func NewAuthzReplayMiddleware(replayer AuthzReplayer) StrictMiddlewareFunc {
	return func(f StrictHandlerFunc, _ string) StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			collectCtx, marked := mirror.CollectMarks(ctx)
			response, err := f(collectCtx, w, r, request)
			if err == nil {
				replayer.Replay(ctx, marked())
			}
			return response, err
		}
	}
}
