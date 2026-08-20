package itdsl

import (
	"context"

	"github.com/stretchr/testify/require"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"
)

// ProductUserOpts configures product user creation linked to a product alias.
type ProductUserOpts struct {
	Alias        string
	ProductAlias string
	Email        string
	Name         string
}

// ProductRoleOpts configures product role creation linked to a product alias.
type ProductRoleOpts struct {
	Alias        string
	ProductAlias string
	Name         string
	Description  *string
	Permissions  []string
}

// ProductOrganizationOpts configures organization creation linked to a product alias.
type ProductOrganizationOpts struct {
	Alias        string
	ProductAlias string
	Name         string
	Description  *string
}

// MembershipOpts configures organization membership creation by alias links.
type MembershipOpts struct {
	Alias             string
	ProductAlias      string
	ProductUserAlias  string
	OrganizationAlias string
	RoleAlias         string
}

// ExistingProductOpts registers an existing product context under an alias.
type ExistingProductOpts struct {
	Alias   string
	Context *ProductContext
}

// ExistingProductUserOpts registers an existing product user under an alias.
type ExistingProductUserOpts struct {
	Alias        string
	ProductAlias string
	ID           string
	Email        string
	Name         string
	Status       string
}

// ExistingProductRoleOpts registers an existing product role under an alias.
type ExistingProductRoleOpts struct {
	Alias        string
	ProductAlias string
	ID           string
	Name         string
}

// ExistingProductOrganizationOpts registers an existing organization under an alias.
type ExistingProductOrganizationOpts struct {
	Alias        string
	ProductAlias string
	ID           string
	Name         string
}

// ProductUserExternalIDOpts updates external id for a product user alias.
type ProductUserExternalIDOpts struct {
	ProductUserAlias string
	ExternalID       string
}

// ExistingProduct registers an existing product context under an alias.
func (b *Builder) ExistingProduct(opts ExistingProductOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "product alias is required")
	require.NotNil(b.t, opts.Context, "product context is required")
	_, exists := b.products[opts.Alias]
	require.False(b.t, exists, "product alias '%s' already exists", opts.Alias)
	b.products[opts.Alias] = opts.Context
	return b
}

// ExistingProductUser registers an existing product user under an alias.
func (b *Builder) ExistingProductUser(opts ExistingProductUserOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "product user alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	require.NotEmpty(b.t, opts.ID, "product user id is required")
	_, exists := b.productUsers[opts.Alias]
	require.False(b.t, exists, "product user alias '%s' already exists", opts.Alias)
	_, productExists := b.products[opts.ProductAlias]
	require.True(b.t, productExists, "unknown product alias '%s'", opts.ProductAlias)
	b.productUsers[opts.Alias] = &ProductUser{
		ID:           opts.ID,
		ProductAlias: opts.ProductAlias,
		Email:        opts.Email,
		Name:         opts.Name,
		Status:       opts.Status,
	}
	return b
}

// ExistingProductRole registers an existing product role under an alias.
func (b *Builder) ExistingProductRole(opts ExistingProductRoleOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "product role alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	require.NotEmpty(b.t, opts.ID, "product role id is required")
	_, exists := b.productRoles[opts.Alias]
	require.False(b.t, exists, "product role alias '%s' already exists", opts.Alias)
	_, productExists := b.products[opts.ProductAlias]
	require.True(b.t, productExists, "unknown product alias '%s'", opts.ProductAlias)
	b.productRoles[opts.Alias] = &ProductRole{ID: opts.ID, ProductAlias: opts.ProductAlias, Name: opts.Name}
	return b
}

