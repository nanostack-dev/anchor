package api

// The generated searches get their filter and pagination checks from the
// framework, with other messages. A client that sent a request Anchor refused
// must get the same 400 body, so every search handler runs the request through
// the validation Anchor always had (the struct tags of the filter, the
// pagination, the sort_by values the resource offers) before it builds the
// search. The functions below rebuild that request exactly as the handler did
// when the service validated it; the result is only validated, never run.
//
// Do not read the legacy mappers as a second implementation of a search: the
// generated builder is the one that runs.

import (
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openapi_types "github.com/oapi-codegen/runtime/types"

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
	"anchor/internal/domain/product/user"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/searchvalid"
)

func legacyMapToSearchEmailTemplatesRequest(
	body *SearchEmailTemplatesJSONRequestBody,
) search.Request[email.SearchTemplatesFilter, email.SortFieldTemplate] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f EmailTemplateFilter) email.SearchTemplatesFilter {
			return email.SearchTemplatesFilter{IDs: f.Ids, Slugs: f.Slugs, IsActive: f.IsActive}
		}).
		ToPtr()
	sortBy, sortDirection := searchvalid.Sort(
		body.SortBy, body.SortDirection, email.SortFieldTemplateCreatedAt, search.SortDescending,
	)

	return search.NewRequest[email.SearchTemplatesFilter, email.SortFieldTemplate]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapToSearchEmailSendsRequest(
	body *SearchEmailSendsJSONRequestBody,
) search.Request[email.SearchSendsFilter, email.SortFieldSend] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f EmailSendFilter) email.SearchSendsFilter {
			return email.SearchSendsFilter{TemplateIDs: f.TemplateIds, Statuses: f.Statuses}
		}).
		ToPtr()
	sortBy, sortDirection := searchvalid.Sort(
		body.SortBy, body.SortDirection, email.SortFieldSendCreatedAt, search.SortDescending,
	)

	return search.NewRequest[email.SearchSendsFilter, email.SortFieldSend]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapToSearchIntegrationAuditLogsRequest(
	body *SearchIntegrationAuditLogsJSONRequestBody,
) search.Request[integration.SearchAuditLogsFilter, integration.SortFieldAuditLog] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f IntegrationAuditLogFilter) integration.SearchAuditLogsFilter {
			return integration.SearchAuditLogsFilter{
				Actions:       f.Actions,
				Severities:    f.Severities,
				CreatedAfter:  f.CreatedAfter,
				CreatedBefore: f.CreatedBefore,
			}
		}).
		ToPtr()
	sortBy, sortDirection := searchvalid.Sort(
		body.SortBy, body.SortDirection, integration.SortFieldAuditLogCreatedAt, search.SortDescending,
	)

	return search.NewRequest[integration.SearchAuditLogsFilter, integration.SortFieldAuditLog]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapToSearchLicenseTemplatesRequest(
	body *SearchLicenseTemplatesJSONRequestBody,
) search.Request[license.SearchTemplatesFilter, license.SortFieldTemplate] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f LicenseTemplateFilter) license.SearchTemplatesFilter {
			return license.SearchTemplatesFilter{Statuses: f.Statuses}
		}).
		ToPtr()
	sortBy, sortDirection := searchvalid.Sort(
		body.SortBy, body.SortDirection, license.SortFieldTemplateName, search.SortAscending,
	)

	return search.NewRequest[license.SearchTemplatesFilter, license.SortFieldTemplate]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapToSearchProductOrganizationInput(
	searchReqBody *SearchProductOrganizationsJSONRequestBody,
) search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization] {
	filter := functional.FromPtr(searchReqBody.Filter).
		Map(func(f OrganizationFilter) organization.SearchProductOrganizationFilter {
			return organization.SearchProductOrganizationFilter{
				IDs:   f.Ids,
				Names: f.Names,
			}
		}).
		ToPtr()

	return search.NewRequest[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]().
		WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(searchReqBody.Pagination)
}

