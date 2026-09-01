package api

import (
	"context"
	"slices"

	"anchor/internal/domain/product"
	"anchor/internal/domain/product/user"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

func mapToSearchProductInput(request *SearchProductsRequestObject) search.
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
		).WithFullTextSearch(request.Body.FullTextSearch).WithPagination(
		request.Body.Pagination,
	)
}

func mapProductToResponse(prod product.Product) ProductResponse {
	var description *string
	if prod.Description != "" {
		description = &prod.Description
	}

	config := ProductConfigResponse{
		OrganizationApiKeys: ProductOrganizationAPIKeysConfigResponse{
			Prefix: prod.Config.WithDefaults().OrganizationAPIKeys.Prefix,
		},
	}
	if prod.Config.Events != nil && prod.Config.Events.EndpointURL != "" {
		eventsResponse := ProductEventsConfigResponse{
			EndpointUrl:             prod.Config.Events.EndpointURL,
			SigningSecretObfuscated: prod.Config.Events.SigningSecretObfuscated,
		}
		if prod.Config.Events.SigningSecret != "" {
			eventsResponse.SigningSecret = &prod.Config.Events.SigningSecret
		}
		config.Events = &eventsResponse
	}

	return ProductResponse{
		Id:          prod.ID,
		TenantId:    prod.PlatformTenantID,
		Name:        prod.Name,
		Description: description,
		Config:      config,
		CreatedAt:   prod.CreatedAt,
		UpdatedAt:   prod.UpdatedAt,
	}
}

func mapProductRequestConfig(config *ProductConfigRequest) product.Config {
	productConfig := product.DefaultConfig()
	if config == nil {
		return productConfig
	}
	if config.OrganizationApiKeys != nil {
		productConfig.OrganizationAPIKeys.Prefix = config.OrganizationApiKeys.Prefix
	}
	if config.Events != nil {
		eventsConfig := product.EventsConfig{}
		if config.Events.EndpointUrl != nil {
			eventsConfig.EndpointURL = *config.Events.EndpointUrl
		}
		if config.Events.SigningSecret != nil {
			eventsConfig.SigningSecret = *config.Events.SigningSecret
		}
		productConfig.Events = &eventsConfig
	}
	return productConfig
}

func (s *AnchorAPI) SearchProducts(
	ctx context.Context, request SearchProductsRequestObject,
) (SearchProductsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	searchRequest := mapToSearchProductInput(&request)

	input := product.SearchProductInput{
		TenantID: tenantID,
		Request:  searchRequest,
	}

	result, err := s.ProductService.Search(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to search products")
		return nil, err
	}

	return SearchProducts200JSONResponse{
		Items: functional.Slice(result.Items).Map(mapProductToResponse),
		Total: result.Total,
		Count: result.Count,
	}, nil
}

func (s *AnchorAPI) CreateProduct(
	ctx context.Context, request CreateProductRequestObject,
) (CreateProductResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	description := functional.FromPtr(request.Body.Description).OrElse("")

	input := product.CreateProductInput{
		TenantID:    tenantID,
		Name:        request.Body.Name,
		Description: description,
		Config:      mapProductRequestConfig(request.Body.Config),
	}

	createdProduct, err := s.ProductService.Create(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to create product")
		return nil, err
	}

	return CreateProduct201JSONResponse(mapProductToResponse(createdProduct)), nil
}

func (s *AnchorAPI) DeleteProduct(
	ctx context.Context, request DeleteProductRequestObject,
) (DeleteProductResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	productID := request.ProductId

	input := product.DeleteProductInput{
		TenantID:  tenantID,
		ProductID: productID,
	}

	err = s.ProductService.Delete(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", productID).Msg("failed to delete product")
		return nil, err
	}

	return DeleteProduct204Response{}, nil
}

func (s *AnchorAPI) GetProduct(
	ctx context.Context, request GetProductRequestObject,
) (GetProductResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	productID := request.ProductId

	input := product.GetProductInput{
		TenantID:  tenantID,
		ProductID: productID,
	}

	prod, err := s.ProductService.Get(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", productID).Msg("failed to get product")
		return nil, err
	}

	if prod == nil {
		return GetProduct404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PRODUCT_NOT_FOUND", "Product does not exist."),
		)}, nil
	}

	return GetProduct200JSONResponse(mapProductToResponse(*prod)), nil
}

func (s *AnchorAPI) UpdateProduct(
	ctx context.Context, request UpdateProductRequestObject,
) (UpdateProductResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	productID := request.ProductId

	input := product.UpdateProductInput{
		TenantID:    tenantID,
		ProductID:   productID,
		Name:        &request.Body.Name,
		Description: request.Body.Description,
	}
	if request.Body.Config != nil {
		config := mapProductRequestConfig(request.Body.Config)
		input.Config = &config
	}

	updatedProduct, err := s.ProductService.Update(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", productID).Msg("failed to update product")
		return nil, err
	}

	return UpdateProduct200JSONResponse(mapProductToResponse(updatedProduct)), nil
}

// Product User helper functions.
func mapToSearchProductUserInput(
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
			result.Statuses = functional.Slice(*f.Statuses).Map(
				func(status ProductUserStatus) user.ProductUserStatus { return user.ProductUserStatus(status) })
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
		).WithFullTextSearch(request.Body.FullTextSearch).WithPagination(
		request.Body.Pagination,
	)
}

func mapProductUserToResponse(user user.ProductUser) ProductUserResponse {
	name := functional.OptionOf(user.Name, user.Name != "").ToPtr()

	return ProductUserResponse{
		Id:         user.ID,
		ProductId:  user.ProductID,
		Email:      user.Email,
		Name:       name,
		ExternalId: user.ExternalID,
		Status:     ProductUserStatus(user.Status),
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}
}

