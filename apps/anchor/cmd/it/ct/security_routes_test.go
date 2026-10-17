package ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouteSecurityMatrix(t *testing.T) {
	// Every protected route refuses unauthenticated or underprivileged callers and accepts the required permissions.
	t.Parallel()
	routes, components := loadSecurityRouteCases(t)
	require.NotEmpty(t, routes)
	fx := newSecurityFixture(t)

	for _, route := range routes {
		t.Run(route.OperationID, func(t *testing.T) {
			// The route enforces the authentication and permission policy declared for this operation.
			t.Run("no_auth", func(t *testing.T) {
				// The protected route refuses a caller without credentials.
				resp := securitySendRequest(t, route, components, nil, fx)
				defer resp.Body.Close()

				if route.IsPublic {
					require.NotEqual(t, http.StatusForbidden, resp.StatusCode)
					return
				}

				require.Equal(
					t,
					http.StatusUnauthorized,
					resp.StatusCode,
				)
				require.Equal(t, "UNAUTHORIZED", requireJSONErrorCode(t, resp))
			})

			t.Run("valid_auth_with_required_permissions", func(t *testing.T) {
				// The protected route accepts an authenticated caller with its required permissions.
				headers := validHeadersForRoute(t, route, fx)
				resp := securitySendRequest(t, route, components, headers, fx)
				defer resp.Body.Close()

				if route.IsPublic {
					require.NotEqual(t, http.StatusForbidden, resp.StatusCode)
					return
				}

				assertNoAuthError(t, resp, route, "valid_auth_with_required_permissions")
			})

			t.Run("auth_without_required_permissions", func(t *testing.T) {
				// The protected route refuses an authenticated caller missing its required permissions.
				if route.IsPublic {
					t.Skip("public route has no required auth scope")
				}

				headers := missingScopeHeadersForRoute(t, route, fx)
				resp := securitySendRequest(t, route, components, headers, fx)
				defer resp.Body.Close()

				assertAuthRejected(t, resp, route, "auth_without_required_permissions")
			})
		})
	}
}

func TestSecurityCoverageAllRoutesExplicitAndCovered(t *testing.T) {
	// Every API route has an explicit security policy and a matching route-security test.
	t.Parallel()
	routes, _ := loadSecurityRouteCases(t)
	require.NotEmpty(t, routes)

	seenOperationIDs := map[string]struct{}{}
	for _, route := range routes {
		require.Truef(
			t,
			route.HasExplicitSecurity,
			"operation %s (%s %s) must explicitly declare security in OpenAPI",
			route.OperationID,
			route.Method,
			route.Path,
		)
		if _, exists := seenOperationIDs[route.OperationID]; exists {
			require.Failf(
				t,
				"duplicate operationId",
				"operationId %s appears more than once",
				route.OperationID,
			)
		}
		seenOperationIDs[route.OperationID] = struct{}{}
	}

	require.Len(t, seenOperationIDs, len(routes))
}
