package anchorsdk

import (
	"context"
	"iter"
	"net/http"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// organizationType is the built-in root type every resource tree starts from.
const organizationType = "organization"

// Resource is a handle to one resource of the product, named by its type and
// the product's own ID. Making a handle calls nothing:
//
//	ws := c.Organization(orgID).Resource("workspace", workspaceID)
//	_, err := ws.Put("Default").Do(ctx)
//	ok, err := c.AccessForUser(externalUserID).On(ws).Can(ctx, "flow:run")
type Resource struct {
	o   *Org
	typ string
	id  string
}

// Resource returns a handle to one resource of this organization.
func (o *Org) Resource(resourceType, id string) *Resource {
	return &Resource{o: o, typ: resourceType, id: id}
}

// Type returns the resource type's slug.
func (r *Resource) Type() string { return r.typ }

// ID returns the product's own ID of the resource.
func (r *Resource) ID() string { return r.id }

// Put starts registering the resource, or renaming or moving it. It sits
// directly under its organization unless [ResourcePutBuilder.Under] names a
// parent.
func (r *Resource) Put(name string) *ResourcePutBuilder {
	return &ResourcePutBuilder{r: r, req: nanoclient.ResourcePutRequest{
		Name:   name,
		Parent: nanoclient.ResourceParentRef{Type: organizationType, Id: r.o.id},
	}}
}

// Get returns the resource. A resource that is not registered answers
// [ErrNotFound].
func (r *Resource) Get(ctx context.Context) (*nanoclient.ResourceResponse, error) {
	const op = "Resource.Get"

	c := r.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ResourceResponse, error) {
		resp, err := c.api.GetResourceWithResponse(ctx, c.productID, r.o.id, r.typ, r.id)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Delete removes a resource that holds no other resource and no role. Any other
// answers [ErrConflict]; use [Resource.DeleteTree] for it. A retried delete whose first attempt succeeded answers success:
// the resource is gone either way.
func (r *Resource) Delete(ctx context.Context) error {
	return r.delete(ctx, "Resource.Delete", false)
}

// DeleteTree removes the resource, every resource below it, and the roles held
// on all of them. It is retried as [Resource.Delete] is.
func (r *Resource) DeleteTree(ctx context.Context) error {
	return r.delete(ctx, "Resource.DeleteTree", true)
}

func (r *Resource) delete(ctx context.Context, op string, cascade bool) error {
	c := r.o.c
	params := &nanoclient.DeleteResourceParams{}
	if cascade {
		params.Cascade = new(true)
	}

	retried := false
	return c.retry.do(ctx, func(ctx context.Context) error {
		resp, err := c.api.DeleteResourceWithResponse(ctx, c.productID, r.o.id, r.typ, r.id, params)
		if err != nil {
			retried = true
			return transportError(op, err)
		}
		return expectGone(op, resp.StatusCode(), resp.Body, &retried)
	})
}

// ResourcePutBuilder accumulates a resource to register. Setter methods chain;
// [ResourcePutBuilder.Do] sends.
type ResourcePutBuilder struct {
	r   *Resource
	req nanoclient.ResourcePutRequest
}

// Under places the resource under another resource of the same organization.
func (b *ResourcePutBuilder) Under(parentType, parentID string) *ResourcePutBuilder {
	b.req.Parent = nanoclient.ResourceParentRef{Type: parentType, Id: parentID}
	return b
}

// Do registers the resource and returns it. The same call again changes
// nothing, so it is retried on transient failures.
func (b *ResourcePutBuilder) Do(ctx context.Context) (*nanoclient.ResourceResponse, error) {
	const op = "Resource.Put"

	c := b.r.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ResourceResponse, error) {
		resp, err := c.api.PutResourceWithResponse(ctx, c.productID, b.r.o.id, b.r.typ, b.r.id, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Assign registers the resource, or renames or moves it, as [ResourcePutBuilder.Do]
// does, then gives the member this role on it, replacing the role they held
// there before. Both happen in one transaction, so sharing an item Anchor
// does not know yet takes one call:
//
//	_, err := org.Resource("document", docID).Put("Pricing page").Under("folder", folderID).
//	    Assign(ctx, productUserID, viewerRoleID)
//
// The parent must be registered already, or be the organization. The API key
// needs resource:write besides resource_member:write. The same call again
// changes nothing, so it is retried on transient failures.
func (b *ResourcePutBuilder) Assign(
	ctx context.Context,
	productUserID, roleID string,
) (*nanoclient.ResourceMemberResponse, error) {
	const op = "Resource.Put.Assign"

	c := b.r.o.c
	body := nanoclient.ResourceMemberPutRequest{RoleIds: []string{roleID}, Resource: &b.req}

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ResourceMemberResponse, error) {
		resp, err := c.api.PutResourceMemberWithResponse(ctx, c.productID, b.r.o.id, b.r.typ, b.r.id, productUserID, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// Resources is the facade for finding an organization's resources; one resource
// is read and changed through its [Resource] handle. Obtain one from an [Org]
// handle:
//
//	for ws, err := range c.Organization(orgID).Resources().Search().Types("workspace").All(ctx) {
//	    ...
//	}
type Resources struct{ o *Org }

// Resources returns the resource search facade for this organization.
func (o *Org) Resources() Resources { return Resources{o: o} }

// Search starts building a resource query.
func (s Resources) Search() *ResourceSearch { return &ResourceSearch{o: s.o} }

// ResourceSearch accumulates a resource query. Setter methods chain;
// [ResourceSearch.Do] runs it.
type ResourceSearch struct {
	o   *Org
	req nanoclient.ResourceSearchRequest
}

// Types keeps resources of these types.
func (s *ResourceSearch) Types(resourceTypes ...string) *ResourceSearch {
	s.filter().ResourceTypes = resourceTypes
	return s
}

// IDs keeps resources with these product IDs.
func (s *ResourceSearch) IDs(ids ...string) *ResourceSearch {
	s.filter().Ids = ids
	return s
}

// NameContains keeps resources whose name contains the text, regardless of case.
func (s *ResourceSearch) NameContains(text string) *ResourceSearch {
	s.filter().NameContains = new(text)
	return s
}

// Under keeps the direct children of one parent: a resource, or the
// organization ("organization", its ID).
func (s *ResourceSearch) Under(parentType, parentID string) *ResourceSearch {
	s.filter().Parent = &nanoclient.ResourceParentRef{Type: parentType, Id: parentID}
	return s
}

// SortBy orders the result, for example by [nanoclient.ResourceSortFieldName].
func (s *ResourceSearch) SortBy(field nanoclient.ResourceSortField, direction nanoclient.SortDirection) *ResourceSearch {
	s.req.SortBy = new(field)
	s.req.SortDirection = new(direction)
	return s
}

// Limit caps the number of resources on a page.
func (s *ResourceSearch) Limit(limit int32) *ResourceSearch {
	s.page().Limit = new(limit)
	return s
}

// Cursor continues after the last item of a previous page. To read every page,
// use [ResourceSearch.All].
func (s *ResourceSearch) Cursor(cursor string) *ResourceSearch {
	s.page().Cursor = new(cursor)
	return s
}

// All reads every matching resource, following NextCursor from page to page.
// The builder is not changed. A failed page ends the sequence with its error.
func (s *ResourceSearch) All(ctx context.Context) iter.Seq2[nanoclient.ResourceResponse, error] {
	return allPages(ctx, "Resources.Search", startCursor(s.req.Pagination),
		func(ctx context.Context, cursor string) ([]nanoclient.ResourceResponse, string, error) {
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
func (s *ResourceSearch) Do(ctx context.Context) (*nanoclient.ResourceListResponse, error) {
	const op = "Resources.Search"

	c := s.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.ResourceListResponse, error) {
		resp, err := c.api.SearchResourcesWithResponse(ctx, c.productID, s.o.id, s.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

func (s *ResourceSearch) filter() *nanoclient.ResourceFilter {
	if s.req.Filter == nil {
		s.req.Filter = &nanoclient.ResourceFilter{}
	}
	return s.req.Filter
}

func (s *ResourceSearch) page() *nanoclient.PaginationRequest {
	if s.req.Pagination == nil {
		s.req.Pagination = &nanoclient.PaginationRequest{}
	}
	return s.req.Pagination
}

// expectGone accepts a delete's answer. A 404 that follows a failed attempt
// means that attempt reached Anchor and deleted it. retried records whether
// this attempt failed, for the next one.
func expectGone(op string, code int, body []byte, retried *bool) error {
	if code == http.StatusNotFound && *retried {
		return nil
	}
	err := expectSuccess(op, code, body)
	*retried = err != nil
	return err
}
