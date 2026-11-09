package api

import (
	"context"
	"log/slog"
	"slices"

	"anchor/internal/domain/product"
	"anchor/internal/domain/product/user"
	"anchor/internal/events"
	"anchor/internal/security"

	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

// productSearch builds the search from the request body. The tenant scope comes
// from the authenticated principal; the body never sets it.
func productSearch(tenantID string, body *SearchProductsJSONRequestBody) repository.ProductSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := repository.NewProductSearch(tenantID)
	q = searchexec.Some(q, q.ID().In, f.Ids)
	q = searchexec.Some(q, q.Name().In, f.Names)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.ProductSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}

func mapProductToResponse(prod product.Product) ProductResponse {
	var description *string
	if prod.Description != "" {
		description = &prod.Description
	}

	config := ProductConfigResponse{
		Protected: prod.Config.Protected,
		OrganizationApiKeys: ProductOrganizationAPIKeysConfigResponse{
			Prefix: prod.Config.WithDefaults().OrganizationAPIKeys.Prefix,
		},
	}
	if prod.Config.Events != nil && prod.Config.Events.EndpointURL != "" {
		eventsResponse := ProductEventsConfigResponse{
			EndpointUrl:             prod.Config.Events.EndpointURL,
			SigningSecretObfuscated: prod.Config.Events.SigningSecretObfuscated,
			Events:                  prod.Config.Events.Events,
			DeliveryStatus:          ProductEventDeliveryStatus(prod.Config.Events.DeliveryStatus),
			ConsecutiveFailedCalls:  prod.Config.Events.ConsecutiveFailedCalls,
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
	productConfig.Protected = functional.FromPtr(config.Protected).OrElse(false)
	if config.OrganizationApiKeys != nil {
		productConfig.OrganizationAPIKeys.Prefix = config.OrganizationApiKeys.Prefix
	}
	if config.Events != nil {
		eventsConfig := product.EventsConfig{}
		if config.Events.EndpointUrl != nil {
			eventsConfig.EndpointURL = *config.Events.EndpointUrl
		}
		if config.Events.Events != nil {
			eventsConfig.Events = *config.Events.Events
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

	if err = validateProductsSearch(tenantID, &request); err != nil {
		return nil, err
	}
	result, err := s.ProductService.Search(ctx, productSearch(tenantID, request.Body))
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search products")
		return nil, err
	}

	return SearchProducts200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapProductToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      result.Count,
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
		log.ErrorTo(ctx, s.logger, err, "failed to create product")
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
		log.ErrorTo(ctx, s.logger, err, "failed to delete product", slog.String("product_id", productID))
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
		log.ErrorTo(ctx, s.logger, err, "failed to get product", slog.String("product_id", productID))
		return nil, err
	}

	return GetProduct200JSONResponse(mapProductToResponse(prod)), nil
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
		input.Config = &product.UpdateConfigInput{
			Protected:           request.Body.Config.Protected,
			OrganizationAPIKeys: config.OrganizationAPIKeys,
			Events:              config.Events,
		}
	}

	updatedProduct, err := s.ProductService.Update(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to update product", slog.String("product_id", productID))
		return nil, err
	}

	return UpdateProduct200JSONResponse(mapProductToResponse(updatedProduct)), nil
}

func (s *AnchorAPI) GetProductEventsCatalog(
	ctx context.Context, _ GetProductEventsCatalogRequestObject,
) (GetProductEventsCatalogResponseObject, error) {
	definitions := s.ProductService.EventsCatalog(ctx)
	items := functional.Slice(definitions).Map(func(d events.Definition) ProductEventDefinitionResponse {
		return ProductEventDefinitionResponse{
			Type:        string(d.Type),
			Name:        d.Name,
			Description: d.Description,
			GroupType:   d.GroupType,
			GroupName:   d.GroupName,
		}
	})
	return GetProductEventsCatalog200JSONResponse{
		Items: items,
	}, nil
}

// productUserSearch builds the search from the request body. The product scope
// comes from the route, which the authorization middleware checked against the
// principal; the body never sets it.
func productUserSearch(
	productID string, body *SearchProductUsersJSONRequestBody,
) repository.ProductUserSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	statuses := functional.Slice(orEmpty(f.Statuses)).Map(foldProductUserStatus)
	q := repository.NewProductUserSearch(productID)
	q = searchexec.Some(q, q.ID().In, orEmpty(f.Ids))
	q = searchexec.Some(q, q.Email().In, f.Emails)
	q = searchexec.Some(q, q.Name().ContainsAny, orEmpty(f.Names))
	q = searchexec.Some(q, q.Status().In, statuses)
	q = searchexec.Some(q, q.ExternalID().In, f.ExternalIds)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.ProductUserSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}

