package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/resource"
	"anchor/internal/searchx"
)

func mapResourceToResponse(r resource.Resource) ResourceResponse {
	return ResourceResponse{
		Type:           r.Type,
		Id:             r.ExternalID,
		OrganizationId: r.OrganizationID,
		Name:           r.Name,
		Parent:         ResourceParentRef{Type: r.Parent.Type, Id: r.Parent.ID},
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func (s *AnchorAPI) PutResource(
	ctx context.Context, request PutResourceRequestObject,
) (PutResourceResponseObject, error) {
	stored, err := s.ResourceService.Put(ctx, resource.PutInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Type:           request.ResourceType,
		ID:             request.ResourceId,
		Name:           request.Body.Name,
		Parent:         resource.Ref{Type: request.Body.Parent.Type, ID: request.Body.Parent.Id},
	})
	if err != nil {
		return nil, err
	}
	return PutResource200JSONResponse(mapResourceToResponse(stored)), nil
}

func (s *AnchorAPI) GetResource(
	ctx context.Context, request GetResourceRequestObject,
) (GetResourceResponseObject, error) {
	found, err := s.ResourceService.Get(
		ctx, request.ProductId, request.OrganizationId, request.ResourceType, request.ResourceId,
	)
	if err != nil {
		return nil, err
	}
	return GetResource200JSONResponse(mapResourceToResponse(found)), nil
}

func (s *AnchorAPI) DeleteResource(
	ctx context.Context, request DeleteResourceRequestObject,
) (DeleteResourceResponseObject, error) {
	if err := s.ResourceService.Delete(
		ctx, request.ProductId, request.OrganizationId, request.ResourceType, request.ResourceId,
		functional.FromPtr(request.Params.Cascade).OrElse(false),
	); err != nil {
		return nil, err
	}
	return DeleteResource204Response{}, nil
}

func (s *AnchorAPI) SearchResources(
	ctx context.Context, request SearchResourcesRequestObject,
) (SearchResourcesResponseObject, error) {
	filter := functional.FromPtr(request.Body.Filter).
		Map(func(f ResourceFilter) resource.Filter {
			return resource.Filter{
				ResourceTypes: f.ResourceTypes,
				Parent: functional.FromPtr(f.Parent).Map(func(p ResourceParentRef) resource.Ref {
					return resource.Ref{Type: p.Type, ID: p.Id}
				}).ToPtr(),
				IDs:          f.Ids,
				NameContains: f.NameContains,
			}
		}).
		ToPtr()
	sortBy, sortDirection := searchx.Sort(
		request.Body.SortBy, request.Body.SortDirection, resource.SortFieldCreatedAt, search.SortAscending,
	)
	result, err := s.ResourceService.Search(ctx, resource.SearchInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Request: search.NewRequest[resource.Filter, resource.SortField]().
			WithFilter(filter).
			WithSort(sortBy, sortDirection).
			WithPagination(request.Body.Pagination),
	})
	if err != nil {
		return nil, err
	}
	return SearchResources200JSONResponse(ResourceListResponse{
		Items:      functional.Slice(result.Items).Map(mapResourceToResponse),
		Count:      result.Count,
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
	}), nil
}
