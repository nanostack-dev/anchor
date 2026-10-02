package anchorsdk

import (
	"context"
	"time"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Invitations is the facade for one organization's invitations. Obtain one from
// an [Org] handle:
//
//	created, err := c.Organization(orgID).Invitations().Create("alice@example.com", roleID).Do(ctx)
//
// Anchor stores the invitation and sends no email. When a person signs in, the
// Product finds their invitations with [ProductInvitations.Search] by their
// verified email addresses, and calls [Invitations.Accept].
type Invitations struct{ o *Org }

// Invitations returns the invitation facade for this organization.
func (o *Org) Invitations() Invitations { return Invitations{o: o} }

// Create starts building an invitation of an email address to the organization
// with a role. Anchor refuses it while a pending invitation exists for the
// email address, and when the email address belongs to a member.
func (i Invitations) Create(email, roleID string) *InvitationCreateBuilder {
	return &InvitationCreateBuilder{
		o: i.o,
		req: nanoclient.CreateOrganizationInvitationJSONRequestBody{
			Email:  openapi_types.Email(email),
			RoleId: roleID,
		},
	}
}

// List returns the first page of the organization's invitations, using Anchor's
// default page size. It is shorthand for Search().Do(ctx).
func (i Invitations) List(ctx context.Context) (*nanoclient.OrganizationInvitationListResponse, error) {
	return i.Search().Do(ctx)
}

// Search starts building an invitation query.
//
//	page, err := org.Invitations().Search().Statuses(nanoclient.Pending).Limit(20).Do(ctx)
func (i Invitations) Search() *InvitationSearch {
	return &InvitationSearch{c: i.o.c, o: i.o}
}

// Get returns one invitation by its ID.
func (i Invitations) Get(
	ctx context.Context,
	invitationID string,
) (*nanoclient.OrganizationInvitationResponse, error) {
	const op = "Invitations.Get"

	return retrying(ctx, i.o.c, func(ctx context.Context) (*nanoclient.OrganizationInvitationResponse, error) {
		resp, err := i.o.c.api.GetOrganizationInvitationWithResponse(
			ctx, i.o.c.productID, i.o.id, invitationID,
		)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Update changes the role and the expiry of a pending invitation that has not
// expired. The email address of an invitation never changes: delete the
// invitation and create a new one.
func (i Invitations) Update(
	ctx context.Context,
	invitationID, roleID string,
	expiresAt time.Time,
) (*nanoclient.OrganizationInvitationResponse, error) {
	const op = "Invitations.Update"

	body := nanoclient.UpdateOrganizationInvitationJSONRequestBody{RoleId: roleID, ExpiresAt: expiresAt}

	return retrying(ctx, i.o.c, func(ctx context.Context) (*nanoclient.OrganizationInvitationResponse, error) {
		resp, err := i.o.c.api.UpdateOrganizationInvitationWithResponse(
			ctx, i.o.c.productID, i.o.id, invitationID, body,
		)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Delete removes an invitation for good, so it can no longer be accepted.
// There is no revoke: deleting is how a Product withdraws an invitation.
func (i Invitations) Delete(ctx context.Context, invitationID string) error {
	const op = "Invitations.Delete"

	return i.o.c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := i.o.c.api.DeleteOrganizationInvitationWithResponse(
			ctx, i.o.c.productID, i.o.id, invitationID,
		)
		if err != nil {
			return transportError(op, err)
		}
		return expectSuccess(op, resp.StatusCode(), resp.Body)
	})
}

// Accept turns a pending invitation into a membership of the organization for
// an existing product user, with the invited role. Anchor never creates the
// product user and never compares its email address with the invitation email:
// the Product checks it against an email address it has verified first.
func (i Invitations) Accept(
	ctx context.Context,
	invitationID, productUserID string,
) (*nanoclient.OrganizationInvitationResponse, error) {
	const op = "Invitations.Accept"

	body := nanoclient.AcceptOrganizationInvitationJSONRequestBody{ProductUserId: productUserID}

	return retrying(ctx, i.o.c, func(ctx context.Context) (*nanoclient.OrganizationInvitationResponse, error) {
		resp, err := i.o.c.api.AcceptOrganizationInvitationWithResponse(
			ctx, i.o.c.productID, i.o.id, invitationID, body,
		)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// InvitationCreateBuilder accumulates an invitation to create. Setter methods
// chain; [InvitationCreateBuilder.Do] sends. A builder is single-use and not
// safe for concurrent mutation.
type InvitationCreateBuilder struct {
	o   *Org
	req nanoclient.CreateOrganizationInvitationJSONRequestBody
}

// ExpiresAt sets when the invitation expires. It must be in the future. Left
// unset, Anchor expires the invitation 7 days from now.
func (b *InvitationCreateBuilder) ExpiresAt(expiresAt time.Time) *InvitationCreateBuilder {
	b.req.ExpiresAt = new(expiresAt)
	return b
}

// Do creates the invitation.
func (b *InvitationCreateBuilder) Do(
	ctx context.Context,
) (*nanoclient.OrganizationInvitationResponse, error) {
	const op = "Invitations.Create"

	c := b.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationInvitationResponse, error) {
		resp, err := c.api.CreateOrganizationInvitationWithResponse(ctx, c.productID, b.o.id, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}

// InvitationSearch accumulates an invitation query over one organization, or
// over every organization of the product when it comes from
// [ProductInvitations.Search]. Setter methods chain; [InvitationSearch.Do] runs
// it.
type InvitationSearch struct {
	c   *Client
	o   *Org
	req nanoclient.OrganizationInvitationSearchRequest
}

// Statuses restricts the result to invitations holding one of the statuses. A
// pending invitation whose expiry has passed counts as expired.
func (s *InvitationSearch) Statuses(statuses ...nanoclient.OrganizationInvitationStatus) *InvitationSearch {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.OrganizationInvitationFilter{}
	}
	s.req.Filter.Statuses = new(statuses)
	return s
}

// Emails restricts the result to invitations addressed to one of the email
// addresses, compared without regard to letter case.
func (s *InvitationSearch) Emails(emails ...string) *InvitationSearch {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.OrganizationInvitationFilter{}
	}
	typed := make([]openapi_types.Email, 0, len(emails))
	for _, email := range emails {
		typed = append(typed, openapi_types.Email(email))
	}
	s.req.Filter.Emails = new(typed)
	return s
}

// SortBy orders the result by one of Anchor's supported fields, for example
// [nanoclient.OrganizationInvitationSearchRequestSortByExpiresAt].
func (s *InvitationSearch) SortBy(
	field nanoclient.OrganizationInvitationSearchRequestSortBy,
	direction nanoclient.SortDirection,
) *InvitationSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of invitations returned.
func (s *InvitationSearch) Limit(limit int32) *InvitationSearch {
	s.page().Limit = new(limit)
	return s
}

// Offset skips the given number of invitations.
func (s *InvitationSearch) Offset(offset int32) *InvitationSearch {
	s.page().Offset = new(offset)
	return s
}

// Do runs the search.
func (s *InvitationSearch) Do(ctx context.Context) (*nanoclient.OrganizationInvitationListResponse, error) {
	const op = "Invitations.Search"

	c := s.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationInvitationListResponse, error) {
		if s.o == nil {
			resp, err := c.api.SearchProductOrganizationInvitationsWithResponse(ctx, c.productID, s.req)
			if err != nil {
				return nil, transportError(op, err)
			}
			return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
		}
		resp, err := c.api.SearchOrganizationInvitationsWithResponse(ctx, c.productID, s.o.id, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *InvitationSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}

// ProductInvitations is the facade for the invitations of every organization of
// this client's product. Obtain one with [Client.Invitations].
type ProductInvitations struct{ c *Client }

// Invitations returns the product-wide invitation facade.
func (c *Client) Invitations() ProductInvitations { return ProductInvitations{c: c} }

// Search starts building a query over the invitations of every organization of
// the product. A Product runs it when a person signs in, with the verified
// email addresses of that person:
//
//	page, err := c.Invitations().Search().Emails(verified...).Statuses(nanoclient.Pending).Do(ctx)
func (p ProductInvitations) Search() *InvitationSearch {
	return &InvitationSearch{c: p.c}
}
