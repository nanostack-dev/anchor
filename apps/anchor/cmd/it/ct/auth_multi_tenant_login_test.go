package ct_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jwtTenantID(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "expected 3-part JWT")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		TenantID string `json:"tenant_id"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims.TenantID
}

func TestMultiTenantRegisterAndLoginResolvesUsersOwnTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Ensure the default (first) tenant exists before adding a second tenant.
	firstTenant := testTenant(t)

	t.Run("User registered in a second tenant authenticates to that tenant on register and login", func(t *testing.T) {
		t.Parallel()

		secondTenantID := ids.MustNew("tenant")
		invitationID := ids.MustNew("pinv")
		invitationCode := "code-" + ids.MustNew("inv")
		email := "second-tenant-" + ids.MustNew("usr") + "@example.test"
		password := "SecondTenantPass123!"

		execSQL(t,
			`INSERT INTO platform_tenants (id, name, status) VALUES ($1, $2, 'ACTIVE')`,
			secondTenantID, "Second Tenant "+secondTenantID,
		)
		execSQL(t,
			`INSERT INTO platform_invitations (id, platform_tenant_id, email, code)
			 VALUES ($1, $2, $3, $4)`,
			invitationID, secondTenantID, email, invitationCode,
		)

		regResp, err := firstTenant.NoAuthClient.RegisterWithResponse(ctx, ct.RegisterJSONRequestBody{
			Email:          email,
			Password:       password,
			InvitationCode: &invitationCode,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, regResp.StatusCode(), string(regResp.Body))
		require.NotNil(t, regResp.JSON200)
		assert.Equal(t, secondTenantID, jwtTenantID(t, regResp.JSON200.AccessToken),
			"register must issue a token for the invited tenant, not tenants[0]")

		loginResp, err := firstTenant.NoAuthClient.LoginWithResponse(ctx, ct.LoginJSONRequestBody{
			Email:    email,
			Password: password,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, loginResp.StatusCode(), string(loginResp.Body))
		require.NotNil(t, loginResp.JSON200)
		assert.Equal(t, secondTenantID, jwtTenantID(t, loginResp.JSON200.AccessToken),
			"login must resolve the user's own tenant by user_id, not tenants[0]")
	})

	t.Run("User belonging to multiple tenants is refused on password login", func(t *testing.T) {
		t.Parallel()

		secondTenantID := ids.MustNew("tenant")
		invitationID := ids.MustNew("pinv")
		invitationCode := "code-" + ids.MustNew("inv")
		email := "multi-tenant-" + ids.MustNew("usr") + "@example.test"
		password := "MultiTenantPass123!"

		execSQL(t,
			`INSERT INTO platform_tenants (id, name, status) VALUES ($1, $2, 'ACTIVE')`,
			secondTenantID, "Multi Tenant "+secondTenantID,
		)
		execSQL(t,
			`INSERT INTO platform_invitations (id, platform_tenant_id, email, code)
			 VALUES ($1, $2, $3, $4)`,
			invitationID, secondTenantID, email, invitationCode,
		)

		regResp, err := firstTenant.NoAuthClient.RegisterWithResponse(ctx, ct.RegisterJSONRequestBody{
			Email:          email,
			Password:       password,
			InvitationCode: &invitationCode,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, regResp.StatusCode(), string(regResp.Body))

		// Add a second platform_users row for the same user_id in firstTenant.
		extraPlatformUserID := ids.MustNew("pu")
		execSQL(t,
			`INSERT INTO platform_users (id, user_id, platform_tenant_id, email, hashed_password, role)
			 SELECT $1, user_id, $2, email, hashed_password, 'ADMIN'
			 FROM platform_users
			 WHERE email = $3 AND platform_tenant_id = $4`,
			extraPlatformUserID, firstTenant.ID, email, secondTenantID,
		)

		loginResp, err := firstTenant.NoAuthClient.LoginWithResponse(ctx, ct.LoginJSONRequestBody{
			Email:    email,
			Password: password,
		})
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, loginResp.StatusCode(), string(loginResp.Body))
		errResp := decodeAPIError(t, loginResp.Body)
		assert.Equal(t, "EMAIL_OR_PASSWORD_INCORRECT", errResp.Errors[0].Code)
	})
}