func legacyMapToSearchOrganizationAPIKeyInput(
	searchReqBody *SearchOrganizationAPIKeysJSONRequestBody,
) search.Request[
	orgapikey.SearchOrganizationAPIKeyFilter,
	orgapikey.SortFieldOrganizationAPIKey,
] {
	filter := functional.FromPtr(searchReqBody.Filter).
		Map(func(f OrganizationAPIKeyFilter) orgapikey.SearchOrganizationAPIKeyFilter {
			result := orgapikey.SearchOrganizationAPIKeyFilter{
				OrganizationAPIKeyIDs: f.Ids,
				Names:                 f.Names,
				LastUsedBefore:        f.LastUsedBefore,
				LastUsedAfter:         f.LastUsedAfter,
			}
			if f.Status != nil {
				result.Status = functional.Slice(*f.Status).Map(func(s OrganizationAPIKeyStatus) string {
					return string(s)
				})
			}
			return result
		}).
		ToPtr()

	return search.NewRequest[
		orgapikey.SearchOrganizationAPIKeyFilter,
		orgapikey.SortFieldOrganizationAPIKey,
	]().WithFilter(filter).
		WithSort(searchReqBody.SortBy, searchReqBody.SortDirection).
		WithFullTextSearch(searchReqBody.FullTextSearch).
		WithPagination(searchReqBody.Pagination)
}

func legacyMapSearchOrganizationInvitationsRequestToInput(
	body SearchOrganizationInvitationsJSONRequestBody,
) search.Request[organizationinvitation.SearchFilter, organizationinvitation.SortField] {
	var req search.Request[organizationinvitation.SearchFilter, organizationinvitation.SortField]

	var filter *organizationinvitation.SearchFilter
	if body.Filter != nil {
		filter = &organizationinvitation.SearchFilter{
			Statuses: functional.FromPtr(body.Filter.Statuses).OrElse(nil),
			Emails: functional.Slice(functional.FromPtr(body.Filter.Emails).OrElse(nil)).Map(
				func(email openapi_types.Email) string { return string(email) },
			),
		}
	}

	return req.WithFilter(filter).
		WithSort(body.SortBy, body.SortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapToSearchLicenseChangesRequest(
	body *SearchOrganizationLicenseHistoryJSONRequestBody,
) search.Request[license.SearchLicenseChangesFilter, license.SortFieldChange] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f OrganizationLicenseHistoryFilter) license.SearchLicenseChangesFilter {
			return license.SearchLicenseChangesFilter{
				Types:         f.Types,
				ChangedAfter:  f.ChangedAfter,
				ChangedBefore: f.ChangedBefore,
			}
		}).
		ToPtr()
	sortBy, sortDirection := searchvalid.Sort(
		body.SortBy, body.SortDirection, license.SortFieldChangeChangedAt, search.SortDescending,
	)

	return search.NewRequest[license.SearchLicenseChangesFilter, license.SortFieldChange]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapToSearchOrganizationLicensesRequest(
	body *SearchOrganizationLicensesJSONRequestBody,
) search.Request[license.SearchOrganizationLicenseFilter, license.SortFieldOrganizationLicense] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f OrganizationLicenseFilter) license.SearchOrganizationLicenseFilter {
			return license.SearchOrganizationLicenseFilter{
				OrganizationIDs:    f.OrganizationIds,
				LicenseTemplateIDs: f.LicenseTemplateIds,
				Licensed:           f.Licensed,
			}
		}).
		ToPtr()

	sortBy, sortDirection := searchvalid.Sort(
		body.SortBy, body.SortDirection,
		license.SortFieldOrganizationLicenseOrganizationName, search.SortAscending,
	)

	return search.NewRequest[license.SearchOrganizationLicenseFilter, license.SortFieldOrganizationLicense]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapSearchOrganizationMembersRequestToInput(
	request SearchOrganizationMembersRequestObject,
) search.Request[organization.SearchMembersFilter, organization.SortFieldMember] {
	var filter *organization.SearchMembersFilter
	var req search.Request[organization.SearchMembersFilter, organization.SortFieldMember]

	if request.Body == nil {
		return req
	}

	if request.Body.Filter != nil {
		filter = &organization.SearchMembersFilter{}

		if request.Body.Filter.ProductUserIds != nil {
			filter.ProductUserIDs = append(filter.ProductUserIDs, *request.Body.Filter.ProductUserIds...)
		}
		if request.Body.Filter.ExternalIds != nil {
			filter.ExternalIDs = *request.Body.Filter.ExternalIds
		}
		if request.Body.Filter.Emails != nil {
			filter.Emails = append(
				filter.Emails,
				functional.Slice(*request.Body.Filter.Emails).Map(func(e openapi_types.Email) string {
					return string(e)
				})...)
		}
		if request.Body.Filter.RoleIds != nil {
			filter.RoleIDs = append(filter.RoleIDs, *request.Body.Filter.RoleIds...)
		}
	}

	sortBy, sortDirection := searchvalid.Sort(
		request.Body.SortBy, request.Body.SortDirection,
		organization.SortFieldMemberJoinedAt, search.SortDescending,
	)

	return req.WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(request.Body.FullTextSearch).WithPagination(request.Body.Pagination)
}