// ExistingProductOrganization registers an existing organization under an alias.
func (b *Builder) ExistingProductOrganization(opts ExistingProductOrganizationOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "organization alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	require.NotEmpty(b.t, opts.ID, "organization id is required")
	_, exists := b.organizations[opts.Alias]
	require.False(b.t, exists, "organization alias '%s' already exists", opts.Alias)
	_, productExists := b.products[opts.ProductAlias]
	require.True(b.t, productExists, "unknown product alias '%s'", opts.ProductAlias)
	b.organizations[opts.Alias] = &ProductOrganization{ID: opts.ID, ProductAlias: opts.ProductAlias, Name: opts.Name}
	return b
}

// ProductUserExternalID updates external id by product user alias.
func (b *Builder) ProductUserExternalID(opts ProductUserExternalIDOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.ProductUserAlias, "product user alias is required")
	require.NotEmpty(b.t, opts.ExternalID, "external id is required")

	productUser, userExists := b.productUsers[opts.ProductUserAlias]
	require.True(b.t, userExists, "unknown product user alias '%s'", opts.ProductUserAlias)

	productCtx, productExists := b.products[productUser.ProductAlias]
	require.True(b.t, productExists, "unknown product alias '%s'", productUser.ProductAlias)

	require.NotNil(
		b.t,
		itshared.ProductUserRepository,
		"product user repository is not available in test setup",
	)

	foundUserOpt := itshared.ProductUserRepository.FindByProductIDAndID(
		context.Background(),
		productCtx.ProductID,
		productUser.ID,
	)
	require.NoError(b.t, foundUserOpt.Err(), "failed to load product user")
	require.True(b.t, foundUserOpt.IsPresent(), "product user not found")
	foundUser := foundUserOpt.ToPtr()

	foundUser.ExternalID = &opts.ExternalID
	_, err := itshared.ProductUserRepository.Update(
		context.Background(),
		productCtx.ProductID,
		productUser.ID,
		*foundUser,
	)
	require.NoError(b.t, err, "failed to update product user external_id")
	return b
}

// ProductUser creates a product user under a product alias.
func (b *Builder) ProductUser(opts ProductUserOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "product user alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	_, exists := b.productUsers[opts.Alias]
	require.False(b.t, exists, "product user alias '%s' already exists", opts.Alias)

	productCtx, ok := b.products[opts.ProductAlias]
	require.True(b.t, ok, "unknown product alias '%s'", opts.ProductAlias)

	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:create"})
	name := opts.Name
	if name == "" {
		name = "User " + itshared.Faker.Person().FirstName()
	}
	email := opts.Email
	if email == "" {
		email = itshared.Faker.Internet().Email()
	}
	status := nanostackClient.ProductUserStatus("ACTIVE")
	resp, err := apiKeyClient.CreateProductUserWithResponse(
		context.Background(),
		productCtx.ProductID,
		nanostackClient.CreateProductUserJSONRequestBody{Email: email, Name: &name, Status: &status},
	)
	require.NoError(b.t, err)
	require.NotNil(b.t, resp.JSON201)
	resolvedName := ""
	if resp.JSON201.Name != nil {
		resolvedName = *resp.JSON201.Name
	}

	b.productUsers[opts.Alias] = &ProductUser{
		ID:           resp.JSON201.Id,
		ProductAlias: opts.ProductAlias,
		Email:        resp.JSON201.Email,
		Name:         resolvedName,
		Status:       string(resp.JSON201.Status),
	}
	return b
}

// ProductRole creates a product role under a product alias.
func (b *Builder) ProductRole(opts ProductRoleOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "product role alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	_, exists := b.productRoles[opts.Alias]
	require.False(b.t, exists, "product role alias '%s' already exists", opts.Alias)

	productCtx, ok := b.products[opts.ProductAlias]
	require.True(b.t, ok, "unknown product alias '%s'", opts.ProductAlias)

	roleName := opts.Name
	if roleName == "" {
		roleName = "role-" + itshared.Faker.UUID().V4()
	}
	resp, err := productCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		context.Background(),
		productCtx.ProductID,
		nanostackClient.CreateProductRoleJSONRequestBody{
			Name:        roleName,
			Description: opts.Description,
			Permissions: opts.Permissions,
		},
	)
	require.NoError(b.t, err)
	require.NotNil(b.t, resp.JSON201)
	b.productRoles[opts.Alias] = &ProductRole{
		ID:           resp.JSON201.Id,
		ProductAlias: opts.ProductAlias,
		Name:         resp.JSON201.Name,
	}
	return b
}

