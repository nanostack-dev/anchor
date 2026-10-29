package api

import (
	"context"
	"net/http"

	authzservice "anchor/internal/authz/service"
)

// NewAuthzReplayMiddleware returns a StrictMiddlewareFunc that collects the
// Organizations a request's writes marked and, once the handler has returned
// and its transaction committed, replays them into the relationship store
// before the response is written, so the next request already sees the change
// (ADR-0022 E5).
func NewAuthzReplayMiddleware(replayer *authzservice.Replayer) StrictMiddlewareFunc {
	return func(f StrictHandlerFunc, _ string) StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			collectCtx, marked := authzservice.CollectMarks(ctx)
			response, err := f(collectCtx, w, r, request)
			if err == nil {
				replayer.Replay(ctx, marked())
			}
			return response, err
		}
	}
}