func legacyMapToSearchPlatformInvitationRequest(
	searchReqBody *SearchPlatformInvitationsJSONRequestBody,
) search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation] {
	filter := functional.FromPtr(searchReqBody.Filter).
		Map(func(f PlatformInvitationFilter) invitation.SearchPlatformInvitationFilter {
			return invitation.SearchPlatformInvitationFilter{
				Emails: f.Emails,
				IDs:    f.Ids,
			}
		}).
		ToPtr()

	return search.NewRequest[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]().
		WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(searchReqBody.Pagination)
}

func legacyMapToSearchPlatformUserInput(
	searchReqBody *SearchPlatformUsersJSONRequestBody,
) search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser] {
	filter := functional.FromPtr(searchReqBody.Filter).
		Map(func(f PlatformUserFilter) platform.SearchPlatformUserFilter {
			return platform.SearchPlatformUserFilter{
				Emails: f.Emails,
				IDs:    f.Ids,
				Roles:  f.Roles,
			}
		}).
		ToPtr()
	return search.NewRequest[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]().
		WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(searchReqBody.Pagination)
}

func legacyMapToSearchProductInput(request *SearchProductsRequestObject) search.
	Request[product.SearchProductFilter, product.SortFieldProduct] {
	filter := functional.FromPtr(request.Body.Filter).Map(func(f ProductFilter) product.SearchProductFilter {
		return product.SearchProductFilter{
			IDs:   f.Ids,
			Names: f.Names,
		}
	}).ToPtr()

	return search.NewRequest[product.SearchProductFilter, product.SortFieldProduct]().
		WithFilter(filter).
		WithSort(
			request.Body.SortBy,
			request.Body.SortDirection,
		).WithFullTextSearch(request.Body.FullTextSearch).WithPagination(request.Body.Pagination)
}

func legacyMapToSearchProductUserInput(
	request *SearchProductUsersRequestObject,
) search.Request[user.SearchProductUserFilter,
	user.SortFieldProductUser] {
	var req search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]

	if request.Body == nil {
		return req
	}

	// Map filter fields based on the consistent OpenAPI spec structure
	filter := functional.FromPtr(request.Body.Filter).Map(func(f ProductUserFilter) user.SearchProductUserFilter {
		result := user.SearchProductUserFilter{}

		if f.Ids != nil {
			result.IDs = functional.Slice(*f.Ids).Map(func(id Ksuid) string { return id })
		}
		if f.Emails != nil {
			result.Emails = f.Emails
		}
		if f.Names != nil {
			result.Names = *f.Names
		}
		if f.Statuses != nil {
			result.Statuses = functional.Slice(*f.Statuses).Map(foldProductUserStatus)
		}
		if f.ExternalIds != nil {
			result.ExternalIDs = f.ExternalIds
		}

		return result
	}).ToPtr()

	return req.WithFilter(filter).
		WithSort(
			request.Body.SortBy,
			request.Body.SortDirection,
		).WithFullTextSearch(request.Body.FullTextSearch).WithPagination(request.Body.Pagination)
}

func legacyMapToSearchUserOrganizationsRequest(
	body *SearchUserOrganizationsJSONRequestBody,
) search.Request[user.SearchUserOrganizationsFilter, user.SortFieldUserOrganization] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f UserOrganizationFilter) user.SearchUserOrganizationsFilter {
			return user.SearchUserOrganizationsFilter{OrganizationIDs: f.OrganizationIds, RoleIDs: f.RoleIds}
		}).
		ToPtr()
	sortBy, sortDirection := searchvalid.Sort(
		body.SortBy, body.SortDirection, user.SortFieldUserOrganizationJoinedAt, search.SortAscending,
	)

	return search.NewRequest[user.SearchUserOrganizationsFilter, user.SortFieldUserOrganization]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func legacyMapToSearchProductAPIKeyInput(
	searchReqBody *SearchProductAPIKeysJSONRequestBody,
) search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey] {
	filter := functional.FromPtr(searchReqBody.Filter).
		Map(func(f ProductAPIKeyFilter) apikey.SearchProductAPIKeyFilter {
			result := apikey.SearchProductAPIKeyFilter{
				ProductAPIKeyIDs: f.Ids,
				Names:            f.Names,
			}
			if f.Status != nil {
				result.Status = functional.Slice(*f.Status).Map(func(s ProductAPIKeyStatus) string {
					return string(s)
				})
			}
			return result
		}).
		ToPtr()
	return search.NewRequest[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]().
		WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(searchReqBody.Pagination)
}