// ProductOrganization creates an organization under a product alias.
func (b *Builder) ProductOrganization(opts ProductOrganizationOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "organization alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	_, exists := b.organizations[opts.Alias]
	require.False(b.t, exists, "organization alias '%s' already exists", opts.Alias)

	productCtx, ok := b.products[opts.ProductAlias]
	require.True(b.t, ok, "unknown product alias '%s'", opts.ProductAlias)

	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"organization:create"})
	orgName := opts.Name
	if orgName == "" {
		orgName = "org-" + itshared.Faker.UUID().V4()
	}
	resp, err := apiKeyClient.CreateProductOrganizationWithResponse(
		context.Background(),
		productCtx.ProductID,
		nanostackClient.CreateProductOrganizationJSONRequestBody{Name: orgName, Description: opts.Description},
	)
	require.NoError(b.t, err)
	require.NotNil(b.t, resp.JSON201)
	b.organizations[opts.Alias] = &ProductOrganization{
		ID:           resp.JSON201.Id,
		ProductAlias: opts.ProductAlias,
		Name:         resp.JSON201.Name,
	}
	return b
}

// Membership creates an organization membership by linking aliased entities.
func (b *Builder) Membership(opts MembershipOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "membership alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	require.NotEmpty(b.t, opts.ProductUserAlias, "product user alias is required")
	require.NotEmpty(b.t, opts.OrganizationAlias, "organization alias is required")
	require.NotEmpty(b.t, opts.RoleAlias, "role alias is required")
	_, exists := b.memberships[opts.Alias]
	require.False(b.t, exists, "membership alias '%s' already exists", opts.Alias)

	productCtx, ok := b.products[opts.ProductAlias]
	require.True(b.t, ok, "unknown product alias '%s'", opts.ProductAlias)

	productUser, userExists := b.productUsers[opts.ProductUserAlias]
	require.True(b.t, userExists, "unknown product user alias '%s'", opts.ProductUserAlias)
	organization, orgExists := b.organizations[opts.OrganizationAlias]
	require.True(b.t, orgExists, "unknown organization alias '%s'", opts.OrganizationAlias)
	role, roleExists := b.productRoles[opts.RoleAlias]
	require.True(b.t, roleExists, "unknown role alias '%s'", opts.RoleAlias)

	require.Equal(
		b.t,
		opts.ProductAlias,
		productUser.ProductAlias,
		"product user alias '%s' belongs to different product",
		opts.ProductUserAlias,
	)
	require.Equal(
		b.t,
		opts.ProductAlias,
		organization.ProductAlias,
		"organization alias '%s' belongs to different product",
		opts.OrganizationAlias,
	)
	require.Equal(
		b.t,
		opts.ProductAlias,
		role.ProductAlias,
		"role alias '%s' belongs to different product",
		opts.RoleAlias,
	)

	require.NotNil(
		b.t,
		itshared.OrgMembershipRepository,
		"organization membership repository is not available in test setup",
	)
	_, err := itshared.OrgMembershipRepository.Create(
		context.Background(),
		productCtx.ProductID,
		organization.ID,
		productUser.ID,
		role.ID,
	)
	require.NoError(b.t, err)

	b.memberships[opts.Alias] = &OrganizationMembership{
		ProductAlias:      opts.ProductAlias,
		ProductUserAlias:  opts.ProductUserAlias,
		OrganizationAlias: opts.OrganizationAlias,
		RoleAlias:         opts.RoleAlias,
	}
	return b
}
