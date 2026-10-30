package anchorsdk_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"reflect"
	"strings"
	"testing"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
)

// pageReply renders a page of ids as the search response Anchor sends, with the
// next_cursor property absent on the last page.
func pageReply(item func(id string) string, nextCursor string, ids ...string) stubResponse {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, item(id))
	}

	cursor := ""
	if nextCursor != "" {
		cursor = fmt.Sprintf(`,"next_cursor":%q`, nextCursor)
	}

	return stubResponse{
		status: http.StatusOK,
		body:   fmt.Sprintf(`{"count":%d,"total":5,"items":[%s]%s}`, len(ids), strings.Join(items, ","), cursor),
	}
}

func idItem(id string) string { return fmt.Sprintf(`{"id":%q}`, id) }

// requestPagination decodes the pagination object a request carried.
func requestPagination(t *testing.T, call recordedRequest) map[string]any {
	t.Helper()

	var body struct {
		Pagination map[string]any `json:"pagination"`
	}
	if err := json.Unmarshal(call.body, &body); err != nil {
		t.Fatalf("decode request body %q: %v", call.body, err)
	}

	return body.Pagination
}

// mapIDs turns a sequence of items into a sequence of their identities.
func mapIDs[T any](seq iter.Seq2[T, error], id func(T) string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for item, err := range seq {
			if !yield(id(item), err) {
				return
			}
		}
	}
}

func collect(t *testing.T, seq iter.Seq2[string, error]) []string {
	t.Helper()

	var got []string
	for id, err := range seq {
		if err != nil {
			t.Fatalf("iteration: %v", err)
		}
		got = append(got, id)
	}

	return got
}

type allCase struct {
	name string
	path string
	item func(id string) string
	// all builds the search with a limit of two and an offset of four, then
	// iterates it.
	all func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error]
}

func allCases() []allCase {
	return []allCase{
		{
			name: "organizations",
			path: "/v1/products/prd_test/organizations/search",
			item: idItem,
			all: func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error] {
				seq := c.Organizations().Search().Query("acme").Limit(2).Offset(4).All(ctx)
				return mapIDs(seq, func(o nanoclient.ProductOrganizationResponse) string { return o.Id })
			},
		},
		{
			name: "members",
			path: "/v1/products/prd_test/organizations/org_1/members/search",
			item: func(id string) string { return fmt.Sprintf(`{"product_user_id":%q}`, id) },
			all: func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error] {
				seq := c.Organization(testOrgID).Members().Search().Limit(2).Offset(4).All(ctx)
				return mapIDs(seq, func(m nanoclient.OrganizationMemberResponse) string { return m.ProductUserId })
			},
		},
		{
			name: "api keys",
			path: "/v1/products/prd_test/organizations/org_1/api-keys/search",
			item: idItem,
			all: func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error] {
				seq := c.Organization(testOrgID).APIKeys().Search().Limit(2).Offset(4).All(ctx)
				return mapIDs(seq, func(k nanoclient.OrganizationAPIKeyResponse) string { return k.Id })
			},
		},
		{
			name: "organization invitations",
			path: "/v1/products/prd_test/organizations/org_1/invitations/search",
			item: idItem,
			all: func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error] {
				seq := c.Organization(testOrgID).Invitations().Search().Limit(2).Offset(4).All(ctx)
				return mapIDs(seq, func(i nanoclient.OrganizationInvitationResponse) string { return i.Id })
			},
		},
		{
			name: "product invitations",
			path: "/v1/products/prd_test/invitations/search",
			item: idItem,
			all: func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error] {
				seq := c.Invitations().Search().Limit(2).Offset(4).All(ctx)
				return mapIDs(seq, func(i nanoclient.OrganizationInvitationResponse) string { return i.Id })
			},
		},
		{
			name: "users",
			path: "/v1/products/prd_test/product-users/search",
			item: idItem,
			all: func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error] {
				seq := c.Users().Search().Limit(2).Offset(4).All(ctx)
				return mapIDs(seq, func(u nanoclient.ProductUserResponse) string { return u.Id })
			},
		},
		{
			name: "user organizations",
			path: "/v1/products/prd_test/product-users/pusr_1/organizations/search",
			item: func(id string) string { return fmt.Sprintf(`{"organization":{"id":%q}}`, id) },
			all: func(ctx context.Context, c *anchorsdk.Client) iter.Seq2[string, error] {
				seq := c.Users().SearchOrganizations(testUserID).Limit(2).Offset(4).All(ctx)
				return mapIDs(seq, func(o nanoclient.UserOrganizationResponse) string { return o.Organization.Id })
			},
		},
	}
}

