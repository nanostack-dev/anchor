package api

import (
	"context"

	"anchor/internal/domain/license"
	"anchor/internal/security"
)

// ---------------------------------------------------------------------------
// License schema handlers
// ---------------------------------------------------------------------------
//
// A Product declares exactly one license schema, so every handler addresses it
// by product rather than by its own identifier.

func (s *AnchorAPI) CreateLicenseSchema(
	ctx context.Context, request CreateLicenseSchemaRequestObject,
) (CreateLicenseSchemaResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	b := request.Body
	in := license.CreateSchemaInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Fields:    mapFieldDeclarationsFromAPI(b.Fields),
	}
	if b.Description != nil {
		in.Description = *b.Description
	}

	schema, err := s.LicenseSchemaService.CreateSchema(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to create license schema")
		return nil, err
	}
	return CreateLicenseSchema201JSONResponse(mapLicenseSchemaToResponse(schema)), nil
}

func (s *AnchorAPI) GetLicenseSchema(
	ctx context.Context, request GetLicenseSchemaRequestObject,
) (GetLicenseSchemaResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	schema, err := s.LicenseSchemaService.GetSchema(ctx, license.GetSchemaInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to get license schema")
		return nil, err
	}
	if schema == nil {
		return GetLicenseSchema404JSONResponse{NotFoundJSONResponse(
			notFoundBody("LICENSE_SCHEMA_NOT_FOUND", "License Schema does not exist."),
		)}, nil
	}
	return GetLicenseSchema200JSONResponse(mapLicenseSchemaToResponse(*schema)), nil
}

func (s *AnchorAPI) UpdateLicenseSchema(
	ctx context.Context, request UpdateLicenseSchemaRequestObject,
) (UpdateLicenseSchemaResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	in := license.UpdateSchemaInput{
		TenantID:    tenantID,
		ProductID:   request.ProductId,
		Description: request.Body.Description,
	}
	// A nil Fields leaves the declaration alone; a supplied one replaces it,
	// so an omitted field is a removal rather than a no-op.
	if request.Body.Fields != nil {
		declared := mapFieldDeclarationsFromAPI(*request.Body.Fields)
		in.Fields = &declared
	}

	schema, err := s.LicenseSchemaService.UpdateSchema(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to update license schema")
		return nil, err
	}
	return UpdateLicenseSchema200JSONResponse(mapLicenseSchemaToResponse(schema)), nil
}

func (s *AnchorAPI) DeleteLicenseSchema(
	ctx context.Context, request DeleteLicenseSchemaRequestObject,
) (DeleteLicenseSchemaResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err = s.LicenseSchemaService.DeleteSchema(ctx, license.DeleteSchemaInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
	}); err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to delete license schema")
		return nil, err
	}
	return DeleteLicenseSchema204Response{}, nil
}
