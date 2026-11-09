package api

import (
	"context"
	"log/slog"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"anchor/internal/domain/organization"
	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
)

func (s *AnchorAPI) AddOrganizationMember(
	ctx context.Context,
	request AddOrganizationMemberRequestObject,
) (AddOrganizationMemberResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	input := organization.AddMemberInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ProductUserID:  request.Body.ProductUserId,
		RoleID:         request.Body.RoleId,
	}

	membership, err := s.OrganizationMembershipService.AddMember(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to add organization member",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("product_user_id", request.Body.ProductUserId),
		)
		return nil, err
	}

	return AddOrganizationMember201JSONResponse(
		mapOrgMemberToResponse(membership, organization.WithoutRolePermissions),
	), nil
}

func (s *AnchorAPI) GetOrganizationMember(
	ctx context.Context, request GetOrganizationMemberRequestObject,
) (GetOrganizationMemberResponseObject, error) {
	includePermissions := functional.FromPtr(request.Params.Include).
		Exists(func(include []OrganizationMemberInclude) bool {
			return slices.Contains(include, OrganizationMemberIncludeRolePermissions)
		})

	input := organization.GetMemberInput{
		ProductID:          request.ProductId,
		OrganizationID:     request.OrganizationId,
		ProductUserID:      request.ProductUserId,
		IncludePermissions: includePermissions,
	}

	membership, err := s.OrganizationMembershipService.GetMember(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to get organization member",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("product_user_id", request.ProductUserId),
		)
		return nil, err
	}

	resp := mapOrgMemberToResponse(membership, includePermissions)
	return GetOrganizationMember200JSONResponse(resp), nil
}

func (s *AnchorAPI) UpdateOrganizationMemberRole(
	ctx context.Context,
	request UpdateOrganizationMemberRoleRequestObject,
) (UpdateOrganizationMemberRoleResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	roleID, err := requiredRoleID(request.Body.RoleId)
	if err != nil {
		return nil, err
	}

	input := organization.UpdateMemberRoleInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ProductUserID:  request.ProductUserId,
		RoleID:         roleID,
	}

	membership, err := s.OrganizationMembershipService.UpdateMemberRole(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to update organization member role",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("product_user_id", request.ProductUserId),
		)
		return nil, err
	}

	return UpdateOrganizationMemberRole200JSONResponse(
		mapOrgMemberToResponse(membership, organization.WithoutRolePermissions),
	), nil
}

func (s *AnchorAPI) RemoveOrganizationMember(
	ctx context.Context,
	request RemoveOrganizationMemberRequestObject,
) (RemoveOrganizationMemberResponseObject, error) {
	input := organization.RemoveMemberInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ProductUserID:  request.ProductUserId,
	}

	err := s.OrganizationMembershipService.RemoveMember(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to remove organization member",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("product_user_id", request.ProductUserId),
		)
		return nil, err
	}

	return RemoveOrganizationMember204Response{}, nil
}

// organizationMemberSearch builds the search from the request body. The
// organization scope comes from the route, and the product scope from the route,
// which the authorization middleware checked against the principal; the body
// never sets them.
func organizationMemberSearch(
	productID, organizationID string, body *SearchOrganizationMembersJSONRequestBody,
) repository.OrganizationMemberSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	emails := functional.Slice(orEmpty(f.Emails)).Map(func(e openapi_types.Email) string { return string(e) })
	q := repository.NewOrganizationMemberSearch(repository.OrganizationMemberScope{
		OrganizationID: organizationID, ProductID: productID,
	})
	q = searchexec.Some(q, q.ProductUserID().In, orEmpty(f.ProductUserIds))
	q = searchexec.Some(q, q.ExternalID().In, orEmpty(f.ExternalIds))
	q = searchexec.Some(q, q.Email().In, emails)
	q = searchexec.Some(q, q.ProductRoleID().In, orEmpty(f.RoleIds))
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.OrganizationMemberSortJoinedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}

func (s *AnchorAPI) SearchOrganizationMembers(
	ctx context.Context,
	request SearchOrganizationMembersRequestObject,
) (SearchOrganizationMembersResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	if err := validateOrganizationMembersSearch(request); err != nil {
		return nil, err
	}
	res, err := s.OrganizationMembershipService.SearchMembers(
		ctx, organizationMemberSearch(request.ProductId, request.OrganizationId, request.Body),
	)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search organization members",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	resp := OrganizationMemberListResponse{
		Items: functional.Slice(
			res.Items).Map(

			func(m organization.Membership) OrganizationMemberResponse {
				return mapOrgMemberToResponse(m, organization.WithoutRolePermissions)
			}),

		Total:      res.Total,
		NextCursor: nextCursor(res.NextCursor),
		Count:      len(res.Items),
	}

	return SearchOrganizationMembers200JSONResponse(resp), nil
}

func mapOrgMemberToResponse(m organization.Membership, includePermissions bool) OrganizationMemberResponse {
	var name *string
	if m.UserName != "" {
		name = &m.UserName
	}

	return OrganizationMemberResponse{
		Email:         openapi_types.Email(m.UserEmail),
		ExternalId:    m.UserExternalID,
		JoinedAt:      m.JoinedAt,
		Name:          name,
		ProductUserId: m.ProductUserID,
		Role:          memberRoleResponse(m, includePermissions),
	}
}

// memberRoleResponse is the member's role, or nil for a member who holds none.
func memberRoleResponse(m organization.Membership, includePermissions bool) *OrganizationMemberRoleResponse {
	if m.Role == nil {
		return nil
	}
	return &OrganizationMemberRoleResponse{
		Id:          m.Role.ID,
		Name:        m.Role.Name,
		Permissions: includedRolePermissions(m.RolePermissions, includePermissions),
	}
}

// includedRolePermissions keeps an omitted projection distinct from requested empty permissions.
func includedRolePermissions(names []string, include bool) *[]string {
	if !include {
		return nil
	}
	if names == nil {
		names = []string{}
	}
	return &names
}