func TestAllFollowsNextCursorToTheLastPage(t *testing.T) {
	for _, tt := range allCases() {
		t.Run(tt.name, func(t *testing.T) {
			stub, baseURL := newStubServer(t,
				pageReply(tt.item, "cursor-1", "a1", "a2"),
				pageReply(tt.item, "cursor-2", "b1", "b2"),
				pageReply(tt.item, "", "c1"),
			)
			c := newTestClient(t, baseURL, 1)

			got := collect(t, tt.all(t.Context(), c))

			if want := []string{"a1", "a2", "b1", "b2", "c1"}; !reflect.DeepEqual(got, want) {
				t.Errorf("items = %v, want %v", got, want)
			}

			calls := stub.calls()
			if len(calls) != 3 {
				t.Fatalf("server saw %d requests, want 3", len(calls))
			}
			for _, call := range calls {
				if call.method != http.MethodPost || call.path != tt.path {
					t.Errorf("request = %s %s, want POST %s", call.method, call.path, tt.path)
				}
			}

			first := requestPagination(t, calls[0])
			if _, ok := first["cursor"]; ok {
				t.Errorf("first page sent a cursor: %v", first)
			}
			if first["offset"] != float64(4) || first["limit"] != float64(2) {
				t.Errorf("first page pagination = %v, want the offset and limit the caller set", first)
			}

			// A cursor already says where the page starts. Anchor answers a cursor
			// combined with an offset with a 400, so only the first page carries one.
			for i, wantCursor := range []string{"cursor-1", "cursor-2"} {
				next := requestPagination(t, calls[i+1])
				if next["cursor"] != wantCursor {
					t.Errorf("page %d cursor = %v, want %q", i+2, next["cursor"], wantCursor)
				}
				if _, ok := next["offset"]; ok {
					t.Errorf("page %d sent an offset with its cursor: %v", i+2, next)
				}
				if next["limit"] != float64(2) {
					t.Errorf("page %d limit = %v, want 2", i+2, next["limit"])
				}
			}
		})
	}
}

func TestAllStopsAtTheFirstPageWithoutNextCursor(t *testing.T) {
	// A full last page has no next_cursor, and a short page can still have one:
	// the cursor decides, never the count.
	stub, baseURL := newStubServer(t,
		pageReply(idItem, "cursor-1", "a1"),
		pageReply(idItem, "", "b1", "b2"),
	)
	c := newTestClient(t, baseURL, 1)

	seq := mapIDs(c.Organizations().Search().Limit(2).All(t.Context()),
		func(o nanoclient.ProductOrganizationResponse) string { return o.Id })
	got := collect(t, seq)

	if want := []string{"a1", "b1", "b2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if n := len(stub.calls()); n != 2 {
		t.Errorf("server saw %d requests, want 2", n)
	}
}

func TestAllResumesFromACursor(t *testing.T) {
	stub, baseURL := newStubServer(t, pageReply(idItem, "", "z1"))
	c := newTestClient(t, baseURL, 1)

	seq := mapIDs(c.Organizations().Search().Cursor("saved-cursor").All(t.Context()),
		func(o nanoclient.ProductOrganizationResponse) string { return o.Id })
	got := collect(t, seq)

	if want := []string{"z1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if cursor := requestPagination(t, stub.calls()[0])["cursor"]; cursor != "saved-cursor" {
		t.Errorf("cursor = %v, want the one the caller set", cursor)
	}
}

func TestAllKeepsFilterAndSortOnEveryPage(t *testing.T) {
	stub, baseURL := newStubServer(t,
		pageReply(idItem, "cursor-1", "a1"),
		pageReply(idItem, "", "b1"),
	)
	c := newTestClient(t, baseURL, 1)

	search := c.Organizations().Search().
		Names("Acme").
		SortBy(nanoclient.ProductOrganizationSearchRequestSortByName, nanoclient.DESC).
		Limit(1)
	collect(t, mapIDs(search.All(t.Context()),
		func(o nanoclient.ProductOrganizationResponse) string { return o.Id }))

	for i, call := range stub.calls() {
		var body struct {
			Filter        map[string]any `json:"filter"`
			SortBy        string         `json:"sort_by"`
			SortDirection string         `json:"sort_direction"`
		}
		if err := json.Unmarshal(call.body, &body); err != nil {
			t.Fatalf("decode request %d: %v", i, err)
		}
		if body.SortBy != "name" || body.SortDirection != "DESC" || body.Filter["names"] == nil {
			t.Errorf("request %d lost the filter or sort: %s", i, call.body)
		}
	}
}

func TestAllCanRunTwice(t *testing.T) {
	_, baseURL := newStubServer(t,
		pageReply(idItem, "cursor-1", "a1"),
		pageReply(idItem, "", "b1"),
		pageReply(idItem, "cursor-1", "a1"),
		pageReply(idItem, "", "b1"),
	)
	c := newTestClient(t, baseURL, 1)
	search := c.Organizations().Search().Limit(1).Offset(3)

	for run := range 2 {
		got := collect(t, mapIDs(search.All(t.Context()),
			func(o nanoclient.ProductOrganizationResponse) string { return o.Id }))
		if want := []string{"a1", "b1"}; !reflect.DeepEqual(got, want) {
			t.Errorf("run %d items = %v, want %v", run, got, want)
		}
	}
}

