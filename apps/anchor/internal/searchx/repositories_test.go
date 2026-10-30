package searchx_test

import (
	"context"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/email"
	"anchor/internal/domain/integration"
	"anchor/internal/domain/invitation"
	"anchor/internal/domain/license"
	"anchor/internal/domain/organization"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/domain/organizationinvitation"
	"anchor/internal/domain/permission"
	"anchor/internal/domain/platform"
	"anchor/internal/domain/product"
	"anchor/internal/domain/product/apikey"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/product/user"
	"anchor/internal/domain/resource"
	emailrepository "anchor/internal/email/repository"
	invitationrepository "anchor/internal/invitation/repository"
	licenserepository "anchor/internal/license/repository"
	"anchor/internal/repository"
	resourcerepository "anchor/internal/resource/repository"
)

// unknownSort asks for a sort field no search maps.
func unknownSort[F any, S ~string]() search.Request[F, S] {
	return search.Request[F, S]{
		Sort:       []search.Sort[S]{{Field: S("searchtest_unknown_field"), Direction: search.SortAscending}},
		Pagination: search.Pagination{Limit: 10},
	}
}

// Request validation refuses an unknown sort_by before a search reads anything,
// so the 400 an unknown sort earns over HTTP is a validation error. This test
// pins the line behind it: every search repository maps its sorts with
// transactor.StrictKeys, so a field that validation lets through but the
// repository does not map fails with search.ErrUnknownSort, which the error
// handler answers with 400 INVALID_SORT, instead of silently ordering by
// something else. The ordering is built before the first statement runs, so the
// repositories need no database here.
func TestSearchRepositoriesRefuseASortTheyDoNotMap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := zerolog.Nop()
	tests := map[string]func() error{
		"searchProducts": func() error {
			_, err := repository.NewProductRepository(nil, nil, logger).SearchByTenantID(
				ctx, "tenant", unknownSort[product.SearchProductFilter, product.SortFieldProduct]())
			return err
		},
		"searchProductResourcePermissions": func() error {
			_, err := repository.NewProductResourcePermissionRepository(nil, nil, logger).SearchByProduct(
				ctx, "product", unknownSort[
					resourcepermission.SearchProductResourcePermissionFilter,
					resourcepermission.SortFieldProductResourcePermission,
				]())
			return err
		},
		"searchResources": func() error {
			_, err := resourcerepository.NewRepository(nil).Search(
				ctx, "product", "organization", unknownSort[resource.Filter, resource.SortField]())
			return err
		},
		"searchProductPermissions": func() error {
			_, err := repository.NewProductPermissionRepository(nil, nil, logger).SearchByProduct(
				ctx, "product", unknownSort[
					permission.SearchProductPermissionFilter, permission.SortFieldProductPermission,
				]())
			return err
		},
		"searchPlatformInvitations": func() error {
			_, err := repository.NewPlatformInvitationRepository(nil, nil, logger).SearchByTenantID(
				ctx, "tenant", unknownSort[
					invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation,
				]())
			return err
		},
		"searchPlatformUsers": func() error {
			_, err := repository.NewPlatformTenantUserRepository(nil, nil, logger).SearchByTenantID(
				ctx, "tenant", unknownSort[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]())
			return err
		},
		"searchProductRoles": func() error {
			_, err := repository.NewProductRoleRepository(nil, nil, logger).SearchByProductID(
				ctx, "product", unknownSort[role.SearchProductRoleFilter, role.SortFieldProductRole]())
			return err
		},
		"searchProductAPIKeys": func() error {
			_, err := repository.NewProductAPIKeyRepository(nil, nil, logger).SearchByProductID(
				ctx, apikey.SearchProductAPIKeysInput{
					ProductID: "product",
					Request:   unknownSort[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey](),
				})
			return err
		},
		"searchProductUsers": func() error {
			_, err := repository.NewProductUserRepository(nil, nil, logger).SearchByProductID(
				ctx, "product", unknownSort[user.SearchProductUserFilter, user.SortFieldProductUser]())
			return err
		},
		"searchOrganizationAPIKeys": func() error {
			_, err := repository.NewOrganizationAPIKeyRepository(nil, nil, logger).SearchByOrganizationID(
				ctx, orgapikey.SearchOrganizationAPIKeysInput{
					ProductID:      "product",
					OrganizationID: "organization",
					Request: unknownSort[
						orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey,
					](),
				})
			return err
		},
		"searchProductOrganizations": func() error {
			_, err := repository.NewOrganizationRepository(nil, nil, logger).SearchByProductID(
				ctx, "product", unknownSort[
					organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization,
				]())
			return err
		},
		"searchOrganizationMembers": func() error {
			_, err := repository.NewOrganizationMembershipRepository(nil, logger).SearchByOrgID(
				ctx, "product", "organization",
				unknownSort[organization.SearchMembersFilter, organization.SortFieldMember]())
			return err
		},
		"searchUserOrganizations": func() error {
			_, err := repository.NewOrganizationMembershipRepository(nil, logger).SearchByProductUserID(
				ctx, "product", "user", false, unknownSort[
					user.SearchUserOrganizationsFilter, user.SortFieldUserOrganization,
				]())
			return err
		},
		"searchOrganizationInvitations and searchProductOrganizationInvitations": func() error {
			_, err := invitationrepository.NewRepository(nil, logger).Search(
				ctx, "product", functional.None[string](),
				unknownSort[organizationinvitation.SearchFilter, organizationinvitation.SortField]())
			return err
		},
		"searchOrganizationLicenses": func() error {
			_, err := licenserepository.NewOrganizationLicenseRepository(nil, nil, logger).Search(
				ctx, license.SearchOrganizationLicensesInput{
					TenantID:  "tenant",
					ProductID: "product",
					Request: unknownSort[
						license.SearchOrganizationLicenseFilter, license.SortFieldOrganizationLicense,
					](),
				})
			return err
		},
		"searchIntegrationAuditLogs": func() error {
			_, err := repository.NewIntegrationAuditLogRepository(nil, nil, logger).SearchByInstanceScoped(
				ctx, integration.SearchAuditLogsInput{
					TenantID:              "tenant",
					ProductID:             "product",
					IntegrationInstanceID: "instance",
					Request:               unknownSort[integration.SearchAuditLogsFilter, integration.SortFieldAuditLog](),
				})
			return err
		},
		"searchEmailTemplates": func() error {
			_, err := emailrepository.NewTemplateRepository(nil, nil, logger).Search(
				ctx, email.SearchTemplatesInput{
					TenantID:  "tenant",
					ProductID: "product",
					Request:   unknownSort[email.SearchTemplatesFilter, email.SortFieldTemplate](),
				})
			return err
		},
		"searchEmailSends": func() error {
			_, err := emailrepository.NewSendRecordRepository(nil, nil, logger).Search(
				ctx, email.SearchSendsInput{
					TenantID:  "tenant",
					ProductID: "product",
					Request:   unknownSort[email.SearchSendsFilter, email.SortFieldSend](),
				})
			return err
		},
		"searchLicenseTemplates": func() error {
			_, err := licenserepository.NewTemplateRepository(nil, nil, logger).Search(
				ctx, license.SearchTemplatesInput{
					TenantID:  "tenant",
					ProductID: "product",
					Request:   unknownSort[license.SearchTemplatesFilter, license.SortFieldTemplate](),
				})
			return err
		},
		"searchOrganizationLicenseHistory": func() error {
			_, err := licenserepository.NewOrganizationLicenseChangeRepository(nil, nil, logger).Search(
				ctx, license.SearchLicenseChangesInput{
					TenantID:       "tenant",
					ProductID:      "product",
					OrganizationID: "organization",
					Request:        unknownSort[license.SearchLicenseChangesFilter, license.SortFieldChange](),
				})
			return err
		},
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := run()
			require.Error(t, err)
			assert.ErrorIs(t, err, search.ErrUnknownSort)
		})
	}
}