func legacyMapToSearchProductPermissionInput(
	searchReqBody *SearchProductPermissionsJSONRequestBody,
) search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission] {
	filter := functional.FromPtr(searchReqBody.Filter).
		Map(func(f ProductPermissionFilter) permission.SearchProductPermissionFilter {
			return permission.SearchProductPermissionFilter{
				Names: lowerPermissionNames(f.Names),
			}
		}).
		ToPtr()
	return search.NewRequest[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]().
		WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(searchReqBody.Pagination)
}

func legacyMapToSearchProductResourcePermissionInput(
	req *ProductResourcePermissionSearchRequest,
) search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission] {
	filter := functional.FromPtr(req.Filter).
		Map(func(f ProductResourcePermissionFilter) resourcepermission.SearchProductResourcePermissionFilter {
			return resourcepermission.SearchProductResourcePermissionFilter{
				Names:         f.Names,
				ResourceTypes: f.ResourceTypes,
			}
		}).
		ToPtr()
	return search.NewRequest[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission]().
		WithFullTextSearch(req.FullTextSearch).
		WithFilter(filter).
		WithSort(req.SortBy, req.SortDirection).
		WithPagination(req.Pagination)
}

func legacyResourceTypeRequest(
	body *SearchProductResourceTypesJSONRequestBody,
) search.Request[resourcetype.Filter, resourcetype.SortField] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f ProductResourceTypeFilter) resourcetype.Filter {
			return resourcetype.Filter{IDs: f.Ids, Slugs: f.Slugs}
		}).
		ToPtr()
	return search.NewRequest[resourcetype.Filter, resourcetype.SortField]().
		WithFullTextSearch(body.FullTextSearch).
		WithFilter(filter).
		WithSort(body.SortBy, body.SortDirection).
		WithPagination(body.Pagination)
}

func validateEmailTemplatesSearch(tenantID, productID string, body *SearchEmailTemplatesJSONRequestBody) error {
	return email.SearchTemplatesInput{
		TenantID: tenantID, ProductID: productID, Request: legacyMapToSearchEmailTemplatesRequest(body),
	}.Validate()
}

func validateEmailSendsSearch(tenantID, productID string, body *SearchEmailSendsJSONRequestBody) error {
	return email.SearchSendsInput{
		TenantID: tenantID, ProductID: productID, Request: legacyMapToSearchEmailSendsRequest(body),
	}.Validate()
}

func validateAuditLogsSearch(
	tenantID, productID, instanceID string, body *SearchIntegrationAuditLogsJSONRequestBody,
) error {
	return integration.SearchAuditLogsInput{
		TenantID: tenantID, ProductID: productID, IntegrationInstanceID: instanceID,
		Request: legacyMapToSearchIntegrationAuditLogsRequest(body),
	}.Validate()
}

func validateLicenseTemplatesSearch(tenantID, productID string, body *SearchLicenseTemplatesJSONRequestBody) error {
	return license.SearchTemplatesInput{
		TenantID: tenantID, ProductID: productID, Request: legacyMapToSearchLicenseTemplatesRequest(body),
	}.Validate()
}

func validateProductOrganizationsSearch(
	tenantID, productID string, includes []organization.Include, body *SearchProductOrganizationsJSONRequestBody,
) error {
	return organization.SearchProductOrganizationsInput{
		TenantID: tenantID, ProductID: productID, Include: includes,
		Request: legacyMapToSearchProductOrganizationInput(body),
	}.Validate()
}

func validateOrganizationAPIKeysSearch(
	productID, organizationID string, body *SearchOrganizationAPIKeysJSONRequestBody,
) error {
	return orgapikey.SearchOrganizationAPIKeysInput{
		ProductID: productID, OrganizationID: organizationID, Request: legacyMapToSearchOrganizationAPIKeyInput(body),
	}.Validate()
}