func TestAllStopsWhenTheCallerBreaks(t *testing.T) {
	stub, baseURL := newStubServer(t,
		pageReply(idItem, "cursor-1", "a1", "a2"),
		pageReply(idItem, "", "b1"),
	)
	c := newTestClient(t, baseURL, 1)

	var got []string
	for org, err := range c.Organizations().Search().Limit(2).All(t.Context()) {
		if err != nil {
			t.Fatalf("iteration: %v", err)
		}
		got = append(got, org.Id)
		break
	}

	if want := []string{"a1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if n := len(stub.calls()); n != 1 {
		t.Errorf("server saw %d requests, want the one page the caller used", n)
	}
}

func TestAllYieldsAFailedPageOnceAndStops(t *testing.T) {
	_, baseURL := newStubServer(
		t,
		pageReply(idItem, "cursor-1", "a1"),
		stubResponse{
			status: http.StatusBadRequest,
			body:   `{"errors":[{"code":"INVALID_CURSOR","message":"The cursor is invalid for this search."}]}`,
		},
	)
	c := newTestClient(t, baseURL, 3)

	var items []string
	var failures []error
	for org, err := range c.Organizations().Search().Limit(1).All(t.Context()) {
		if err != nil {
			failures = append(failures, err)
			continue
		}
		items = append(items, org.Id)
	}

	if !reflect.DeepEqual(items, []string{"a1"}) {
		t.Errorf("items = %v, want the page that succeeded", items)
	}
	if len(failures) != 1 {
		t.Fatalf("saw %d errors, want exactly one", len(failures))
	}
	if !errors.Is(failures[0], anchorsdk.ErrInvalid) {
		t.Errorf("error = %v, want ErrInvalid", failures[0])
	}
	var apiErr *anchorsdk.Error
	if !errors.As(failures[0], &apiErr) || len(apiErr.Details) == 0 || apiErr.Details[0].Code != "INVALID_CURSOR" {
		t.Errorf("error = %v, want the INVALID_CURSOR detail", failures[0])
	}
}

func TestAllRefusesACursorThatDoesNotAdvance(t *testing.T) {
	_, baseURL := newStubServer(t, pageReply(idItem, "stuck", "a1"))
	c := newTestClient(t, baseURL, 1)

	var failure error
	count := 0
	for _, err := range c.Organizations().Search().Cursor("stuck").All(t.Context()) {
		if err != nil {
			failure = err
			break
		}
		count++
	}

	if failure == nil {
		t.Fatal("a cursor that never advances must end the walk with an error, not loop")
	}
	if count != 1 {
		t.Errorf("items before the error = %d, want 1", count)
	}
}

func TestSearchDoExposesNextCursor(t *testing.T) {
	_, baseURL := newStubServer(t, pageReply(idItem, "cursor-1", "a1"))
	c := newTestClient(t, baseURL, 1)

	page, err := c.Organizations().Search().Limit(1).Do(t.Context())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if page.NextCursor == nil || *page.NextCursor != "cursor-1" {
		t.Errorf("NextCursor = %v, want cursor-1", page.NextCursor)
	}
}

func TestUserOrganizationsReadsEveryPage(t *testing.T) {
	item := func(id string) string {
		return fmt.Sprintf(`{"organization":{"id":%q},"role":{"id":"prole_1","name":"Admin"}}`, id)
	}
	stub, baseURL := newStubServer(t,
		pageReply(item, "cursor-1", "org_1", "org_2"),
		pageReply(item, "", "org_3"),
	)
	c := newTestClient(t, baseURL, 1)

	result, err := c.Users().OrganizationsWithRolePermissions(t.Context(), testUserID)
	if err != nil {
		t.Fatalf("OrganizationsWithRolePermissions: %v", err)
	}

	var got []string
	for _, membership := range result.Items {
		got = append(got, membership.Organization.Id)
	}
	if want := []string{"org_1", "org_2", "org_3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("organizations = %v, want %v", got, want)
	}

	calls := stub.calls()
	if len(calls) != 2 {
		t.Fatalf("server saw %d requests, want one per page", len(calls))
	}
	if query := calls[0].path; query != "/v1/products/prd_test/product-users/pusr_1/organizations/search" {
		t.Errorf("path = %s", query)
	}
	if cursor := requestPagination(t, calls[1])["cursor"]; cursor != "cursor-1" {
		t.Errorf("second page cursor = %v, want cursor-1", cursor)
	}
}