// foldProductUserStatus ignores the letter case of a requested status: Anchor
// returned ACTIVE until 2026-11-08, so a client may send it back. Any other
// value passes through unchanged, and the service validation rejects it.
func foldProductUserStatus(status ProductUserStatus) user.ProductUserStatus {
	parsed, err := user.ParseProductUserStatus(string(status))
	if err != nil {
		return user.ProductUserStatus(status)
	}
	return parsed
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
	status := functional.FromPtr(request.Body.Status).Map(foldProductUserStatus).OrElse(user.ProductUserStatusActive)

	input := user.CreateProductUserInput{
		ProductID:  request.ProductId,
		Email:      request.Body.Email,
		Name:       request.Body.Name,
		Status:     status,
		ExternalID: request.Body.ExternalId,
	}

	createdUser, err := s.ProductUserService.Create(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to create product user",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	return CreateProductUser201JSONResponse(mapProductUserToResponse(createdUser)), nil
}

func (s *AnchorAPI) SearchProductUsers(
	ctx context.Context, request SearchProductUsersRequestObject,
) (SearchProductUsersResponseObject, error) {
	if err := validateProductUsersSearch(request.ProductId, &request); err != nil {
		return nil, err
	}
	q := productUserSearch(request.ProductId, request.Body)

	result, err := s.ProductUserService.Search(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search product users",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	// Using consistent structure with other list responses (items field)
	return SearchProductUsers200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapProductUserToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      result.Count,
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
		log.ErrorTo(ctx, s.logger, err, "failed to get product user",
			slog.String("product_id", request.ProductId),
			slog.String("product_user_id", productUserID),
		)
		return nil, err
	}

	return GetProductUser200JSONResponse(mapProductUserToResponse(user)), nil
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
		log.ErrorTo(ctx, s.logger, err, "failed to delete product user",
			slog.String("product_id", request.ProductId),
			slog.String("product_user_id", productUserID),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to list user organizations",
			slog.String("product_id", request.ProductId),
			slog.String("product_user_id", request.ProductUserId),
		)
		return nil, err
	}

	return ListUserOrganizations200JSONResponse{
		Items: functional.Slice(memberships).Map(
			func(m user.OrganizationMembership) UserOrganizationResponse {
				return mapUserOrgMembershipToResponse(m, includePermissions)
			}),
	}, nil
}

func (s *AnchorAPI) SearchUserOrganizations(
	ctx context.Context, request SearchUserOrganizationsRequestObject,
) (SearchUserOrganizationsResponseObject, error) {
	includePermissions := functional.FromPtr(request.Params.Include).
		Exists(func(include []UserOrganizationInclude) bool {
			return slices.Contains(include, UserOrganizationIncludeRolePermissions)
		})

	body := request.Body
	if body == nil {
		body = &SearchUserOrganizationsJSONRequestBody{}
	}

	if err := validateUserOrganizationsSearch(
		request.ProductId,
		request.ProductUserId,
		includePermissions,
		body,
	); err != nil {
		return nil, err
	}
	q := userOrganizationSearch(request.ProductId, request.ProductUserId, body)
	result, err := s.ProductUserService.SearchUserOrganizations(ctx, q, includePermissions)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search user organizations",
			slog.String("product_id", request.ProductId),
			slog.String("product_user_id", request.ProductUserId),
		)
		return nil, err
	}

	return SearchUserOrganizations200JSONResponse{
		Items: functional.Slice(result.Items).Map(
			func(m user.OrganizationMembership) UserOrganizationResponse {
				return mapUserOrgMembershipToResponse(m, includePermissions)
			}),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}, nil
}

// userOrganizationSearch builds the search from the request body. The product
// scope comes from the route, which the authorization middleware checked against
// the principal, and the route names the product user; the body never sets
// them. full_text_search is accepted and ignored: a membership has no text
// column that the contract names.
func userOrganizationSearch(
	productID, productUserID string, body *SearchUserOrganizationsJSONRequestBody,
) repository.UserOrganizationSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := repository.NewUserOrganizationSearch(repository.UserOrganizationScope{
		ProductID: productID, ProductUserID: productUserID,
	})
	q = searchexec.Some(q, q.OrganizationID().In, f.OrganizationIds)
	q = searchexec.Some(q, q.ProductRoleID().In, f.RoleIds)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.UserOrganizationSortJoinedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
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
		log.ErrorTo(ctx, s.logger, err, "failed to get user organization",
			slog.String("product_id", request.ProductId),
			slog.String("product_user_id", request.ProductUserId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	resp := mapUserOrgMembershipToResponse(membership, includePermissions)
	return GetUserOrganization200JSONResponse(resp), nil
}

func mapUserOrgMembershipToResponse(
	m user.OrganizationMembership, includePermissions bool,
) UserOrganizationResponse {
	var role *UserOrganizationRoleResponse
	if m.Role != nil {
		role = &UserOrganizationRoleResponse{
			Id:          m.Role.ID,
			Name:        m.Role.Name,
			Permissions: includedRolePermissions(m.RolePermissions, includePermissions),
		}
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
