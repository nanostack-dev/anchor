package itdsl

import (
	"context"

	itshared "anchor/cmd/it/shared"
	dslfactory "anchor/cmd/it/shared/dsl/factory"
	platformdomain "anchor/internal/domain/platform"
	"anchor/internal/domain/tenant"

	"github.com/stretchr/testify/require"
)

// TenantOpts configures tenant alias registration in the fluent builder.
type TenantOpts struct {
	Alias    string
	Isolated bool
	Name     string
}

// Tenant registers the shared tenant context under the provided alias.
func (b *Builder) Tenant(opts TenantOpts) *Builder {
	b.t.Helper()

	require.NotEmpty(b.t, opts.Alias, "tenant alias is required")
	_, exists := b.tenants[opts.Alias]
	require.False(b.t, exists, "tenant alias '%s' already exists", opts.Alias)

	require.NotNil(b.t, itshared.TenantRepository, "tenant repository is not available in test setup")

	tenantID := findDefaultTenantID(b.t)
	if opts.Isolated {
		createdTenant := createTenant(b.t, opts.Name)
		tenantID = createdTenant.ID
	}
	ownerUserResult := dslfactory.CreatePlatformUserWithRole(
		b.t,
		tenantID,
		platformdomain.TenantRoleOwner,
	)
	ownerUser := &PlatformUser{
		ID:                  ownerUserResult.ID,
		UserID:              ownerUserResult.UserID,
		Email:               ownerUserResult.Email,
		Password:            ownerUserResult.Password,
		AccessToken:         ownerUserResult.AccessToken,
		RefreshToken:        ownerUserResult.RefreshToken,
		AuthenticatedClient: ownerUserResult.AuthenticatedClient,
		TenantAlias:         opts.Alias,
	}

	b.tenants[opts.Alias] = &Tenant{
		ID:           tenantID,
		OwnerUser:    ownerUser,
		OwnerClient:  ownerUser.AuthenticatedClient,
		NoAuthClient: dslfactory.NewNoAuthClient(b.t, itshared.ServerURL),
		ServerURL:    itshared.ServerURL,
	}
	return b
}

func findDefaultTenantID(t require.TestingT) string {
	tenants, err := itshared.TenantRepository.FindAll(context.Background(), nil)
	require.NoError(t, err)
	if len(tenants) == 0 {
		createdTenant := createTenant(t, "")
		return createdTenant.ID
	}

	return tenants[0].ID
}

func createTenant(t require.TestingT, name string) tenant.PlatformTenant {
	tenantName := name
	if tenantName == "" {
		tenantName = "tenant-" + itshared.Faker.Company().Name()
	}

	newTenant := tenant.PlatformTenant{
		Name:   tenantName,
		Status: tenant.Active,
	}
	newTenant.GenerateID()

	createdTenant, err := itshared.TenantRepository.Create(context.Background(), newTenant, nil)
	require.NoError(t, err)

	return createdTenant
}
