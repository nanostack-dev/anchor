package anchorsdk

import (
	"context"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Members is the facade for one organization's membership. Obtain one from an
// [Org] handle:
//
//	members, err := c.Organization(orgID).Members().List(ctx)
//
// A member is a product user granted a role inside the organization, so adding
// and removing members is also how a product user is attached to and detached
// from an organization — see [Users] for the users themselves.
type Members struct{ o *Org }

// Members returns the membership facade for this organization.
func (o *Org) Members() Members { return Members{o: o} }

// List returns the first page of the organization's members, using Anchor's
// default page size. It is shorthand for Search().Do(ctx).
func (m Members) List(ctx context.Context) (*nanoclient.OrganizationMemberListResponse, error) {
	return m.Search().Do(ctx)
}

// Search starts building a member query.
//
//	page, err := org.Members().Search().Query("alice").Limit(20).Do(ctx)
func (m Members) Search() *MemberSearch {
	return &MemberSearch{o: m.o}
}

// Get returns one member by product user ID.
func (m Members) Get(
	ctx context.Context,
	productUserID string,
) (*nanoclient.OrganizationMemberResponse, error) {
	return m.get(ctx, "Members.Get", productUserID, nil)
}

// GetWithRolePermissions returns one member with the permissions of their role
// resolved, saving a second call when the caller needs to authorize the member.
func (m Members) GetWithRolePermissions(
	ctx context.Context,
	productUserID string,
) (*nanoclient.OrganizationMemberResponse, error) {
	params := &nanoclient.GetOrganizationMemberParams{
		Include: new(nanoclient.OrganizationMemberIncludeRolePermissions),
	}
	return m.get(ctx, "Members.GetWithRolePermissions", productUserID, params)
}

// Add grants an existing product user the given role in the organization.
func (m Members) Add(
	ctx context.Context,
	productUserID, roleID string,
) (*nanoclient.OrganizationMemberResponse, error) {
	const op = "Members.Add"

	body := nanoclient.AddOrganizationMemberJSONRequestBody{ProductUserId: productUserID, RoleId: roleID}

	return retrying(ctx, m.o.c, func(ctx context.Context) (*nanoclient.OrganizationMemberResponse, error) {
		resp, err := m.o.c.api.AddOrganizationMemberWithResponse(ctx, m.o.c.productID, m.o.id, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}

// Remove revokes a product user's membership. The product user itself is left
// alone; delete it with [Users.Delete].
func (m Members) Remove(ctx context.Context, productUserID string) error {
	const op = "Members.Remove"

	return m.o.c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := m.o.c.api.RemoveOrganizationMemberWithResponse(ctx, m.o.c.productID, m.o.id, productUserID)
		if err != nil {
			return transportError(op, err)
		}
		return expectSuccess(op, resp.StatusCode(), resp.Body)
	})
}

// SetRole replaces a member's role and returns the membership as it now stands.
func (m Members) SetRole(
	ctx context.Context,
	productUserID, roleID string,
) (*nanoclient.OrganizationMemberResponse, error) {
	const op = "Members.SetRole"

	body := nanoclient.UpdateOrganizationMemberRoleJSONRequestBody{RoleId: roleID}

	return retrying(ctx, m.o.c, func(ctx context.Context) (*nanoclient.OrganizationMemberResponse, error) {
		resp, err := m.o.c.api.UpdateOrganizationMemberRoleWithResponse(
			ctx, m.o.c.productID, m.o.id, productUserID, body,
		)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// get is the shared body of Get and GetWithRolePermissions.
func (m Members) get(
	ctx context.Context,
	op, productUserID string,
	params *nanoclient.GetOrganizationMemberParams,
) (*nanoclient.OrganizationMemberResponse, error) {
	return retrying(ctx, m.o.c, func(ctx context.Context) (*nanoclient.OrganizationMemberResponse, error) {
		resp, err := m.o.c.api.GetOrganizationMemberWithResponse(
			ctx, m.o.c.productID, m.o.id, productUserID, params,
		)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// MemberSearch accumulates a member query. Setter methods chain;
// [MemberSearch.Do] runs it.
type MemberSearch struct {
	o   *Org
	req nanoclient.OrganizationMemberSearchRequest
}

// Query sets a full-text search term matched against the searchable fields.
func (s *MemberSearch) Query(term string) *MemberSearch {
	s.req.FullTextSearch = new(term)
	return s
}

// ProductUserIDs restricts the result to the given product users.
func (s *MemberSearch) ProductUserIDs(productUserIDs ...string) *MemberSearch {
	s.filter().ProductUserIds = new(productUserIDs)
	return s
}

// Emails restricts the result to members with the given email addresses.
func (s *MemberSearch) Emails(emails ...string) *MemberSearch {
	typed := make([]openapi_types.Email, 0, len(emails))
	for _, email := range emails {
		typed = append(typed, openapi_types.Email(email))
	}
	s.filter().Emails = new(typed)
	return s
}

// RoleIDs restricts the result to members holding one of the given roles.
func (s *MemberSearch) RoleIDs(roleIDs ...string) *MemberSearch {
	s.filter().RoleIds = new(roleIDs)
	return s
}

// SortBy orders the result by one of Anchor's supported fields, for example
// [nanoclient.OrganizationMemberSearchRequestSortByJoinedAt].
func (s *MemberSearch) SortBy(
	field nanoclient.OrganizationMemberSearchRequestSortBy,
	direction nanoclient.SortDirection,
) *MemberSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of members returned.
func (s *MemberSearch) Limit(limit int32) *MemberSearch {
	s.page().Limit = new(limit)
	return s
}

// Offset skips the given number of members.
func (s *MemberSearch) Offset(offset int32) *MemberSearch {
	s.page().Offset = new(offset)
	return s
}

// Do runs the search.
func (s *MemberSearch) Do(ctx context.Context) (*nanoclient.OrganizationMemberListResponse, error) {
	const op = "Members.Search"

	c := s.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationMemberListResponse, error) {
		resp, err := c.api.SearchOrganizationMembersWithResponse(ctx, c.productID, s.o.id, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *MemberSearch) filter() *nanoclient.OrganizationMemberFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.OrganizationMemberFilter{}
	}
	return s.req.Filter
}

func (s *MemberSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}
