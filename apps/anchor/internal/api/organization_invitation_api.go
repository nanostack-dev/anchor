package api

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"anchor/internal/domain/organizationinvitation"
	invitationrepo "anchor/internal/invitation/repository"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
)

func (s *AnchorAPI) CreateOrganizationInvitation(
	ctx context.Context, request CreateOrganizationInvitationRequestObject,
) (CreateOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	created, err := s.OrganizationInvitationService.Create(ctx, organizationinvitation.CreateInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Email:          string(request.Body.Email),
		RoleID:         request.Body.RoleId,
		ExpiresAt:      request.Body.ExpiresAt,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to create organization invitation",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	return CreateOrganizationInvitation201JSONResponse(mapInvitationToResponse(created)), nil
}

func (s *AnchorAPI) SearchOrganizationInvitations(
	ctx context.Context, request SearchOrganizationInvitationsRequestObject,
) (SearchOrganizationInvitationsResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	if err := validateOrganizationInvitationsSearch(
		request.ProductId,
		request.OrganizationId,
		*request.Body,
	); err != nil {
		return nil, err
	}
	q, statuses := organizationInvitationSearch(request.ProductId, request.OrganizationId, *request.Body)
	result, err := s.OrganizationInvitationService.Search(ctx, q, statuses)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search organization invitations",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	return SearchOrganizationInvitations200JSONResponse(OrganizationInvitationListResponse{
		Items:      functional.Slice(result.Items).Map(mapInvitationToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      len(result.Items),
	}), nil
}

func (s *AnchorAPI) GetOrganizationInvitation(
	ctx context.Context, request GetOrganizationInvitationRequestObject,
) (GetOrganizationInvitationResponseObject, error) {
	invitation, err := s.OrganizationInvitationService.Get(ctx, organizationinvitation.GetInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to get organization invitation",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("invitation_id", request.InvitationId),
		)
		return nil, err
	}

	return GetOrganizationInvitation200JSONResponse(mapInvitationToResponse(invitation)), nil
}

func (s *AnchorAPI) UpdateOrganizationInvitation(
	ctx context.Context, request UpdateOrganizationInvitationRequestObject,
) (UpdateOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	roleID, err := requiredRoleID(request.Body.RoleId)
	if err != nil {
		return nil, err
	}

	invitation, err := s.OrganizationInvitationService.Update(ctx, organizationinvitation.UpdateInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
		RoleID:         roleID,
		ExpiresAt:      request.Body.ExpiresAt,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to update organization invitation",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("invitation_id", request.InvitationId),
		)
		return nil, err
	}

	return UpdateOrganizationInvitation200JSONResponse(mapInvitationToResponse(invitation)), nil
}

func (s *AnchorAPI) DeleteOrganizationInvitation(
	ctx context.Context, request DeleteOrganizationInvitationRequestObject,
) (DeleteOrganizationInvitationResponseObject, error) {
	err := s.OrganizationInvitationService.Delete(ctx, organizationinvitation.DeleteInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to delete organization invitation",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("invitation_id", request.InvitationId),
		)
		return nil, err
	}

	return DeleteOrganizationInvitation204Response{}, nil
}

func (s *AnchorAPI) SearchProductOrganizationInvitations(
	ctx context.Context, request SearchProductOrganizationInvitationsRequestObject,
) (SearchProductOrganizationInvitationsResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	if err := validateProductInvitationsSearch(request.ProductId, *request.Body); err != nil {
		return nil, err
	}
	q, statuses := productInvitationSearch(request.ProductId, *request.Body)
	result, err := s.OrganizationInvitationService.SearchInProduct(ctx, q, statuses)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search product organization invitations",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	return SearchProductOrganizationInvitations200JSONResponse(OrganizationInvitationListResponse{
		Items:      functional.Slice(result.Items).Map(mapInvitationToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      len(result.Items),
	}), nil
}

func (s *AnchorAPI) AcceptOrganizationInvitation(
	ctx context.Context, request AcceptOrganizationInvitationRequestObject,
) (AcceptOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	invitation, err := s.OrganizationInvitationService.Accept(ctx, organizationinvitation.AcceptInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
		ProductUserID:  request.Body.ProductUserId,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to accept organization invitation",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
			slog.String("invitation_id", request.InvitationId),
			slog.String("product_user_id", request.Body.ProductUserId),
		)
		return nil, err
	}

	return AcceptOrganizationInvitation200JSONResponse(mapInvitationToResponse(invitation)), nil
}

// organizationInvitationSearch builds the search of one organization's
// invitations from the request body. The scope comes from the route, which the
// authorization middleware checked against the principal; the body never sets
// it. full_text_search is accepted and ignored: an invitation has no text column
// that the contract names. The statuses come back beside the search: the
// repository derives the status on its own clock, so it is not a column filter.
func organizationInvitationSearch(
	productID, organizationID string, body SearchOrganizationInvitationsJSONRequestBody,
) (invitationrepo.OrganizationInvitationSearch, []organizationinvitation.Status) {
	statuses, emails := invitationFilters(body)
	q := invitationrepo.NewOrganizationInvitationSearch(
		invitationrepo.OrganizationInvitationScope{ProductID: productID, OrganizationID: organizationID},
	)
	q = searchexec.Some(q, q.Email().In, emails)
	q = searchexec.APISort(q, q.SortBy, body.SortBy, body.SortDirection,
		invitationrepo.OrganizationInvitationSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, body.Pagination), statuses
}

// productInvitationSearch builds the search of every invitation of the product
// from the request body.
func productInvitationSearch(
	productID string, body SearchOrganizationInvitationsJSONRequestBody,
) (invitationrepo.ProductInvitationSearch, []organizationinvitation.Status) {
	statuses, emails := invitationFilters(body)
	q := invitationrepo.NewProductInvitationSearch(productID)
	q = searchexec.Some(q, q.Email().In, emails)
	q = searchexec.APISort(q, q.SortBy, body.SortBy, body.SortDirection,
		invitationrepo.ProductInvitationSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, body.Pagination), statuses
}

// invitationFilters reads the status and email filters of an invitation search.
func invitationFilters(body SearchOrganizationInvitationsJSONRequestBody) ([]organizationinvitation.Status, []string) {
	var statuses []organizationinvitation.Status
	var emails []string
	if body.Filter != nil {
		statuses = orEmpty(body.Filter.Statuses)
		emails = functional.Slice(orEmpty(body.Filter.Emails)).Map(
			func(email openapi_types.Email) string { return string(email) },
		)
	}
	return statuses, emails
}

func mapInvitationToResponse(invitation organizationinvitation.Invitation) OrganizationInvitationResponse {
	return OrganizationInvitationResponse{
		Id:             invitation.ID,
		OrganizationId: invitation.OrganizationID,
		Email:          openapi_types.Email(invitation.Email),
		RoleId:         invitation.RoleID,
		Status:         invitation.Status,
		ExpiresAt:      invitation.ExpiresAt,
		AcceptedAt:     invitation.AcceptedAt,
		CreatedAt:      invitation.CreatedAt,
		UpdatedAt:      invitation.UpdatedAt,
	}
}
