package api

import (
	"context"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/resource"
	"anchor/internal/domain/roleassignment"
	"anchor/internal/searchx"
	"anchor/internal/security"
	"anchor/internal/service"
)

func mapResourceMemberToResponse(m roleassignment.Member) ResourceMemberResponse {
	return ResourceMemberResponse{
		ProductUserId: m.ProductUserID,
		Roles: functional.Slice(m.Roles).Map(func(r roleassignment.Role) ResourceMemberRole {
			return ResourceMemberRole{Id: r.ID, Name: r.Name}
		}),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func resourceMemberTarget(
	productID, organizationID, resourceType, resourceID, productUserID string,
) roleassignment.Target {
	return roleassignment.Target{
		ProductID:      productID,
		OrganizationID: organizationID,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		ProductUserID:  productUserID,
	}
}

// resourceWriteScope is what the resource PUT demands. A member PUT that
// registers, renames or moves its resource demands it too.
const resourceWriteScope = "resource:write"

func (s *AnchorAPI) PutResourceMember(
	ctx context.Context, request PutResourceMemberRequestObject,
) (PutResourceMemberResponseObject, error) {
	input := roleassignment.PutInput{
		Target: resourceMemberTarget(
			request.ProductId, request.OrganizationId, request.ResourceType, request.ResourceId, request.ProductUserId,
		),
		RoleIDs: request.Body.RoleIds,
	}
	if request.Body.Resource == nil {
		stored, err := s.RoleAssignmentService.Put(ctx, input)
		if err != nil {
			return nil, err
		}
		return PutResourceMember200JSONResponse(mapResourceMemberToResponse(stored)), nil
	}
	if err := requireProductAPIKeyScope(ctx, resourceWriteScope); err != nil {
		return nil, err
	}
	block := request.Body.Resource
	stored, err := s.ResourceService.PutWithMember(ctx, input, block.Name,
		resource.Ref{Type: block.Parent.Type, ID: block.Parent.Id})
	if err != nil {
		return nil, err
	}
	return PutResourceMember200JSONResponse(mapResourceMemberToResponse(stored)), nil
}

// requireProductAPIKeyScope refuses a request whose product API key lacks a
// scope its body demands beyond the route's own.
func requireProductAPIKeyScope(ctx context.Context, scope string) error {
	caller, ok := security.GetProductAPIKeyCaller(ctx)
	if !ok {
		return fault.ErrUnauthorized
	}
	if slices.Contains(caller.Permissions, scope) {
		return nil
	}
	return service.NewProductAPIKeyInsufficientPermissionsError(caller.ID, []string{scope}, caller.Permissions)
}

func (s *AnchorAPI) GetResourceMember(
	ctx context.Context, request GetResourceMemberRequestObject,
) (GetResourceMemberResponseObject, error) {
	found, err := s.RoleAssignmentService.Get(ctx, resourceMemberTarget(
		request.ProductId, request.OrganizationId, request.ResourceType, request.ResourceId, request.ProductUserId,
	))
	if err != nil {
		return nil, err
	}
	return GetResourceMember200JSONResponse(mapResourceMemberToResponse(found)), nil
}

func (s *AnchorAPI) DeleteResourceMember(
	ctx context.Context, request DeleteResourceMemberRequestObject,
) (DeleteResourceMemberResponseObject, error) {
	if err := s.RoleAssignmentService.Delete(ctx, resourceMemberTarget(
		request.ProductId, request.OrganizationId, request.ResourceType, request.ResourceId, request.ProductUserId,
	)); err != nil {
		return nil, err
	}
	return DeleteResourceMember204Response{}, nil
}

func (s *AnchorAPI) SearchResourceMembers(
	ctx context.Context, request SearchResourceMembersRequestObject,
) (SearchResourceMembersResponseObject, error) {
	filter := functional.FromPtr(request.Body.Filter).
		Map(func(f ResourceMemberFilter) roleassignment.Filter {
			return roleassignment.Filter{ProductUserIDs: f.ProductUserIds, RoleIDs: f.RoleIds}
		}).
		ToPtr()
	sortBy, sortDirection := searchx.Sort(
		request.Body.SortBy, request.Body.SortDirection, roleassignment.SortFieldCreatedAt, search.SortAscending,
	)
	result, err := s.RoleAssignmentService.Search(ctx, roleassignment.SearchInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ResourceType:   request.ResourceType,
		ResourceID:     request.ResourceId,
		Request: search.NewRequest[roleassignment.Filter, roleassignment.SortField]().
			WithFilter(filter).
			WithSort(sortBy, sortDirection).
			WithPagination(request.Body.Pagination),
	})
	if err != nil {
		return nil, err
	}
	return SearchResourceMembers200JSONResponse(ResourceMemberListResponse{
		Items:      functional.Slice(result.Items).Map(mapResourceMemberToResponse),
		Count:      result.Count,
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
	}), nil
}
