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
// Anchor stores the invitation. Under Product delivery, the default, it sends no
// email: the token in the response of [InvitationCreateBuilder.Do] and
// [Invitations.Resend] is the only time the caller sees it, so deliver it to the
// invited person, who gives it back to the Product, which calls
// [ProductInvitations.Accept]. Under Anchor delivery, chosen in the invitation
// settings by a Platform User, Anchor also sends the email during those two
// calls, and a failed send fails the call. The token is returned in both modes.
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
	return &InvitationSearch{o: i.o}
}

// Get returns one invitation by its ID. The response never carries the token.
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

// Delete removes an invitation for good. Its token stops working. There is no
// revoke: deleting is how a Product withdraws an invitation.
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

// Resend replaces the token of a pending invitation that has not expired,
// resets its expiry to Anchor's default, and returns the new token. The old
// token stops working. Anchor refuses it on an accepted or an expired
// invitation.
func (i Invitations) Resend(
	ctx context.Context,
	invitationID string,
) (*nanoclient.CreatedOrganizationInvitationResponse, error) {
	const op = "Invitations.Resend"

	return retrying(ctx, i.o.c, func(ctx context.Context) (*nanoclient.CreatedOrganizationInvitationResponse, error) {
		resp, err := i.o.c.api.ResendOrganizationInvitationWithResponse(
			ctx, i.o.c.productID, i.o.id, invitationID,
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

// Do creates the invitation and returns it with its token.
func (b *InvitationCreateBuilder) Do(
	ctx context.Context,
) (*nanoclient.CreatedOrganizationInvitationResponse, error) {
	const op = "Invitations.Create"

	c := b.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.CreatedOrganizationInvitationResponse, error) {
		resp, err := c.api.CreateOrganizationInvitationWithResponse(ctx, c.productID, b.o.id, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}

// InvitationSearch accumulates an invitation query. Setter methods chain;
// [InvitationSearch.Do] runs it.
type InvitationSearch struct {
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

	c := s.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationInvitationListResponse, error) {
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

// ProductInvitations is the facade for the invitation calls that start from a
// token instead of an organization. Obtain one with [Client.Invitations]. The
// token travels in the request body, never in the URL.
type ProductInvitations struct{ c *Client }

// Invitations returns the token-level invitation facade for this client's
// product.
func (c *Client) Invitations() ProductInvitations { return ProductInvitations{c: c} }

// Lookup finds the invitation a token belongs to, so the Product can compare
// the email address of the signed-in person with the invitation email before
// it calls [ProductInvitations.Accept]. An expired or accepted invitation is
// found too: read its status.
func (p ProductInvitations) Lookup(
	ctx context.Context,
	token string,
) (*nanoclient.OrganizationInvitationResponse, error) {
	const op = "Invitations.Lookup"

	body := nanoclient.LookupOrganizationInvitationJSONRequestBody{Token: token}

	return retrying(ctx, p.c, func(ctx context.Context) (*nanoclient.OrganizationInvitationResponse, error) {
		resp, err := p.c.api.LookupOrganizationInvitationWithResponse(ctx, p.c.productID, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Accept turns a pending invitation into a membership of its organization for
// an existing product user, with the invited role. Anchor never creates the
// product user and never compares its email address with the invitation email:
// the Product makes that check first.
func (p ProductInvitations) Accept(
	ctx context.Context,
	token, productUserID string,
) (*nanoclient.OrganizationInvitationResponse, error) {
	const op = "Invitations.Accept"

	body := nanoclient.AcceptOrganizationInvitationJSONRequestBody{Token: token, ProductUserId: productUserID}

	return retrying(ctx, p.c, func(ctx context.Context) (*nanoclient.OrganizationInvitationResponse, error) {
		resp, err := p.c.api.AcceptOrganizationInvitationWithResponse(ctx, p.c.productID, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}
