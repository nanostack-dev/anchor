package api

import (
	"context"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/security"
	billing "anchor/internal/stripebilling/billing"
)

func stripeBillingError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, billing.ErrNotFound) {
		return fault.NotFound("STRIPE_BILLING_RESOURCE_NOT_FOUND", err.Error())
	}
	if errors.Is(err, billing.ErrInput) {
		return fault.BadRequest("STRIPE_BILLING_INVALID_REQUEST", err.Error())
	}
	if errors.Is(err, billing.ErrConflict) {
		return fault.Conflict("STRIPE_BILLING_CONFLICT", err.Error())
	}
	if _, ok := errors.AsType[*fault.Error](err); ok {
		return err
	}
	return fault.Internal(
		"STRIPE_BILLING_UNAVAILABLE",
		"Stripe billing could not complete the request. Check the integration connection and retry.",
	)
}

func (s *AnchorAPI) GetStripeBillingState(
	ctx context.Context,
	request GetStripeBillingStateRequestObject,
) (GetStripeBillingStateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.StripeBilling.ReadState(ctx, tenantID, request.ProductId)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return GetStripeBillingState200JSONResponse(result), nil
}

func (s *AnchorAPI) CreateStripeBillingPrice(
	ctx context.Context,
	request CreateStripeBillingPriceRequestObject,
) (CreateStripeBillingPriceResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fault.BadRequest("STRIPE_BILLING_BODY_REQUIRED", "A request body is required.")
	}
	var result billing.Price
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.CreatePrice(actionCtx, *request.Body)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return CreateStripeBillingPrice201JSONResponse(result), nil
}

func (s *AnchorAPI) ArchiveStripeBillingPrice(
	ctx context.Context,
	request ArchiveStripeBillingPriceRequestObject,
) (ArchiveStripeBillingPriceResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	var result billing.Price
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.ArchivePrice(actionCtx, request.PriceId)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return ArchiveStripeBillingPrice200JSONResponse(result), nil
}

func (s *AnchorAPI) UpdateStripeBillingSettings(
	ctx context.Context,
	request UpdateStripeBillingSettingsRequestObject,
) (UpdateStripeBillingSettingsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fault.BadRequest("STRIPE_BILLING_BODY_REQUIRED", "A request body is required.")
	}
	var result billing.Settings
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.UpdateSettings(actionCtx, *request.Body)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return UpdateStripeBillingSettings200JSONResponse(result), nil
}

func (s *AnchorAPI) CreateStripeBillingCheckout(
	ctx context.Context,
	request CreateStripeBillingCheckoutRequestObject,
) (CreateStripeBillingCheckoutResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fault.BadRequest("STRIPE_BILLING_BODY_REQUIRED", "A request body is required.")
	}
	var result billing.URLResponse
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.Checkout(actionCtx, request.OrganizationId, *request.Body)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return CreateStripeBillingCheckout200JSONResponse(result), nil
}

func (s *AnchorAPI) ChangeStripeBillingSubscription(
	ctx context.Context,
	request ChangeStripeBillingSubscriptionRequestObject,
) (ChangeStripeBillingSubscriptionResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fault.BadRequest("STRIPE_BILLING_BODY_REQUIRED", "A request body is required.")
	}
	var result billing.Organization
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.ChangeSubscription(actionCtx, request.OrganizationId, *request.Body)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return ChangeStripeBillingSubscription200JSONResponse(result), nil
}

func (s *AnchorAPI) SetStripeBillingCancellation(
	ctx context.Context,
	request SetStripeBillingCancellationRequestObject,
) (SetStripeBillingCancellationResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fault.BadRequest("STRIPE_BILLING_BODY_REQUIRED", "A request body is required.")
	}
	var result billing.Organization
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.SetCancellation(actionCtx, request.OrganizationId, request.Body.CancelAtPeriodEnd)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return SetStripeBillingCancellation200JSONResponse(result), nil
}

func (s *AnchorAPI) CreateStripeBillingPortal(
	ctx context.Context,
	request CreateStripeBillingPortalRequestObject,
) (CreateStripeBillingPortalResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	var result billing.URLResponse
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.Portal(actionCtx, request.OrganizationId)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return CreateStripeBillingPortal200JSONResponse(result), nil
}

func (s *AnchorAPI) SyncStripeBillingOrganization(
	ctx context.Context,
	request SyncStripeBillingOrganizationRequestObject,
) (SyncStripeBillingOrganizationResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	var result billing.Organization
	err = s.StripeBilling.WithService(
		ctx,
		tenantID,
		request.ProductId,
		func(actionCtx context.Context, svc *billing.Service) error {
			var actionErr error
			result, actionErr = svc.SyncOrganization(actionCtx, request.OrganizationId)
			return actionErr
		},
	)
	if err != nil {
		return nil, stripeBillingError(err)
	}
	return SyncStripeBillingOrganization200JSONResponse(result), nil
}

func (s *AnchorAPI) IngestStripeBillingWebhook(
	ctx context.Context,
	request IngestStripeBillingWebhookRequestObject,
) (IngestStripeBillingWebhookResponseObject, error) {
	payload, ok := webhookPayloadFromContext(ctx)
	if !ok {
		return nil, fault.BadRequest(
			"STRIPE_WEBHOOK_PAYLOAD_REQUIRED",
			"The original Stripe webhook payload is required.",
		)
	}
	signature := WebhookHeadersFromContext(ctx)["stripe-signature"]
	if signature == "" {
		return nil, fault.BadRequest("STRIPE_WEBHOOK_SIGNATURE_REQUIRED", "Stripe-Signature is required.")
	}
	if err := s.StripeBilling.IngestInternal(ctx, request.ProductId, payload, signature); err != nil {
		return nil, stripeBillingError(err)
	}
	return IngestStripeBillingWebhook200JSONResponse{Received: true}, nil
}
