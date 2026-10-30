package anchorsdk

import (
	"context"
	"iter"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// ResourceMembers is the facade for the roles organization members hold on one
// resource. Obtain one from a [Resource] handle:
//
//	_, err := org.Resource("workspace", workspaceID).Members().Assign(ctx, productUserID, roleID)
//
// Members are named by their product user ID (pusr_...), not by the external ID
// access checks use.
type ResourceMembers struct{ r *Resource }

// Members returns the facade for the roles held on this resource.
func (r *Resource) Members() ResourceMembers { return ResourceMembers{r: r} }

// Assign gives the member this role on the resource, replacing the role they
// held there before. The member must belong to the organization, and the role
// must be scoped to the resource's type. The same call again changes nothing,
// so it is retried on transient failures.
func (m ResourceMembers) Assign(
	ctx context.Context,
	productUserID, roleID string,
) (*nanoclient.ResourceMemberResponse, error) {
	const op = "ResourceMembers.Assign"

	c, r := m.r.o.c, m.r
	body := nanoclient.ResourceMemberPutRequest{RoleIds: []string{roleID}}

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ResourceMemberResponse, error) {
		resp, err := c.api.PutResourceMemberWithResponse(ctx, c.productID, r.o.id, r.typ, r.id, productUserID, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Get returns the roles one member holds on the resource. A member who holds
// none answers [ErrNotFound].
func (m ResourceMembers) Get(ctx context.Context, productUserID string) (*nanoclient.ResourceMemberResponse, error) {
	const op = "ResourceMembers.Get"

	c, r := m.r.o.c, m.r

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ResourceMemberResponse, error) {
		resp, err := c.api.GetResourceMemberWithResponse(ctx, c.productID, r.o.id, r.typ, r.id, productUserID)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Remove takes away every role the member holds on the resource. Their
// organization role stays. A retried remove whose first attempt succeeded
// answers success.
func (m ResourceMembers) Remove(ctx context.Context, productUserID string) error {
	const op = "ResourceMembers.Remove"

	c, r := m.r.o.c, m.r

	retried := false
	return c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := c.api.DeleteResourceMemberWithResponse(ctx, c.productID, r.o.id, r.typ, r.id, productUserID)
		if err != nil {
			retried = true
			return transportError(op, err)
		}
		return expectGone(op, resp.StatusCode(), resp.Body, &retried)
	})
}

// Search starts building a query over the members holding roles on the resource.
func (m ResourceMembers) Search() *ResourceMemberSearch { return &ResourceMemberSearch{r: m.r} }

// ResourceMemberSearch accumulates a resource member query. Setter methods
// chain; [ResourceMemberSearch.Do] runs it.
type ResourceMemberSearch struct {
	r   *Resource
	req nanoclient.ResourceMemberSearchRequest
}

// ProductUserIDs keeps these members.
func (s *ResourceMemberSearch) ProductUserIDs(productUserIDs ...string) *ResourceMemberSearch {
	s.filter().ProductUserIds = productUserIDs
	return s
}

// RoleIDs keeps members holding one of these roles on the resource.
func (s *ResourceMemberSearch) RoleIDs(roleIDs ...string) *ResourceMemberSearch {
	s.filter().RoleIds = roleIDs
	return s
}

// SortBy orders the result, for example by [nanoclient.ResourceMemberSortFieldCreatedAt].
func (s *ResourceMemberSearch) SortBy(
	field nanoclient.ResourceMemberSortField,
	direction nanoclient.SortDirection,
) *ResourceMemberSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of members on a page.
func (s *ResourceMemberSearch) Limit(limit int32) *ResourceMemberSearch {
	s.page().Limit = new(limit)
	return s
}

// Cursor continues after the last item of a previous page. To read every page,
// use [ResourceMemberSearch.All].
func (s *ResourceMemberSearch) Cursor(cursor string) *ResourceMemberSearch {
	s.page().Cursor = new(cursor)
	return s
}

// All reads every matching member, following NextCursor from page to page. The
// builder is not changed. A failed page ends the sequence with its error.
func (s *ResourceMemberSearch) All(ctx context.Context) iter.Seq2[nanoclient.ResourceMemberResponse, error] {
	return allPages(ctx, "ResourceMembers.Search", startCursor(s.req.Pagination),
		func(ctx context.Context, cursor string) ([]nanoclient.ResourceMemberResponse, string, error) {
			next := *s
			next.req.Pagination = pageAfter(s.req.Pagination, cursor)
			page, err := next.Do(ctx)
			if err != nil {
				return nil, "", err
			}
			return page.Items, derefCursor(page.NextCursor), nil
		})
}

// Do runs the search and returns one page.
func (s *ResourceMemberSearch) Do(ctx context.Context) (*nanoclient.ResourceMemberListResponse, error) {
	const op = "ResourceMembers.Search"

	c, r := s.r.o.c, s.r

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ResourceMemberListResponse, error) {
		resp, err := c.api.SearchResourceMembersWithResponse(ctx, c.productID, r.o.id, r.typ, r.id, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *ResourceMemberSearch) filter() *nanoclient.ResourceMemberFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.ResourceMemberFilter{}
	}
	return s.req.Filter
}

func (s *ResourceMemberSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}
