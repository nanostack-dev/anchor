package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/resourcetype"
	rtrepo "anchor/internal/resourcetype/repository"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
)

func mapResourceTypeToResponse(rt resourcetype.ResourceType) ProductResourceTypeResponse {
	return ProductResourceTypeResponse{
		Id:                rt.ID,
		ProductId:         rt.ProductID,
		Slug:              rt.Slug,
		Name:              rt.Name,
		Description:       rt.Description,
		ParentTypes:       append([]ResourceTypeSlug{}, rt.ParentTypes...),
		NestsInItself:     rt.NestsInItself,
		AllowsCustomRoles: rt.AllowsCustomRoles,
		BuiltIn:           rt.BuiltIn(),
		CreatedAt:         rt.CreatedAt,
		UpdatedAt:         rt.UpdatedAt,
	}
}

func (s *AnchorAPI) CreateProductResourceType(
	ctx context.Context, request CreateProductResourceTypeRequestObject,
) (CreateProductResourceTypeResponseObject, error) {
	created, err := s.ResourceTypeService.Create(ctx, resourcetype.CreateInput{
		ProductID:     request.ProductId,
		Slug:          request.Body.Slug,
		Name:          request.Body.Name,
		Description:   request.Body.Description,
		ParentTypes:   request.Body.ParentTypes,
		NestsInItself: functional.FromPtr(request.Body.NestsInItself).OrElse(false),
	})
	if err != nil {
		return nil, err
	}
	return CreateProductResourceType201JSONResponse(mapResourceTypeToResponse(created)), nil
}

func (s *AnchorAPI) ReplaceProductResourceTypes(
	ctx context.Context, request ReplaceProductResourceTypesRequestObject,
) (ReplaceProductResourceTypesResponseObject, error) {
	types, err := s.ResourceTypeService.ReplaceTree(ctx, resourcetype.ReplaceTreeInput{
		ProductID: request.ProductId,
		Types: functional.Slice(request.Body.ResourceTypes).Map(
			func(item ProductResourceTypeTreeItem) resourcetype.TreeItem {
				return resourcetype.TreeItem{
					Slug:          item.Slug,
					Name:          item.Name,
					Description:   item.Description,
					ParentTypes:   item.ParentTypes,
					NestsInItself: functional.FromPtr(item.NestsInItself).OrElse(false),
				}
			}),
	})
	if err != nil {
		return nil, err
	}
	return ReplaceProductResourceTypes200JSONResponse(ProductResourceTypeTreeResponse{
		ResourceTypes: functional.Slice(types).Map(mapResourceTypeToResponse),
	}), nil
}

func (s *AnchorAPI) SearchProductResourceTypes(
	ctx context.Context, request SearchProductResourceTypesRequestObject,
) (SearchProductResourceTypesResponseObject, error) {
	result, err := s.ResourceTypeService.Search(ctx, resourceTypeSearch(request.ProductId, request.Body))
	if err != nil {
		return nil, err
	}
	return SearchProductResourceTypes200JSONResponse(ProductResourceTypeListResponse{
		Items:      functional.Slice(result.Items).Map(mapResourceTypeToResponse),
		Count:      result.Count,
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
	}), nil
}

func (s *AnchorAPI) GetProductResourceType(
	ctx context.Context, request GetProductResourceTypeRequestObject,
) (GetProductResourceTypeResponseObject, error) {
	found, err := s.ResourceTypeService.Get(ctx, request.ProductId, request.ResourceTypeId)
	if err != nil {
		return nil, err
	}
	return GetProductResourceType200JSONResponse(mapResourceTypeToResponse(found)), nil
}

func (s *AnchorAPI) UpdateProductResourceType(
	ctx context.Context, request UpdateProductResourceTypeRequestObject,
) (UpdateProductResourceTypeResponseObject, error) {
	updated, err := s.ResourceTypeService.Update(ctx, resourcetype.UpdateInput{
		ProductID:         request.ProductId,
		ID:                request.ResourceTypeId,
		Name:              request.Body.Name,
		Description:       request.Body.Description,
		ParentTypes:       functional.FromPtr(request.Body.ParentTypes).OrElse(nil),
		NestsInItself:     request.Body.NestsInItself,
		AllowsCustomRoles: request.Body.AllowsCustomRoles,
	})
	if err != nil {
		return nil, err
	}
	return UpdateProductResourceType200JSONResponse(mapResourceTypeToResponse(updated)), nil
}

func (s *AnchorAPI) DeleteProductResourceType(
	ctx context.Context, request DeleteProductResourceTypeRequestObject,
) (DeleteProductResourceTypeResponseObject, error) {
	if err := s.ResourceTypeService.Delete(ctx, request.ProductId, request.ResourceTypeId); err != nil {
		return nil, err
	}
	return DeleteProductResourceType204Response{}, nil
}

// resourceTypeSearch builds the search from the request body. The product
// scope comes from the route, which the authorization middleware checked
// against the principal; the body never sets it.
func resourceTypeSearch(
	productID string, body *SearchProductResourceTypesJSONRequestBody,
) rtrepo.ResourceTypeSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := rtrepo.NewResourceTypeSearch(productID)
	q = searchexec.Some(q, q.ID().In, f.Ids)
	q = searchexec.Some(q, q.Slug().In, f.Slugs)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		rtrepo.ResourceTypeSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}
