package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"anchor/internal/domain/organization"
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
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("product_user_id", request.Body.ProductUserId).
			Msg("failed to add organization member")
		return nil, err
	}

	return AddOrganizationMember201JSONResponse(
		mapOrgMemberToResponse(membership, false),
	), nil
}

func (s *AnchorAPI) GetOrganizationMember(
	ctx context.Context, request GetOrganizationMemberRequestObject,
) (GetOrganizationMemberResponseObject, error) {
	includePermissions := false
	if request.Params.Include != nil {
		for _, inc := range *request.Params.Include {
			if string(inc) == string(OrganizationMemberIncludeRolePermissions) {
				includePermissions = true
				break
			}
		}
	}

	input := organization.GetMemberInput{
		ProductID:          request.ProductId,
		OrganizationID:     request.OrganizationId,
		ProductUserID:      request.ProductUserId,
		IncludePermissions: includePermissions,
	}

	membership, err := s.OrganizationMembershipService.GetMember(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("product_user_id", request.ProductUserId).
			Msg("failed to get organization member")
		return nil, err
	}

	if membership == nil {
		return nil, fault.NotFound(
			"MEMBER_NOT_FOUND",
			"Organization member not found",
		)
	}

	resp := mapOrgMemberToResponse(*membership, includePermissions)
	return GetOrganizationMember200JSONResponse(resp), nil
}

func (s *AnchorAPI) UpdateOrganizationMemberRole(
	ctx context.Context,
	request UpdateOrganizationMemberRoleRequestObject,
) (UpdateOrganizationMemberRoleResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	input := organization.UpdateMemberRoleInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ProductUserID:  request.ProductUserId,
		RoleID:         request.Body.RoleId,
	}

	membership, err := s.OrganizationMembershipService.UpdateMemberRole(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("product_user_id", request.ProductUserId).
			Msg("failed to update organization member role")
		return nil, err
	}

	return UpdateOrganizationMemberRole200JSONResponse(
		mapOrgMemberToResponse(membership, false),
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
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("product_user_id", request.ProductUserId).
			Msg("failed to remove organization member")
		return nil, err
	}

	return RemoveOrganizationMember204Response{}, nil
}

func mapSearchOrganizationMembersRequestToInput(
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
			for _, e := range *request.Body.Filter.Emails {
				filter.Emails = append(filter.Emails, string(e))
			}
		}
		if request.Body.Filter.RoleIds != nil {
			filter.RoleIDs = append(filter.RoleIDs, *request.Body.Filter.RoleIds...)
		}
	}

	return req.WithFilter(filter).
		WithSort(
			request.Body.SortBy,
			request.Body.SortDirection,
		).WithFullTextSearch(request.Body.FullTextSearch).WithPagination(
		request.Body.Pagination,
	)
}

func (s *AnchorAPI) SearchOrganizationMembers(
	ctx context.Context,
	request SearchOrganizationMembersRequestObject,
) (SearchOrganizationMembersResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	input := organization.SearchMembersInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Request:        mapSearchOrganizationMembersRequestToInput(request),
	}

	res, err := s.OrganizationMembershipService.SearchMembers(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to search organization members")
		return nil, err
	}

	resp := OrganizationMemberListResponse{
		Items: functional.Slice(
			res.Items).Map(

			func(m organization.Membership) OrganizationMemberResponse {
				return mapOrgMemberToResponse(m, false)
			}),

		Total: res.Total,
		Count: len(res.Items),
	}

	return SearchOrganizationMembers200JSONResponse(resp), nil
}

func mapOrgMemberToResponse(m organization.Membership, includePermissions bool) OrganizationMemberResponse {
	var permissions *[]string
	if includePermissions {
		perms := m.RolePermissions
		if perms == nil {
			perms = []string{}
		}
		permissions = &perms
	}

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
		Role: struct {
			Id          Ksuid     `json:"id"` //nolint:revive,staticcheck // matches generated struct field name
			Name        string    `json:"name"`
			Permissions *[]string `json:"permissions,omitempty"`
		}{
			Id:          m.RoleID,
			Name:        m.RoleName,
			Permissions: permissions,
		},
	}
}
