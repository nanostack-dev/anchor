package api

import (
	"context"
	"net/http"
	"strings"
)

type logoutAccessTokenKey struct{}

// logoutAccessTokenFromContext returns the bearer token the logout request
// carried, or "" when it carried none.
func logoutAccessTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(logoutAccessTokenKey{}).(string)
	return token
}

// NewLogoutAccessTokenMiddleware returns a StrictMiddlewareFunc that, for the
// Logout operation, copies the bearer token into the request context. Logout
// is public, so the auth middleware does not verify the token: logout must
// still end the session after the access token expired.
func NewLogoutAccessTokenMiddleware() StrictMiddlewareFunc {
	return func(f StrictHandlerFunc, operationID string) StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			if operationID == "Logout" {
				token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
				if found {
					ctx = context.WithValue(ctx, logoutAccessTokenKey{}, token)
				}
			}
			return f(ctx, w, r, request)
		}
	}
}
