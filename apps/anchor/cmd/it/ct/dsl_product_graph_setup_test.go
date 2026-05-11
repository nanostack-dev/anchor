package ct_test

import (
	"testing"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

func createDSLProductUser(t *testing.T, productCtx *itdsl.ProductContext) *itdsl.ProductUser {
	t.Helper()
	alias := "product.user." + itshared.Faker.UUID().V4()
	return itdsl.Given(t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "product.ctx", Context: productCtx}).
		ProductUser(itdsl.ProductUserOpts{Alias: alias, ProductAlias: "product.ctx"}).
		Build().
		ProductUser(alias)
}

func createDSLProductRole(
	t *testing.T,
	productCtx *itdsl.ProductContext,
	name string,
	description *string,
) *itdsl.ProductRole {
	t.Helper()
	alias := "product.role." + itshared.Faker.UUID().V4()
	return itdsl.Given(t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "product.ctx", Context: productCtx}).
		ProductRole(
			itdsl.ProductRoleOpts{
				Alias:        alias,
				ProductAlias: "product.ctx",
				Name:         name,
				Description:  description,
			},
		).
		Build().
		ProductRole(alias)
}

func createDSLProductRoleWithPermissions(
	t *testing.T,
	productCtx *itdsl.ProductContext,
	name string,
	description *string,
	permissions []string,
) *itdsl.ProductRole {
	t.Helper()
	alias := "product.role." + itshared.Faker.UUID().V4()
	return itdsl.Given(t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "product.ctx", Context: productCtx}).
		ProductRole(
			itdsl.ProductRoleOpts{
				Alias:        alias,
				ProductAlias: "product.ctx",
				Name:         name,
				Description:  description,
				Permissions:  permissions,
			},
		).
		Build().
		ProductRole(alias)
}

func createDSLMembership(
	t *testing.T,
	productCtx *itdsl.ProductContext,
	productUserID string,
	organizationID string,
	roleID string,
) {
	t.Helper()
	userAlias := "product.user." + itshared.Faker.UUID().V4()
	orgAlias := "organization." + itshared.Faker.UUID().V4()
	roleAlias := "product.role." + itshared.Faker.UUID().V4()
	membershipAlias := "membership." + itshared.Faker.UUID().V4()
	itdsl.Given(t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "product.ctx", Context: productCtx}).
		ExistingProductUser(
			itdsl.ExistingProductUserOpts{Alias: userAlias, ProductAlias: "product.ctx", ID: productUserID},
		).
		ExistingProductOrganization(
			itdsl.ExistingProductOrganizationOpts{Alias: orgAlias, ProductAlias: "product.ctx", ID: organizationID},
		).
		ExistingProductRole(
			itdsl.ExistingProductRoleOpts{Alias: roleAlias, ProductAlias: "product.ctx", ID: roleID},
		).
		Membership(
			itdsl.MembershipOpts{
				Alias:             membershipAlias,
				ProductAlias:      "product.ctx",
				ProductUserAlias:  userAlias,
				OrganizationAlias: orgAlias,
				RoleAlias:         roleAlias,
			},
		).
		Build()
}

func setDSLProductUserExternalID(
	t *testing.T,
	productCtx *itdsl.ProductContext,
	productUserID string,
	externalID string,
) {
	t.Helper()
	userAlias := "product.user." + itshared.Faker.UUID().V4()
	itdsl.Given(t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "product.ctx", Context: productCtx}).
		ExistingProductUser(
			itdsl.ExistingProductUserOpts{Alias: userAlias, ProductAlias: "product.ctx", ID: productUserID},
		).
		ProductUserExternalID(
			itdsl.ProductUserExternalIDOpts{ProductUserAlias: userAlias, ExternalID: externalID},
		).
		Build()
}
