package anchorsdk

import (
	"context"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// Users is the facade for the product's users. Obtain one with [Client.Users]:
//
//	user, err := c.Users().Create("new.user@company.com").Name("New User").Do(ctx)
//
// A product user exists at product scope; membership of an organization is a
// separate grant. Attach a user to an organization with [Members.Add] and detach
// with [Members.Remove]; read the organizations a user belongs to with
// [Users.Organizations].
//
// Anchor exposes no update operation for a product user — change a user by
// recreating it, or reach the platform-user endpoints through [Client.Raw].
type Users struct{ c *Client }

// Users returns the product user facade for this client's product.
func (c *Client) Users() Users { return Users{c: c} }

// Create starts building a product user. The email must be unique within the
// product.
func (u Users) Create(email string) *UserCreateBuilder {
	return &UserCreateBuilder{c: u.c, req: nanoclient.CreateProductUserJSONRequestBody{Email: email}}
}

// Get returns one product user by ID.
func (u Users) Get(ctx context.Context, productUserID string) (*nanoclient.ProductUserResponse, error) {
	const op = "Users.Get"

	return retrying(ctx, u.c, func(ctx context.Context) (*nanoclient.ProductUserResponse, error) {
		resp, err := u.c.api.GetProductUserWithResponse(ctx, u.c.productID, productUserID)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Delete removes a product user and every organization membership it holds.
func (u Users) Delete(ctx context.Context, productUserID string) error {
	const op = "Users.Delete"

	return u.c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := u.c.api.DeleteProductUserWithResponse(ctx, u.c.productID, productUserID)
		if err != nil {
			return transportError(op, err)
		}
		return expectSuccess(op, resp.StatusCode(), resp.Body)
	})
}

// Search starts building a product user query.
//
//	page, err := c.Users().Search().Query("acme.com").Limit(50).Do(ctx)
func (u Users) Search() *UserSearch {
	return &UserSearch{c: u.c}
}

// List returns the first page of the product's users, using Anchor's default
// page size. It is shorthand for Search().Do(ctx).
func (u Users) List(ctx context.Context) (*nanoclient.ProductUserListResponse, error) {
	return u.Search().Do(ctx)
}

// Organizations returns every organization the product user belongs to, with the
// role held in each. The list is not paginated.
func (u Users) Organizations(
	ctx context.Context,
	productUserID string,
) (*nanoclient.UserOrganizationListResponse, error) {
	return u.organizations(ctx, "Users.Organizations", productUserID, nil)
}

// OrganizationsWithRolePermissions is [Users.Organizations] with each role's
// permissions resolved, saving a second call when the caller needs to authorize
// the user.
func (u Users) OrganizationsWithRolePermissions(
	ctx context.Context,
	productUserID string,
) (*nanoclient.UserOrganizationListResponse, error) {
	params := &nanoclient.ListUserOrganizationsParams{
		Include: new([]nanoclient.UserOrganizationInclude{nanoclient.UserOrganizationIncludeRolePermissions}),
	}
	return u.organizations(ctx, "Users.OrganizationsWithRolePermissions", productUserID, params)
}

// Organization returns one organization from the product user's perspective,
// including the role they hold in it. It is the membership check a product
// backend runs on an inbound request: a user who is not a member yields
// [ErrNotFound].
func (u Users) Organization(
	ctx context.Context,
	productUserID, organizationID string,
) (*nanoclient.UserOrganizationResponse, error) {
	return u.organization(ctx, "Users.Organization", productUserID, organizationID, nil)
}

// OrganizationWithRolePermissions is [Users.Organization] with the role's
// permissions resolved.
func (u Users) OrganizationWithRolePermissions(
	ctx context.Context,
	productUserID, organizationID string,
) (*nanoclient.UserOrganizationResponse, error) {
	params := &nanoclient.GetUserOrganizationParams{
		Include: new([]nanoclient.UserOrganizationInclude{nanoclient.UserOrganizationIncludeRolePermissions}),
	}
	return u.organization(ctx, "Users.OrganizationWithRolePermissions", productUserID, organizationID, params)
}

// organizations is the shared body of the list-organizations methods.
func (u Users) organizations(
	ctx context.Context,
	op, productUserID string,
	params *nanoclient.ListUserOrganizationsParams,
) (*nanoclient.UserOrganizationListResponse, error) {
	return retrying(ctx, u.c, func(ctx context.Context) (*nanoclient.UserOrganizationListResponse, error) {
		resp, err := u.c.api.ListUserOrganizationsWithResponse(ctx, u.c.productID, productUserID, params)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// organization is the shared body of the single-organization methods.
func (u Users) organization(
	ctx context.Context,
	op, productUserID, organizationID string,
	params *nanoclient.GetUserOrganizationParams,
) (*nanoclient.UserOrganizationResponse, error) {
	return retrying(ctx, u.c, func(ctx context.Context) (*nanoclient.UserOrganizationResponse, error) {
		resp, err := u.c.api.GetUserOrganizationWithResponse(
			ctx, u.c.productID, productUserID, organizationID, params,
		)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// UserCreateBuilder accumulates a product user to create. Setter methods chain;
// [UserCreateBuilder.Do] sends.
type UserCreateBuilder struct {
	c   *Client
	req nanoclient.CreateProductUserJSONRequestBody
}

// Name sets the user's display name.
func (b *UserCreateBuilder) Name(name string) *UserCreateBuilder {
	b.req.Name = new(name)
	return b
}

// Status sets the initial status. Without one Anchor applies its default.
func (b *UserCreateBuilder) Status(status nanoclient.ProductUserStatus) *UserCreateBuilder {
	b.req.Status = new(status)
	return b
}

// Do creates the product user.
func (b *UserCreateBuilder) Do(ctx context.Context) (*nanoclient.ProductUserResponse, error) {
	const op = "Users.Create"

	return retrying(ctx, b.c, func(ctx context.Context) (*nanoclient.ProductUserResponse, error) {
		resp, err := b.c.api.CreateProductUserWithResponse(ctx, b.c.productID, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}

// UserSearch accumulates a product user query. Setter methods chain;
// [UserSearch.Do] runs it.
type UserSearch struct {
	c   *Client
	req nanoclient.ProductUserSearchRequest
}

// Query sets a full-text search term matched against the searchable fields.
func (s *UserSearch) Query(term string) *UserSearch {
	s.req.FullTextSearch = new(term)
	return s
}

// IDs restricts the result to the given product user IDs.
func (s *UserSearch) IDs(productUserIDs ...string) *UserSearch {
	s.filter().Ids = new(productUserIDs)
	return s
}

// Emails restricts the result to exact email addresses.
func (s *UserSearch) Emails(emails ...string) *UserSearch {
	s.filter().Emails = emails
	return s
}

// ExternalIDs restricts the result to exact identity-provider user IDs.
func (s *UserSearch) ExternalIDs(externalIDs ...string) *UserSearch {
	s.filter().ExternalIds = externalIDs
	return s
}

// Names restricts the result to users whose name contains one of the substrings.
func (s *UserSearch) Names(names ...string) *UserSearch {
	s.filter().Names = new(names)
	return s
}

// Statuses restricts the result to users in one of the given states.
func (s *UserSearch) Statuses(statuses ...nanoclient.ProductUserStatus) *UserSearch {
	s.filter().Statuses = new(statuses)
	return s
}

// SortBy orders the result by one of Anchor's supported fields, for example
// [nanoclient.ProductUserSearchRequestSortByEmail].
func (s *UserSearch) SortBy(
	field nanoclient.ProductUserSearchRequestSortBy,
	direction nanoclient.SortDirection,
) *UserSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of users returned.
func (s *UserSearch) Limit(limit int32) *UserSearch {
	s.page().Limit = new(limit)
	return s
}

// Offset skips the given number of users.
func (s *UserSearch) Offset(offset int32) *UserSearch {
	s.page().Offset = new(offset)
	return s
}

// Do runs the search.
func (s *UserSearch) Do(ctx context.Context) (*nanoclient.ProductUserListResponse, error) {
	const op = "Users.Search"

	return retrying(ctx, s.c, func(ctx context.Context) (*nanoclient.ProductUserListResponse, error) {
		resp, err := s.c.api.SearchProductUsersWithResponse(ctx, s.c.productID, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *UserSearch) filter() *nanoclient.ProductUserFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.ProductUserFilter{}
	}
	return s.req.Filter
}

func (s *UserSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}
