package anchorsdk

import (
	"context"
	"iter"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// Workspaces is the facade for one organization's workspaces. Obtain one from an
// [Org] handle:
//
//	ws, err := c.Organization(orgID).Workspaces().Create("Production").Do(ctx)
//
// Deprecated: Workspaces are resources of the product's workspace resource type
// (ADR-0022 A6). Use the resource routes instead.
type Workspaces struct{ o *Org }

// Workspaces returns the workspace facade for this organization.
//
// Deprecated: Workspaces are resources of the product's workspace resource type
// (ADR-0022 A6). Use the resource routes instead.
func (o *Org) Workspaces() Workspaces { return Workspaces{o: o} }

// Create starts building a workspace. The name must be unique within the
// organization.
func (w Workspaces) Create(name string) *WorkspaceCreateBuilder {
	return &WorkspaceCreateBuilder{o: w.o, req: nanoclient.ProductWorkspaceRequest{Name: name}}
}

// Get returns one workspace by ID.
func (w Workspaces) Get(ctx context.Context, workspaceID string) (*nanoclient.ProductWorkspaceResponse, error) {
	const op = "Workspaces.Get"

	c := w.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ProductWorkspaceResponse, error) {
		resp, err := c.api.GetOrganizationWorkspaceWithResponse(ctx, c.productID, w.o.id, workspaceID)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Update starts building a full replacement of the workspace. Anchor treats the
// update as a PUT: fields the builder leaves unset are cleared.
func (w Workspaces) Update(workspaceID, name string) *WorkspaceUpdateBuilder {
	return &WorkspaceUpdateBuilder{
		o:           w.o,
		workspaceID: workspaceID,
		req:         nanoclient.ProductWorkspaceRequest{Name: name},
	}
}

// Delete removes a workspace.
func (w Workspaces) Delete(ctx context.Context, workspaceID string) error {
	const op = "Workspaces.Delete"

	c := w.o.c

	return c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := c.api.DeleteOrganizationWorkspaceWithResponse(ctx, c.productID, w.o.id, workspaceID)
		if err != nil {
			return transportError(op, err)
		}
		return expectSuccess(op, resp.StatusCode(), resp.Body)
	})
}

// Search starts building a workspace query.
func (w Workspaces) Search() *WorkspaceSearch {
	return &WorkspaceSearch{o: w.o}
}

// List returns the first page of the organization's workspaces, using Anchor's
// default page size. It is shorthand for Search().Do(ctx).
func (w Workspaces) List(ctx context.Context) (*nanoclient.ProductWorkspaceListResponse, error) {
	return w.Search().Do(ctx)
}

// WorkspaceCreateBuilder accumulates a workspace to create. Setter methods chain;
// [WorkspaceCreateBuilder.Do] sends.
type WorkspaceCreateBuilder struct {
	o   *Org
	req nanoclient.ProductWorkspaceRequest
}

// Description sets the workspace's description.
func (b *WorkspaceCreateBuilder) Description(description string) *WorkspaceCreateBuilder {
	b.req.Description = new(description)
	return b
}

// Do creates the workspace.
func (b *WorkspaceCreateBuilder) Do(ctx context.Context) (*nanoclient.ProductWorkspaceResponse, error) {
	const op = "Workspaces.Create"

	c := b.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ProductWorkspaceResponse, error) {
		resp, err := c.api.CreateOrganizationWorkspaceWithResponse(ctx, c.productID, b.o.id, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}

// WorkspaceUpdateBuilder accumulates a workspace update. Setter methods chain;
// [WorkspaceUpdateBuilder.Do] sends.
type WorkspaceUpdateBuilder struct {
	o           *Org
	workspaceID string
	req         nanoclient.ProductWorkspaceRequest
}

// Description sets the workspace's description.
func (b *WorkspaceUpdateBuilder) Description(description string) *WorkspaceUpdateBuilder {
	b.req.Description = new(description)
	return b
}

// Do applies the update and returns the workspace as it now stands.
func (b *WorkspaceUpdateBuilder) Do(ctx context.Context) (*nanoclient.ProductWorkspaceResponse, error) {
	const op = "Workspaces.Update"

	c := b.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ProductWorkspaceResponse, error) {
		resp, err := c.api.UpdateOrganizationWorkspaceWithResponse(ctx, c.productID, b.o.id, b.workspaceID, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// WorkspaceSearch accumulates a workspace query. Setter methods chain;
// [WorkspaceSearch.Do] runs it.
type WorkspaceSearch struct {
	o   *Org
	req nanoclient.ProductWorkspaceSearchRequest
}

// Query sets a full-text search term matched against the searchable fields.
func (s *WorkspaceSearch) Query(term string) *WorkspaceSearch {
	s.req.FullTextSearch = new(term)
	return s
}

// IDs restricts the result to the given workspace IDs.
func (s *WorkspaceSearch) IDs(workspaceIDs ...string) *WorkspaceSearch {
	s.filter().Ids = workspaceIDs
	return s
}

// Names restricts the result to exact workspace names.
func (s *WorkspaceSearch) Names(names ...string) *WorkspaceSearch {
	s.filter().Names = names
	return s
}

// SortBy orders the result by one of Anchor's supported fields, for example
// [nanoclient.ProductWorkspaceSearchRequestSortByName].
func (s *WorkspaceSearch) SortBy(
	field nanoclient.ProductWorkspaceSearchRequestSortBy,
	direction nanoclient.SortDirection,
) *WorkspaceSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of workspaces returned.
func (s *WorkspaceSearch) Limit(limit int32) *WorkspaceSearch {
	s.page().Limit = new(limit)
	return s
}

// Offset skips the given number of workspaces, for page-number navigation. To read every
// page, iterate with Cursor or All instead.
func (s *WorkspaceSearch) Offset(offset int32) *WorkspaceSearch {
	s.page().Offset = new(offset)
	return s
}

// Cursor continues the search after the last item of a previous page. Pass the
// previous page's NextCursor; Anchor refuses a cursor combined with an offset,
// and answers a cursor from another filter or sort with an error. To read every
// page, use [WorkspaceSearch.All].
func (s *WorkspaceSearch) Cursor(cursor string) *WorkspaceSearch {
	s.page().Cursor = new(cursor)
	return s
}

// All reads every matching item, following NextCursor from page to page until
// Anchor sends none. It starts where the builder starts: at the first item, or
// after the cursor set with [WorkspaceSearch.Cursor]. The builder is not changed, so it can
// run again. A failed page ends the sequence with its error.
//
//	for item, err := range search.Limit(100).All(ctx) {
//	    if err != nil {
//	        return err
//	    }
//	    // use item
//	}
func (s *WorkspaceSearch) All(ctx context.Context) iter.Seq2[nanoclient.ProductWorkspaceResponse, error] {
	return allPages(ctx, "Workspaces.Search", startCursor(s.req.Pagination),
		func(ctx context.Context, cursor string) ([]nanoclient.ProductWorkspaceResponse, string, error) {
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
func (s *WorkspaceSearch) Do(ctx context.Context) (*nanoclient.ProductWorkspaceListResponse, error) {
	const op = "Workspaces.Search"

	c := s.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ProductWorkspaceListResponse, error) {
		resp, err := c.api.SearchOrganizationWorkspacesWithResponse(ctx, c.productID, s.o.id, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *WorkspaceSearch) filter() *nanoclient.WorkspaceFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.WorkspaceFilter{}
	}
	return s.req.Filter
}

func (s *WorkspaceSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}