func validateOrganizationInvitationsSearch(
	productID, organizationID string, body SearchOrganizationInvitationsJSONRequestBody,
) error {
	return organizationinvitation.SearchInput{
		ProductID: productID, OrganizationID: organizationID,
		Request: legacyMapSearchOrganizationInvitationsRequestToInput(body),
	}.Validate()
}

func validateProductInvitationsSearch(productID string, body SearchOrganizationInvitationsJSONRequestBody) error {
	return organizationinvitation.SearchInProductInput{
		ProductID: productID, Request: legacyMapSearchOrganizationInvitationsRequestToInput(body),
	}.Validate()
}

func validateLicenseChangesSearch(
	tenantID, productID, organizationID string, body *SearchOrganizationLicenseHistoryJSONRequestBody,
) error {
	return license.SearchLicenseChangesInput{
		TenantID: tenantID, ProductID: productID, OrganizationID: organizationID,
		Request: legacyMapToSearchLicenseChangesRequest(body),
	}.Validate()
}

func validateOrganizationLicensesSearch(
	tenantID, productID string, body *SearchOrganizationLicensesJSONRequestBody,
) error {
	return license.SearchOrganizationLicensesInput{
		TenantID: tenantID, ProductID: productID, Request: legacyMapToSearchOrganizationLicensesRequest(body),
	}.Validate()
}

func validateOrganizationMembersSearch(request SearchOrganizationMembersRequestObject) error {
	return organization.SearchMembersInput{
		ProductID: request.ProductId, OrganizationID: request.OrganizationId,
		Request: legacyMapSearchOrganizationMembersRequestToInput(request),
	}.Validate()
}

func validatePlatformInvitationsSearch(tenantID string, body *SearchPlatformInvitationsJSONRequestBody) error {
	if body == nil {
		body = &SearchPlatformInvitationsJSONRequestBody{}
	}
	return invitation.SearchInvitationInput{
		TenantID: tenantID, Request: legacyMapToSearchPlatformInvitationRequest(body),
	}.Validate()
}

func validatePlatformUsersSearch(tenantID string, body *SearchPlatformUsersJSONRequestBody) error {
	return platform.SearchPlatformUsersInput{
		TenantID: tenantID, Request: legacyMapToSearchPlatformUserInput(body),
	}.Validate()
}

func validateProductsSearch(tenantID string, request *SearchProductsRequestObject) error {
	return product.SearchProductInput{
		TenantID: tenantID, Request: legacyMapToSearchProductInput(request),
	}.Validate()
}

func validateProductUsersSearch(productID string, request *SearchProductUsersRequestObject) error {
	return user.SearchProductUserInput{
		ProductID: productID, Request: legacyMapToSearchProductUserInput(request),
	}.Validate()
}

func validateUserOrganizationsSearch(
	productID, productUserID string, includePermissions bool, body *SearchUserOrganizationsJSONRequestBody,
) error {
	return user.SearchUserOrganizationsInput{
		ProductID: productID, ProductUserID: productUserID, IncludePermissions: includePermissions,
		Request: legacyMapToSearchUserOrganizationsRequest(body),
	}.Validate()
}

func validateProductAPIKeysSearch(productID string, body *SearchProductAPIKeysJSONRequestBody) error {
	return apikey.SearchProductAPIKeysInput{
		ProductID: productID, Request: legacyMapToSearchProductAPIKeyInput(body),
	}.Validate()
}

func validateProductPermissionsSearch(productID string, body *SearchProductPermissionsJSONRequestBody) error {
	return permission.SearchProductPermissionInput{
		ProductID: productID, Request: legacyMapToSearchProductPermissionInput(body),
	}.Validate()
}

func validateProductResourcePermissionsSearch(productID string, body *ProductResourcePermissionSearchRequest) error {
	if body == nil {
		body = &ProductResourcePermissionSearchRequest{}
	}
	return resourcepermission.SearchProductResourcePermissionInput{
		ProductID: productID, Request: legacyMapToSearchProductResourcePermissionInput(body),
	}.Validate()
}

func validateResourceTypesSearch(productID string, body *SearchProductResourceTypesJSONRequestBody) error {
	if body == nil {
		body = &SearchProductResourceTypesJSONRequestBody{}
	}
	return resourcetype.SearchInput{
		ProductID: productID, Request: legacyResourceTypeRequest(body),
	}.Validate()
}