func (s *AnchorAPI) CreateProductUser(
	ctx context.Context, request CreateProductUserRequestObject,
) (CreateProductUserResponseObject, error) {
	status := functional.FromPtr(request.Body.Status).Map(func(s ProductUserStatus) user.ProductUserStatus {
		return user.ProductUserStatus(s)
	}).OrElse(user.ProductUserStatusActive)

	name := functional.FromPtr(request.Body.Name).OrElse("")

	input := user.CreateProductUserInput{
		ProductID: request.ProductId,
		Email:     request.Body.Email,
		Name:      name,
		Status:    status,
	}

	createdUser, err := s.ProductUserService.Create(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str(
			"product_id", request.ProductId,
		).Msg("failed to create product user")
		return nil, err
	}

	return CreateProductUser201JSONResponse(mapProductUserToResponse(createdUser)), nil
}

func (s *AnchorAPI) SearchProductUsers(
	ctx context.Context, request SearchProductUsersRequestObject,
) (SearchProductUsersResponseObject, error) {
	searchRequest := mapToSearchProductUserInput(&request)

	input := user.SearchProductUserInput{
		ProductID: request.ProductId,
		Request:   searchRequest,
	}

	result, err := s.ProductUserService.Search(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str(
			"product_id", request.ProductId,
		).Msg("failed to search product users")
		return nil, err
	}

	// Using consistent structure with other list responses (items field)
	return SearchProductUsers200JSONResponse{
		Items: functional.Slice(result.Items).Map(mapProductUserToResponse),
		Total: result.Total,
		Count: result.Count,
	}, nil
}

func (s *AnchorAPI) GetProductUser(
	ctx context.Context, request GetProductUserRequestObject,
) (GetProductUserResponseObject, error) {
	productUserID := request.ProductUserId

	input := user.FindProductUserInput{
		ProductID:     request.ProductId,
		ProductUserID: productUserID,
	}

	user, err := s.ProductUserService.Find(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("product_user_id", productUserID).
			Msg("failed to get product user")
		return nil, err
	}

	if user == nil {
		return GetProductUser404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PRODUCT_USER_NOT_FOUND", "Product User does not exist."),
		)}, nil
	}

	return GetProductUser200JSONResponse(mapProductUserToResponse(*user)), nil
}

func (s *AnchorAPI) DeleteProductUser(
	ctx context.Context, request DeleteProductUserRequestObject,
) (DeleteProductUserResponseObject, error) {
	productUserID := request.ProductUserId

	input := user.DeleteProductUserInput{
		ProductID:     request.ProductId,
		ProductUserID: productUserID,
	}

	err := s.ProductUserService.Delete(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("product_user_id", productUserID).
			Msg("failed to delete product user")
		return nil, err
	}

	return DeleteProductUser204Response{}, nil
}

func (s *AnchorAPI) ListUserOrganizations(
	ctx context.Context, request ListUserOrganizationsRequestObject,
) (ListUserOrganizationsResponseObject, error) {
	includePermissions := functional.FromPtr(request.Params.Include).
		Exists(func(include []UserOrganizationInclude) bool {
			return slices.Contains(include, UserOrganizationIncludeRolePermissions)
		})

	input := user.ListUserOrganizationsInput{
		ProductID:          request.ProductId,
		ProductUserID:      request.ProductUserId,
		IncludePermissions: includePermissions,
	}

	memberships, err := s.ProductUserService.ListUserOrganizations(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("product_user_id", request.ProductUserId).
			Msg("failed to list user organizations")
		return nil, err
	}

	return ListUserOrganizations200JSONResponse{
		Items: functional.Slice(memberships).Map(
			func(m user.OrganizationMembership) UserOrganizationResponse {
				return mapUserOrgMembershipToResponse(m, includePermissions)
			}),
	}, nil
}

func (s *AnchorAPI) GetUserOrganization(
	ctx context.Context, request GetUserOrganizationRequestObject,
) (GetUserOrganizationResponseObject, error) {
	includePermissions := functional.FromPtr(request.Params.Include).
		Exists(func(include []UserOrganizationInclude) bool {
			return slices.Contains(include, UserOrganizationIncludeRolePermissions)
		})

	input := user.GetUserOrganizationInput{
		ProductID:          request.ProductId,
		ProductUserID:      request.ProductUserId,
		OrganizationID:     request.OrganizationId,
		IncludePermissions: includePermissions,
	}

	membership, err := s.ProductUserService.GetUserOrganization(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("product_user_id", request.ProductUserId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to get user organization")
		return nil, err
	}

	if membership == nil {
		return GetUserOrganization404JSONResponse{NotFoundJSONResponse(
			notFoundBody("USER_ORGANIZATION_NOT_FOUND", "User Organization does not exist."),
		)}, nil
	}

	resp := mapUserOrgMembershipToResponse(*membership, includePermissions)
	return GetUserOrganization200JSONResponse(resp), nil
}

func mapUserOrgMembershipToResponse(
	m user.OrganizationMembership, includePermissions bool,
) UserOrganizationResponse {
	role := UserOrganizationRoleResponse{
		Id:   m.RoleID,
		Name: m.RoleName,
	}
	if includePermissions {
		permissions := m.RolePermissions
		if permissions == nil {
			permissions = []string{}
		}
		role.Permissions = &permissions
	}

	response := UserOrganizationResponse{
		JoinedAt: m.JoinedAt,
		Role:     role,
	}
	response.Organization.Id = m.OrganizationID
	response.Organization.Name = m.OrganizationName
	response.Organization.Description = m.OrganizationDescription
	response.Organization.Metadata = mapMetadataToResponse(m.OrganizationMetadataJSON)
	return response
}
