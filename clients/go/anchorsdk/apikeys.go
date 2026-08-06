package anchorsdk

import (
	"context"
	"time"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// APIKeys is the facade for one organization's API keys. Obtain one from an
// [Org] handle:
//
//	key, err := c.Organization(orgID).APIKeys().
//	    Create("ci").
//	    Permissions("flow:read", "flow:execute").
//	    Do(ctx)
//
// The clear key value is returned only by [APIKeyCreateBuilder.Do]; every later
// read exposes the obfuscated form. Permissions are immutable after creation.
type APIKeys struct{ o *Org }

// APIKeys returns the API key facade for this organization.
func (o *Org) APIKeys() APIKeys { return APIKeys{o: o} }

// Introspect resolves a raw organization API key against this product without
// knowing which organization issued it, and is the call a product backend makes
// on an inbound request. It reports the key, the permissions it holds, and any
// requiredScopes it is missing.
//
// A key that is valid but lacks a required scope yields 403, so the returned
// error matches [ErrForbidden]; the response body naming the missing scopes is
// retained on [Error.Body].
func (c *Client) Introspect(
	ctx context.Context,
	rawKey string,
	requiredScopes ...string,
) (*nanoclient.OrganizationAPIKeyValidateResponse, error) {
	const op = "Client.Introspect"

	// Anchor models required_scopes as optional here but required on the validate
	// endpoint. Always send it, normalized to a non-nil slice, so an empty scope
	// set produces the same body shape as a populated one — callers stubbing this
	// endpoint can then match one request shape instead of two.
	scopes := requiredScopes
	if scopes == nil {
		scopes = []string{}
	}
	body := nanoclient.IntrospectOrganizationAPIKeyJSONRequestBody{
		ApiKey:         rawKey,
		RequiredScopes: &scopes,
	}

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationAPIKeyValidateResponse, error) {
		resp, err := c.api.IntrospectOrganizationAPIKeyWithResponse(ctx, c.productID, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Create starts building an API key. The name must be unique within the
// organization, and at least one permission is required — Anchor rejects a key
// with none, and permissions cannot be changed afterwards.
func (k APIKeys) Create(name string) *APIKeyCreateBuilder {
	return &APIKeyCreateBuilder{o: k.o, req: nanoclient.OrganizationAPIKeyCreateRequest{Name: name}}
}

// Get returns one API key by ID, with its value obfuscated.
func (k APIKeys) Get(ctx context.Context, apiKeyID string) (*nanoclient.OrganizationAPIKeyResponse, error) {
	const op = "APIKeys.Get"

	c := k.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationAPIKeyResponse, error) {
		resp, err := c.api.GetOrganizationAPIKeyWithResponse(ctx, c.productID, k.o.id, apiKeyID)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Update starts building a change to an API key's name and description. A key's
// permissions are immutable; issue a new key to change them.
func (k APIKeys) Update(apiKeyID, name string) *APIKeyUpdateBuilder {
	return &APIKeyUpdateBuilder{
		o:        k.o,
		apiKeyID: apiKeyID,
		req:      nanoclient.OrganizationAPIKeyUpdateRequest{Name: name},
	}
}

// Delete revokes an API key.
func (k APIKeys) Delete(ctx context.Context, apiKeyID string) error {
	const op = "APIKeys.Delete"

	c := k.o.c

	return c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := c.api.DeleteOrganizationAPIKeyWithResponse(ctx, c.productID, k.o.id, apiKeyID)
		if err != nil {
			return transportError(op, err)
		}
		return expectSuccess(op, resp.StatusCode(), resp.Body)
	})
}

// Validate checks a raw API key against this organization and the given required
// scopes. Prefer [Client.Introspect] when the organization is not known upfront.
//
// A key that is valid but lacks a required scope yields 403, so the returned
// error matches [ErrForbidden]; the response body naming the missing scopes is
// retained on [Error.Body].
func (k APIKeys) Validate(
	ctx context.Context,
	rawKey string,
	requiredScopes ...string,
) (*nanoclient.OrganizationAPIKeyValidateResponse, error) {
	const op = "APIKeys.Validate"

	c := k.o.c
	body := nanoclient.ValidateOrganizationAPIKeyJSONRequestBody{ApiKey: rawKey, RequiredScopes: requiredScopes}

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationAPIKeyValidateResponse, error) {
		resp, err := c.api.ValidateOrganizationAPIKeyWithResponse(ctx, c.productID, k.o.id, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Search starts building an API key query.
func (k APIKeys) Search() *APIKeySearch {
	return &APIKeySearch{o: k.o}
}

// List returns the first page of the organization's API keys, using Anchor's
// default page size. It is shorthand for Search().Do(ctx).
func (k APIKeys) List(ctx context.Context) (*nanoclient.OrganizationAPIKeyListResponse, error) {
	return k.Search().Do(ctx)
}

// APIKeyCreateBuilder accumulates an API key to create. Setter methods chain;
// [APIKeyCreateBuilder.Do] sends.
type APIKeyCreateBuilder struct {
	o   *Org
	req nanoclient.OrganizationAPIKeyCreateRequest
}

// Description sets the key's description.
func (b *APIKeyCreateBuilder) Description(description string) *APIKeyCreateBuilder {
	b.req.Description = new(description)
	return b
}

// Permissions sets the permission names granted to the key, replacing any set
// earlier on this builder. At least one is required.
func (b *APIKeyCreateBuilder) Permissions(permissions ...string) *APIKeyCreateBuilder {
	b.req.Permissions = permissions
	return b
}

// ExpiresAt sets an expiry. Without one the key does not expire.
func (b *APIKeyCreateBuilder) ExpiresAt(at time.Time) *APIKeyCreateBuilder {
	b.req.ExpiresAt = new(at)
	return b
}

// ExpiresIn sets an expiry relative to now.
func (b *APIKeyCreateBuilder) ExpiresIn(d time.Duration) *APIKeyCreateBuilder {
	return b.ExpiresAt(time.Now().Add(d))
}

// Do creates the API key. The returned value carries the clear key in its Value
// field — Anchor never discloses it again.
func (b *APIKeyCreateBuilder) Do(ctx context.Context) (*nanoclient.CreatedOrganizationAPIKeyResponse, error) {
	const op = "APIKeys.Create"

	c := b.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.CreatedOrganizationAPIKeyResponse, error) {
		resp, err := c.api.CreateOrganizationAPIKeyWithResponse(ctx, c.productID, b.o.id, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}

// APIKeyUpdateBuilder accumulates an API key update. Setter methods chain;
// [APIKeyUpdateBuilder.Do] sends.
type APIKeyUpdateBuilder struct {
	o        *Org
	apiKeyID string
	req      nanoclient.OrganizationAPIKeyUpdateRequest
}

// Description sets the key's description.
func (b *APIKeyUpdateBuilder) Description(description string) *APIKeyUpdateBuilder {
	b.req.Description = new(description)
	return b
}

// Do applies the update and returns the key as it now stands.
func (b *APIKeyUpdateBuilder) Do(ctx context.Context) (*nanoclient.OrganizationAPIKeyResponse, error) {
	const op = "APIKeys.Update"

	c := b.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationAPIKeyResponse, error) {
		resp, err := c.api.UpdateOrganizationAPIKeyWithResponse(ctx, c.productID, b.o.id, b.apiKeyID, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// APIKeySearch accumulates an API key query. Setter methods chain;
// [APIKeySearch.Do] runs it.
type APIKeySearch struct {
	o   *Org
	req nanoclient.OrganizationAPIKeySearchRequest
}

// Query sets a full-text search term matched against the searchable fields.
func (s *APIKeySearch) Query(term string) *APIKeySearch {
	s.req.FullTextSearch = new(term)
	return s
}

// IDs restricts the result to the given API key IDs.
func (s *APIKeySearch) IDs(apiKeyIDs ...string) *APIKeySearch {
	s.filter().Ids = apiKeyIDs
	return s
}

// Names restricts the result to exact API key names.
func (s *APIKeySearch) Names(names ...string) *APIKeySearch {
	s.filter().Names = names
	return s
}

// Status restricts the result to keys in one of the given states.
func (s *APIKeySearch) Status(statuses ...nanoclient.OrganizationAPIKeyStatus) *APIKeySearch {
	s.filter().Status = new(statuses)
	return s
}

// LastUsedBetween restricts the result to keys last used inside the window.
func (s *APIKeySearch) LastUsedBetween(after, before time.Time) *APIKeySearch {
	f := s.filter()
	f.LastUsedAfter = new(after)
	f.LastUsedBefore = new(before)
	return s
}

// SortBy orders the result by one of Anchor's supported fields, for example
// [nanoclient.OrganizationAPIKeySearchRequestSortByLastUsedAt].
func (s *APIKeySearch) SortBy(
	field nanoclient.OrganizationAPIKeySearchRequestSortBy,
	direction nanoclient.SortDirection,
) *APIKeySearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of API keys returned.
func (s *APIKeySearch) Limit(limit int32) *APIKeySearch {
	s.page().Limit = new(limit)
	return s
}

// Offset skips the given number of API keys.
func (s *APIKeySearch) Offset(offset int32) *APIKeySearch {
	s.page().Offset = new(offset)
	return s
}

// Do runs the search.
func (s *APIKeySearch) Do(ctx context.Context) (*nanoclient.OrganizationAPIKeyListResponse, error) {
	const op = "APIKeys.Search"

	c := s.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationAPIKeyListResponse, error) {
		resp, err := c.api.SearchOrganizationAPIKeysWithResponse(ctx, c.productID, s.o.id, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *APIKeySearch) filter() *nanoclient.OrganizationAPIKeyFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.OrganizationAPIKeyFilter{}
	}
	return s.req.Filter
}

func (s *APIKeySearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}
