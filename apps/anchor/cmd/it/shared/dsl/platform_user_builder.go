package itdsl

import (
	dslfactory "anchor/cmd/it/shared/dsl/factory"
	platformdomain "anchor/internal/domain/platform"

	"github.com/stretchr/testify/require"
)

// PlatformAdminOpts configures platform admin user creation in the fluent builder.
type PlatformAdminOpts struct {
	Alias       string
	TenantAlias string
}

// PlatformAdmin creates a random admin user under a tenant alias.
func (b *Builder) PlatformAdmin(opts PlatformAdminOpts) *Builder {
	b.t.Helper()

	require.NotEmpty(b.t, opts.Alias, "platform user alias is required")
	require.NotEmpty(b.t, opts.TenantAlias, "tenant alias is required")

	_, userAliasExists := b.users[opts.Alias]
	require.False(b.t, userAliasExists, "platform user alias '%s' already exists", opts.Alias)

	tenantCtx, tenantExists := b.tenants[opts.TenantAlias]
	require.True(b.t, tenantExists, "unknown tenant alias '%s'", opts.TenantAlias)
	userResult := dslfactory.CreatePlatformUserWithRole(
		b.t,
		tenantCtx.ID,
		platformdomain.TenantRoleAdmin,
	)
	user := &PlatformUser{
		ID:                  userResult.ID,
		UserID:              userResult.UserID,
		Email:               userResult.Email,
		Password:            userResult.Password,
		AccessToken:         userResult.AccessToken,
		RefreshToken:        userResult.RefreshToken,
		AuthenticatedClient: userResult.AuthenticatedClient,
		TenantAlias:         opts.TenantAlias,
	}

	b.users[opts.Alias] = user

	return b
}
