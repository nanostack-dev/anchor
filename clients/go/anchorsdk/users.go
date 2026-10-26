package anchorsdk

import (
	"context"
	"iter"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// Users is the facade for the product's users. Obtain one with [Client.Users]:
//
//	user, err := c.Users().Create("new.user@company.com").Name("New User").Do(ctx)
//
// A product that is its own identity provider also sends its ID for the user,
// and then evaluates access by that ID:
//
//	user, err := c.Users().Create(email).Name(name).ExternalID(accountID).Do(ctx)
//	decision, err := c.AccessForUser(accountID).InOrganization(orgID).Permissions("flows:read").Do(ctx)
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
// role held in each. It reads every page of [Users.SearchOrganizations], so the
// result is complete however many organizations the user belongs to; search
// instead to page through them yourself.
func (u Users) Organizations(
	ctx context.Context,
	productUserID string,
) (*nanoclient.UserOrganizationListResponse, error) {
	return u.allOrganizations(ctx, productUserID, false)
}

// OrganizationsWithRolePermissions is [Users.Organizations] with each role's
// permissions resolved, saving a second call when the caller needs to authorize
// the user.
func (u Users) OrganizationsWithRolePermissions(
	ctx context.Context,
	productUserID string,
) (*nanoclient.UserOrganizationListResponse, error) {
	return u.allOrganizations(ctx, productUserID, true)
}

// SearchOrganizations starts building a query over the organizations the product
// user belongs to, with the role held in each.
//
//	for membership, err := range c.Users().SearchOrganizations(userID).WithRolePermissions().All(ctx) {
//	    if err != nil {
//	        return err
//	    }
//	    // use membership
//	}
func (u Users) SearchOrganizations(productUserID string) *UserOrganizationSearch {
	return &UserOrganizationSearch{c: u.c, productUserID: productUserID}
}

// userOrganizationsPageSize is the page [Users.Organizations] reads with: the
// largest limit the contract documents.
const userOrganizationsPageSize int32 = 100

func (u Users) allOrganizations(
	ctx context.Context,
	productUserID string,
	withRolePermissions bool,
) (*nanoclient.UserOrganizationListResponse, error) {
	search := u.SearchOrganizations(productUserID).Limit(userOrganizationsPageSize)
	if withRolePermissions {
		search.WithRolePermissions()
	}

	items := []nanoclient.UserOrganizationResponse{}
	for item, err := range search.All(ctx) {
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &nanoclient.UserOrganizationListResponse{Items: items}, nil
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

// ExternalID sets the user's ID in the product's own identity provider, the ID
// [Client.AccessForUser] names the user by. It must be unique within the
// product, and it cannot change after the create.
func (b *UserCreateBuilder) ExternalID(externalID string) *UserCreateBuilder {
	b.req.ExternalId = new(externalID)
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

// Offset skips the given number of users, for page-number navigation. To read every
// page, iterate with Cursor or All instead.
func (s *UserSearch) Offset(offset int32) *UserSearch {
	s.page().Offset = new(offset)
	return s
}

// Cursor continues the search after the last item of a previous page. Pass the
// previous page's NextCursor; Anchor refuses a cursor combined with an offset,
// and answers a cursor from another filter or sort with an error. To read every
// page, use [UserSearch.All].
func (s *UserSearch) Cursor(cursor string) *UserSearch {
	s.page().Cursor = new(cursor)
	return s
}

// All reads every matching item, following NextCursor from page to page until
// Anchor sends none. It starts where the builder starts: at the first item, or
// after the cursor set with [UserSearch.Cursor]. The builder is not changed, so it can
// run again. A failed page ends the sequence with its error.
//
//	for item, err := range search.Limit(100).All(ctx) {
//	    if err != nil {
//	        return err
//	    }
//	    // use item
//	}
func (s *UserSearch) All(ctx context.Context) iter.Seq2[nanoclient.ProductUserResponse, error] {
	return allPages(ctx, "Users.Search", startCursor(s.req.Pagination),
		func(ctx context.Context, cursor string) ([]nanoclient.ProductUserResponse, string, error) {
			next := *s
			next.req.Pagination = pageAfter(s.req.Pagination, cursor)
			page, err := next.Do(ctx)
			if err != nil {
				return nil, "", err
			}
			return page.Items, derefCursor(page.NextCursor), nil
		})
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

// UserOrganizationSearch accumulates a query over the organizations one product
// user belongs to. Setter methods chain; [UserOrganizationSearch.Do] runs it.
type UserOrganizationSearch struct {
	c             *Client
	productUserID string
	include       []nanoclient.UserOrganizationInclude
	req           nanoclient.UserOrganizationSearchRequest
}

// WithRolePermissions resolves the permissions of the role held in every
// organization of the page.
func (s *UserOrganizationSearch) WithRolePermissions() *UserOrganizationSearch {
	s.include = append(s.include, nanoclient.UserOrganizationIncludeRolePermissions)
	return s
}

// OrganizationIDs restricts the result to memberships in the given organizations.
func (s *UserOrganizationSearch) OrganizationIDs(organizationIDs ...string) *UserOrganizationSearch {
	s.filter().OrganizationIds = organizationIDs
	return s
}

// RoleIDs restricts the result to memberships holding one of the given roles.
func (s *UserOrganizationSearch) RoleIDs(roleIDs ...string) *UserOrganizationSearch {
	s.filter().RoleIds = roleIDs
	return s
}

// SortBy orders the result by one of Anchor's supported fields, for example
// [nanoclient.UserOrganizationSearchRequestSortByJoinedAt]. Without it the
// result runs from the earliest joined organization.
func (s *UserOrganizationSearch) SortBy(
	field nanoclient.UserOrganizationSearchRequestSortBy,
	direction nanoclient.SortDirection,
) *UserOrganizationSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of organizations returned.
func (s *UserOrganizationSearch) Limit(limit int32) *UserOrganizationSearch {
	s.page().Limit = new(limit)
	return s
}

// Offset skips the given number of organizations, for page-number navigation. To
// read every page, iterate with Cursor or All instead.
func (s *UserOrganizationSearch) Offset(offset int32) *UserOrganizationSearch {
	s.page().Offset = new(offset)
	return s
}

// Cursor continues the search after the last item of a previous page. Pass the
// previous page's NextCursor; Anchor refuses a cursor combined with an offset.
// To read every page, use [UserOrganizationSearch.All].
func (s *UserOrganizationSearch) Cursor(cursor string) *UserOrganizationSearch {
	s.page().Cursor = new(cursor)
	return s
}

// All reads every matching organization, following NextCursor from page to page
// until Anchor sends none. The builder is not changed, so it can run again. A
// failed page ends the sequence with its error.
func (s *UserOrganizationSearch) All(ctx context.Context) iter.Seq2[nanoclient.UserOrganizationResponse, error] {
	return allPages(ctx, "Users.SearchOrganizations", startCursor(s.req.Pagination),
		func(ctx context.Context, cursor string) ([]nanoclient.UserOrganizationResponse, string, error) {
			next := *s
			next.req.Pagination = pageAfter(s.req.Pagination, cursor)
			page, err := next.Do(ctx)
			if err != nil {
				return nil, "", err
			}
			return page.Items, derefCursor(page.NextCursor), nil
		})
}

// Do runs the search.
func (s *UserOrganizationSearch) Do(ctx context.Context) (*nanoclient.UserOrganizationSearchResponse, error) {
	const op = "Users.SearchOrganizations"

	params := &nanoclient.SearchUserOrganizationsParams{}
	if len(s.include) > 0 {
		params.Include = &s.include
	}

	return retrying(ctx, s.c, func(ctx context.Context) (*nanoclient.UserOrganizationSearchResponse, error) {
		resp, err := s.c.api.SearchUserOrganizationsWithResponse(
			ctx, s.c.productID, s.productUserID, params, s.req,
		)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *UserOrganizationSearch) filter() *nanoclient.UserOrganizationFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.UserOrganizationFilter{}
	}
	return s.req.Filter
}

func (s *UserOrganizationSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}
