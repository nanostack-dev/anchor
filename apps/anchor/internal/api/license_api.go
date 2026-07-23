package api

import (
	"context"
	"math"

	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/domain/license"
	"anchor/internal/domain/plan"
)

// Entitlement mapping helpers.

func mapEntitlementsToDomain(entitlements *EntitlementsMap) plan.Entitlements {
	if entitlements == nil {
		return plan.Entitlements{}
	}

	domain := make(plan.Entitlements, len(*entitlements))
	for key, value := range *entitlements {
		domain[key] = plan.EntitlementValue{Type: value.Type, Value: value.Value}
	}

	return domain
}

func mapEntitlementsToResponse(entitlements plan.Entitlements) EntitlementsMap {
	response := make(EntitlementsMap, len(entitlements))
	for key, value := range entitlements {
		response[key] = EntitlementValue{Type: value.Type, Value: value.Value}
	}

	return response
}

// Plan handlers.

func mapPlanToResponse(p plan.Plan) PlanResponse {
	var description *string
	if p.Description != "" {
		description = &p.Description
	}

	return PlanResponse{
		Id:           p.ID,
		ProductId:    p.ProductID,
		Key:          p.Key,
		Name:         p.Name,
		Description:  description,
		Entitlements: mapEntitlementsToResponse(p.Entitlements),
		IsDefault:    p.IsDefault,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func (s *AnchorAPI) CreatePlan(
	ctx context.Context, request CreatePlanRequestObject,
) (CreatePlanResponseObject, error) {
	var description string
	if request.Body.Description != nil {
		description = *request.Body.Description
	}
	isDefault := false
	if request.Body.IsDefault != nil {
		isDefault = *request.Body.IsDefault
	}

	input := plan.CreatePlanInput{
		ProductID:    request.ProductId,
		Key:          request.Body.Key,
		Name:         request.Body.Name,
		Description:  description,
		Entitlements: mapEntitlementsToDomain(request.Body.Entitlements),
		IsDefault:    isDefault,
	}

	created, err := s.PlanService.Create(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to create plan")
		return nil, err
	}

	return CreatePlan201JSONResponse(mapPlanToResponse(created)), nil
}

func (s *AnchorAPI) ListPlans(
	ctx context.Context, request ListPlansRequestObject,
) (ListPlansResponseObject, error) {
	plans, err := s.PlanService.List(ctx, plan.ListPlansInput{
		ProductID: request.ProductId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to list plans")
		return nil, err
	}

	return ListPlans200JSONResponse{
		Items: slicex.Map(plans, mapPlanToResponse),
	}, nil
}

func (s *AnchorAPI) GetPlan(
	ctx context.Context, request GetPlanRequestObject,
) (GetPlanResponseObject, error) {
	found, err := s.PlanService.Get(ctx, plan.GetPlanInput{
		ProductID: request.ProductId,
		PlanID:    request.PlanId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("plan_id", request.PlanId).Msg("failed to get plan")
		return nil, err
	}
	if found == nil {
		return GetPlan404Response{}, nil
	}

	return GetPlan200JSONResponse(mapPlanToResponse(*found)), nil
}

func (s *AnchorAPI) UpdatePlan(
	ctx context.Context, request UpdatePlanRequestObject,
) (UpdatePlanResponseObject, error) {
	input := plan.UpdatePlanInput{
		ProductID:   request.ProductId,
		PlanID:      request.PlanId,
		Name:        request.Body.Name,
		Description: request.Body.Description,
		IsDefault:   request.Body.IsDefault,
	}
	if request.Body.Entitlements != nil {
		entitlements := mapEntitlementsToDomain(request.Body.Entitlements)
		input.Entitlements = &entitlements
	}

	updated, err := s.PlanService.Update(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str("plan_id", request.PlanId).Msg("failed to update plan")
		return nil, err
	}

	return UpdatePlan200JSONResponse(mapPlanToResponse(updated)), nil
}

func (s *AnchorAPI) DeletePlan(
	ctx context.Context, request DeletePlanRequestObject,
) (DeletePlanResponseObject, error) {
	err := s.PlanService.Delete(ctx, plan.DeletePlanInput{
		ProductID: request.ProductId,
		PlanID:    request.PlanId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("plan_id", request.PlanId).Msg("failed to delete plan")
		return nil, err
	}

	return DeletePlan204Response{}, nil
}

// License handlers.

func mapLicenseToResponse(l license.License) LicenseResponse {
	return LicenseResponse{
		Id:                     l.ID,
		ProductId:              l.ProductID,
		OrganizationId:         l.OrganizationID,
		PlanId:                 l.PlanID,
		Status:                 l.Status,
		ExpiresAt:              l.ExpiresAt,
		GraceUntil:             l.GraceUntil,
		EntitlementOverrides:   mapEntitlementsToResponse(l.EntitlementOverrides),
		RefreshIntervalSeconds: int(l.RefreshIntervalSeconds),
		CreatedAt:              l.CreatedAt,
		UpdatedAt:              l.UpdatedAt,
	}
}

func (s *AnchorAPI) ListLicenses(
	ctx context.Context, request ListLicensesRequestObject,
) (ListLicensesResponseObject, error) {
	licenses, err := s.LicenseService.List(ctx, license.ListLicensesInput{
		ProductID: request.ProductId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to list licenses")
		return nil, err
	}

	return ListLicenses200JSONResponse{
		Items: slicex.Map(licenses, mapLicenseToResponse),
	}, nil
}

func (s *AnchorAPI) GetOrganizationLicense(
	ctx context.Context, request GetOrganizationLicenseRequestObject,
) (GetOrganizationLicenseResponseObject, error) {
	found, err := s.LicenseService.Get(ctx, license.GetLicenseInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("organization_id", request.OrganizationId).
			Msg("failed to get organization license")
		return nil, err
	}
	if found == nil {
		return GetOrganizationLicense404Response{}, nil
	}

	return GetOrganizationLicense200JSONResponse(mapLicenseToResponse(*found)), nil
}

func (s *AnchorAPI) PutOrganizationLicense(
	ctx context.Context, request PutOrganizationLicenseRequestObject,
) (PutOrganizationLicenseResponseObject, error) {
	input := license.PutLicenseInput{
		ProductID:            request.ProductId,
		OrganizationID:       request.OrganizationId,
		PlanID:               request.Body.PlanId,
		Status:               request.Body.Status,
		ExpiresAt:            request.Body.ExpiresAt,
		GraceUntil:           request.Body.GraceUntil,
		EntitlementOverrides: mapEntitlementsToDomain(request.Body.EntitlementOverrides),
	}
	if request.Body.RefreshIntervalSeconds != nil {
		// Values outside int32 keep the out-of-range boundary so the service
		// validator (min=60, max=2592000) still answers them with a 400 rather
		// than the narrowing wrapping into a valid interval.
		interval := int32(math.MaxInt32)
		if v := *request.Body.RefreshIntervalSeconds; v >= math.MinInt32 && v <= math.MaxInt32 {
			interval = int32(v)
		}
		input.RefreshIntervalSeconds = &interval
	}

	result, err := s.LicenseService.Put(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("organization_id", request.OrganizationId).
			Msg("failed to put organization license")
		return nil, err
	}

	return PutOrganizationLicense200JSONResponse(mapLicenseToResponse(result)), nil
}

func (s *AnchorAPI) RevokeOrganizationLicense(
	ctx context.Context, request RevokeOrganizationLicenseRequestObject,
) (RevokeOrganizationLicenseResponseObject, error) {
	result, err := s.LicenseService.Revoke(ctx, license.RevokeLicenseInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("organization_id", request.OrganizationId).
			Msg("failed to revoke organization license")
		return nil, err
	}

	return RevokeOrganizationLicense200JSONResponse(mapLicenseToResponse(result)), nil
}

func (s *AnchorAPI) SuspendOrganizationLicense(
	ctx context.Context, request SuspendOrganizationLicenseRequestObject,
) (SuspendOrganizationLicenseResponseObject, error) {
	result, err := s.LicenseService.Suspend(ctx, license.SuspendLicenseInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("organization_id", request.OrganizationId).
			Msg("failed to suspend organization license")
		return nil, err
	}

	return SuspendOrganizationLicense200JSONResponse(mapLicenseToResponse(result)), nil
}

func (s *AnchorAPI) ReinstateOrganizationLicense(
	ctx context.Context, request ReinstateOrganizationLicenseRequestObject,
) (ReinstateOrganizationLicenseResponseObject, error) {
	result, err := s.LicenseService.Reinstate(ctx, license.ReinstateLicenseInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("organization_id", request.OrganizationId).
			Msg("failed to reinstate organization license")
		return nil, err
	}

	return ReinstateOrganizationLicense200JSONResponse(mapLicenseToResponse(result)), nil
}

// Entitlement read handler (Product API Key auth).

func (s *AnchorAPI) GetOrganizationEntitlements(
	ctx context.Context, request GetOrganizationEntitlementsRequestObject,
) (GetOrganizationEntitlementsResponseObject, error) {
	snapshot, err := s.LicenseService.GetEntitlements(ctx, license.GetEntitlementsInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("organization_id", request.OrganizationId).
			Msg("failed to resolve organization entitlements")
		return nil, err
	}

	return GetOrganizationEntitlements200JSONResponse{
		OrganizationId: snapshot.OrganizationID,
		ProductId:      snapshot.ProductID,
		PlanKey:        snapshot.PlanKey,
		Status:         snapshot.Status,
		Entitlements:   mapEntitlementsToResponse(snapshot.Entitlements),
		ExpiresAt:      snapshot.ExpiresAt,
		GraceUntil:     snapshot.GraceUntil,
		RefreshAfter:   snapshot.RefreshAfter,
	}, nil
}
