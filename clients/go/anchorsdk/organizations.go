package anchorsdk

import (
	"context"
	"maps"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// Organizations is the facade for the product's organizations. Obtain one with
// [Client.Organizations].
//
// Everything scoped to a single organization — its members, workspaces, and API
// keys — hangs off the [Org] handle returned by [Client.Organization] instead,
// so the organization ID is bound once rather than repeated per call.
type Organizations struct{ c *Client }

// Organizations returns the organization facade for this client's product.
func (c *Client) Organizations() Organizations { return Organizations{c: c} }

// Create starts building an organization. The name must be unique within the
// product.
//
//	org, err := c.Organizations().Create("Acme").
//	    Description("Leading provider of innovative solutions").
//	    Meta("billing_ref", "cust_abc123").
//	    FoundingMember(userID, roleID).
//	    Do(ctx)
func (o Organizations) Create(name string) *OrgCreateBuilder {
	return &OrgCreateBuilder{c: o.c, req: nanoclient.ProductOrganizationRequest{Name: name}}
}

// Get returns one organization by ID.
func (o Organizations) Get(
	ctx context.Context,
	organizationID string,
) (*nanoclient.ProductOrganizationResponse, error) {
	const op = "Organizations.Get"

	return retrying(ctx, o.c, func(ctx context.Context) (*nanoclient.ProductOrganizationResponse, error) {
		resp, err := o.c.api.GetProductOrganizationWithResponse(ctx, o.c.productID, organizationID)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Update starts building a full replacement of the organization. Anchor treats
// the update as a PUT: fields the builder leaves unset are cleared.
func (o Organizations) Update(organizationID, name string) *OrgUpdateBuilder {
	return &OrgUpdateBuilder{
		c:     o.c,
		orgID: organizationID,
		req:   nanoclient.ProductOrganizationRequest{Name: name},
	}
}

// Delete removes an organization and everything scoped to it.
func (o Organizations) Delete(ctx context.Context, organizationID string) error {
	const op = "Organizations.Delete"

	return o.c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := o.c.api.DeleteProductOrganizationWithResponse(ctx, o.c.productID, organizationID)
		if err != nil {
			return transportError(op, err)
		}
		return expectSuccess(op, resp.StatusCode(), resp.Body)
	})
}

// Search starts building an organization query. With no criteria it returns the
// product's organizations, first page first.
//
//	page, err := c.Organizations().Search().Query("acme").Limit(20).Do(ctx)
func (o Organizations) Search() *OrgSearch {
	return &OrgSearch{c: o.c}
}

// List returns the first page of the product's organizations, using Anchor's
// default page size. It is shorthand for Search().Do(ctx).
func (o Organizations) List(ctx context.Context) (*nanoclient.ProductOrganizationListResponse, error) {
	return o.Search().Do(ctx)
}

// OrgCreateBuilder accumulates an organization to create. Setter methods chain;
// [OrgCreateBuilder.Do] sends. A builder is single-use and not safe for
// concurrent mutation.
type OrgCreateBuilder struct {
	c   *Client
	req nanoclient.ProductOrganizationRequest
}

// Description sets the organization's description.
func (b *OrgCreateBuilder) Description(description string) *OrgCreateBuilder {
	b.req.Description = new(description)
	return b
}

// Meta sets a single metadata entry, allocating the metadata map on first use.
func (b *OrgCreateBuilder) Meta(key string, value any) *OrgCreateBuilder {
	b.req.Metadata = withMeta(b.req.Metadata, key, value)
	return b
}

// Metadata merges every entry from m into the organization metadata. A nil or
// empty map is a no-op.
func (b *OrgCreateBuilder) Metadata(m map[string]any) *OrgCreateBuilder {
	b.req.Metadata = mergeMeta(b.req.Metadata, m)
	return b
}

// FoundingMember assigns a first member with the given role. Anchor creates the
// organization and the membership atomically in one transaction.
func (b *OrgCreateBuilder) FoundingMember(productUserID, roleID string) *OrgCreateBuilder {
	b.req.FoundingMember = &nanoclient.FoundingMemberRequest{ProductUserId: productUserID, RoleId: roleID}
	return b
}

// Do creates the organization.
func (b *OrgCreateBuilder) Do(ctx context.Context) (*nanoclient.ProductOrganizationResponse, error) {
	const op = "Organizations.Create"

	return retrying(ctx, b.c, func(ctx context.Context) (*nanoclient.ProductOrganizationResponse, error) {
		resp, err := b.c.api.CreateProductOrganizationWithResponse(ctx, b.c.productID, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}

// OrgUpdateBuilder accumulates an organization update. Setter methods chain;
// [OrgUpdateBuilder.Do] sends.
type OrgUpdateBuilder struct {
	c     *Client
	orgID string
	req   nanoclient.ProductOrganizationRequest
}

// Description sets the organization's description.
func (b *OrgUpdateBuilder) Description(description string) *OrgUpdateBuilder {
	b.req.Description = new(description)
	return b
}

// Meta sets a single metadata entry, allocating the metadata map on first use.
func (b *OrgUpdateBuilder) Meta(key string, value any) *OrgUpdateBuilder {
	b.req.Metadata = withMeta(b.req.Metadata, key, value)
	return b
}

// Metadata merges every entry from m into the organization metadata. A nil or
// empty map is a no-op.
func (b *OrgUpdateBuilder) Metadata(m map[string]any) *OrgUpdateBuilder {
	b.req.Metadata = mergeMeta(b.req.Metadata, m)
	return b
}

// Do applies the update and returns the organization as it now stands.
func (b *OrgUpdateBuilder) Do(ctx context.Context) (*nanoclient.ProductOrganizationResponse, error) {
	const op = "Organizations.Update"

	return retrying(ctx, b.c, func(ctx context.Context) (*nanoclient.ProductOrganizationResponse, error) {
		resp, err := b.c.api.UpdateProductOrganizationWithResponse(ctx, b.c.productID, b.orgID, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// OrgSearch accumulates an organization query. Setter methods chain;
// [OrgSearch.Do] runs it.
type OrgSearch struct {
	c   *Client
	req nanoclient.ProductOrganizationSearchRequest
}

// Query sets a full-text search term matched against the searchable fields.
func (s *OrgSearch) Query(term string) *OrgSearch {
	s.req.FullTextSearch = new(term)
	return s
}

// IDs restricts the result to the given organization IDs.
func (s *OrgSearch) IDs(organizationIDs ...string) *OrgSearch {
	s.filter().Ids = organizationIDs
	return s
}

// Names restricts the result to exact organization names.
func (s *OrgSearch) Names(names ...string) *OrgSearch {
	s.filter().Names = names
	return s
}

// SortBy orders the result by one of Anchor's supported fields, for example
// [nanoclient.ProductOrganizationSearchRequestSortByName].
func (s *OrgSearch) SortBy(
	field nanoclient.ProductOrganizationSearchRequestSortBy,
	direction nanoclient.SortDirection,
) *OrgSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of organizations returned.
func (s *OrgSearch) Limit(limit int32) *OrgSearch {
	s.page().Limit = new(limit)
	return s
}

// Offset skips the given number of organizations.
func (s *OrgSearch) Offset(offset int32) *OrgSearch {
	s.page().Offset = new(offset)
	return s
}

// Do runs the search.
func (s *OrgSearch) Do(ctx context.Context) (*nanoclient.ProductOrganizationListResponse, error) {
	const op = "Organizations.Search"

	return retrying(ctx, s.c, func(ctx context.Context) (*nanoclient.ProductOrganizationListResponse, error) {
		resp, err := s.c.api.SearchProductOrganizationsWithResponse(ctx, s.c.productID, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *OrgSearch) filter() *nanoclient.OrganizationFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.OrganizationFilter{}
	}
	return s.req.Filter
}

func (s *OrgSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}

// withMeta sets one entry on an optional metadata map, allocating it if needed.
func withMeta(m *nanoclient.Metadata, key string, value any) *nanoclient.Metadata {
	if m == nil {
		m = &nanoclient.Metadata{}
	}
	(*m)[key] = value
	return m
}

// mergeMeta merges src into an optional metadata map, allocating it if needed.
// An empty src leaves the map untouched, nil included.
func mergeMeta(m *nanoclient.Metadata, src map[string]any) *nanoclient.Metadata {
	if len(src) == 0 {
		return m
	}
	if m == nil {
		m = &nanoclient.Metadata{}
	}
	maps.Copy(*m, src)
	return m
}
