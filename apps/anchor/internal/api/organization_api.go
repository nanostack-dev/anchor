package api

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/organization"
	"anchor/internal/repository"
	"anchor/internal/security"
)

// mapMetadataToResponse decodes stored organization metadata for the API
// response. Metadata is written through a validating service path, so a value
// that no longer decodes is treated as absent rather than failing the read.
func mapMetadataToResponse(metadataJSON json.RawMessage) *Metadata {
	if len(metadataJSON) == 0 {
		return nil
	}

	var metadata Metadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil
	}

	return &metadata
}

// mapMetadataToInput converts a request-body metadata object into the plain map
// the service layer validates.
func mapMetadataToInput(metadata *Metadata) map[string]any {
	return functional.FromPtr(metadata).OrElse(nil)
}

func mapOrganizationToResponse(org organization.Organization) ProductOrganizationResponse {
	response := ProductOrganizationResponse{
		Id:          org.ID,
		ProductId:   org.ProductID,
		Name:        org.Name,
		Description: org.Description,
		Metadata:    mapMetadataToResponse(org.MetadataJSON),
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}
	response.License = functional.FromPtr(org.License).Map(mapOrganizationLicenseToResponse).ToPtr()
	return response
}

// resolveIncludes reads the tenant only when the caller named a related
// resource: an include is read tenant-scoped, an organization is not.
func resolveIncludes(
	ctx context.Context, include *OrganizationIncludeParameter,
) (string, []organization.Include, error) {
	if include == nil || len(*include) == 0 {
		return "", nil, nil
	}

	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return "", nil, err
	}
	return tenantID, *include, nil
}

func (s *AnchorAPI) CreateProductOrganization(
	ctx context.Context, request CreateProductOrganizationRequestObject,
) (CreateProductOrganizationResponseObject, error) {
	// Read only for a license: a template is addressed tenant-scoped, an
	// organization is not.
	var tenantID string
	var licenseTemplateID *string
	if request.Body.License != nil {
		resolved, err := security.GetTenantID(ctx)
		if err != nil {
			return nil, err
		}
		tenantID = resolved
		licenseTemplateID = &request.Body.License.TemplateId
	}

	if request.Body.FoundingMember != nil {
		input := organization.CreateOrganizationWithMemberInput{
			TenantID:          tenantID,
			ProductID:         request.ProductId,
			Name:              request.Body.Name,
			Description:       request.Body.Description,
			Metadata:          mapMetadataToInput(request.Body.Metadata),
			ProductUserID:     request.Body.FoundingMember.ProductUserId,
			RoleID:            request.Body.FoundingMember.RoleId,
			LicenseTemplateID: licenseTemplateID,
		}

		res, err := s.OrganizationService.CreateWithMember(ctx, input)
		if err != nil {
			log.ErrorTo(ctx, s.logger, err, "failed to create organization with founding member",
				slog.String("product_id", request.ProductId),
				slog.String("product_user_id", request.Body.FoundingMember.ProductUserId),
				slog.String(roleIDField, functional.FromPtr(request.Body.FoundingMember.RoleId).OrElse("")),
			)
			return nil, err
		}

		return CreateProductOrganization201JSONResponse(
			mapOrganizationToResponse(res.Organization),
		), nil
	}

	input := organization.CreateOrganizationInput{
		TenantID:          tenantID,
		ProductID:         request.ProductId,
		Name:              request.Body.Name,
		Description:       request.Body.Description,
		Metadata:          mapMetadataToInput(request.Body.Metadata),
		LicenseTemplateID: licenseTemplateID,
	}

	created, err := s.OrganizationService.Create(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to create organization")
		return nil, err
	}

	return CreateProductOrganization201JSONResponse(mapOrganizationToResponse(created)), nil
}

func (s *AnchorAPI) SearchProductOrganizations(
	ctx context.Context, request SearchProductOrganizationsRequestObject,
) (SearchProductOrganizationsResponseObject, error) {
	searchReqBody := request.Body
	if searchReqBody == nil {
		searchReqBody = &SearchProductOrganizationsJSONRequestBody{}
	}

	tenantID, includes, err := resolveIncludes(ctx, request.Params.Include)
	if err != nil {
		return nil, err
	}

	if err = validateProductOrganizationsSearch(tenantID, request.ProductId, includes, searchReqBody); err != nil {
		return nil, err
	}
	q := productOrganizationSearch(request.ProductId, searchReqBody)

	result, err := s.OrganizationService.Search(ctx, q, tenantID, includes)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search product organizations")
		return nil, err
	}

	return SearchProductOrganizations200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapOrganizationToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      result.Count,
	}, nil
}

func (s *AnchorAPI) GetProductOrganization(
	ctx context.Context, request GetProductOrganizationRequestObject,
) (GetProductOrganizationResponseObject, error) {
	tenantID, includes, err := resolveIncludes(ctx, request.Params.Include)
	if err != nil {
		return nil, err
	}

	org, err := s.OrganizationService.Find(ctx, organization.FindOrganizationInput{
		TenantID:       tenantID,
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Include:        includes,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to get organization",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	return GetProductOrganization200JSONResponse(
		mapOrganizationToResponse(org),
	), nil
}

func (s *AnchorAPI) DeleteProductOrganization(
	ctx context.Context, request DeleteProductOrganizationRequestObject,
) (DeleteProductOrganizationResponseObject, error) {
	err := s.OrganizationService.Delete(ctx, organization.DeleteOrganizationInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to delete organization",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	return DeleteProductOrganization204Response{}, nil
}

func (s *AnchorAPI) UpdateProductOrganization(
	ctx context.Context, request UpdateProductOrganizationRequestObject,
) (UpdateProductOrganizationResponseObject, error) {
	input := organization.UpdateOrganizationInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Name:           &request.Body.Name,
		Description:    request.Body.Description,
		Metadata:       mapMetadataToInput(request.Body.Metadata),
	}

	updatedOrganization, err := s.OrganizationService.Update(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to update organization")
		return nil, err
	}

	return UpdateProductOrganization200JSONResponse(mapOrganizationToResponse(updatedOrganization)), nil
}

// productOrganizationSearch builds the search from the request body. The
// product scope comes from the route, which the authorization middleware
// checked against the principal; the body never sets it.
func productOrganizationSearch(
	productID string, body *SearchProductOrganizationsJSONRequestBody,
) repository.OrganizationSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := repository.NewOrganizationSearch(productID)
	q = searchexec.Some(q, q.ID().In, f.Ids)
	q = searchexec.Some(q, q.Name().In, f.Names)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.OrganizationSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}
