package anchorsdk_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
)

const (
	resourceBody = `{"id":"ws_1","type":"workspace","name":"Default","organization_id":"org_1",` +
		`"parent":{"type":"organization","id":"org_1"}}`
	resourceMemberBody = `{"product_user_id":"pusr_1","roles":[{"id":"prole_1","name":"Editor"}]}`
	workspacePath      = "/v1/products/prd_test/organizations/org_1/resources/workspace/ws_1"
)

func TestResourceOperationsUseExpectedRoutes(t *testing.T) {
	workspace := func(c *anchorsdk.Client) *anchorsdk.Resource {
		return c.Organization(testOrgID).Resource("workspace", "ws_1")
	}
	tests := []struct {
		name       string
		reply      stubResponse
		call       func(context.Context, *anchorsdk.Client) error
		wantMethod string
		wantPath   string
		wantQuery  string
		wantBody   string
	}{
		{
			name:  "a resource put without a parent sits under its organization",
			reply: stubResponse{status: http.StatusOK, body: resourceBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := workspace(c).Put("Default").Do(ctx)
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   workspacePath,
			wantBody:   `{"name":"Default","parent":{"type":"organization","id":"org_1"}}`,
		},
		{
			name:  "a resource put under a parent",
			reply: stubResponse{status: http.StatusOK, body: resourceBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).Resource("folder", "f_1").Put("Specs").
					Under("workspace", "ws_1").Do(ctx)
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   "/v1/products/prd_test/organizations/org_1/resources/folder/f_1",
			wantBody:   `{"name":"Specs","parent":{"type":"workspace","id":"ws_1"}}`,
		},
		{
			name:  "a put that assigns sends the resource with the role on the member route",
			reply: stubResponse{status: http.StatusOK, body: resourceMemberBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).Resource("folder", "f_1").Put("Specs").
					Under("workspace", "ws_1").Assign(ctx, testUserID, testRoleID)
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   "/v1/products/prd_test/organizations/org_1/resources/folder/f_1/members/" + testUserID,
			wantBody: `{"resource":{"name":"Specs","parent":{"type":"workspace","id":"ws_1"}},` +
				`"role_ids":["` + testRoleID + `"]}`,
		},
		{
			name:       "a resource get",
			reply:      stubResponse{status: http.StatusOK, body: resourceBody},
			call:       func(ctx context.Context, c *anchorsdk.Client) error { _, err := workspace(c).Get(ctx); return err },
			wantMethod: http.MethodGet,
			wantPath:   workspacePath,
		},
		{
			name:       "a resource delete keeps its children",
			reply:      stubResponse{status: http.StatusNoContent},
			call:       func(ctx context.Context, c *anchorsdk.Client) error { return workspace(c).Delete(ctx) },
			wantMethod: http.MethodDelete,
			wantPath:   workspacePath,
		},
		{
			name:       "a tree delete cascades",
			reply:      stubResponse{status: http.StatusNoContent},
			call:       func(ctx context.Context, c *anchorsdk.Client) error { return workspace(c).DeleteTree(ctx) },
			wantMethod: http.MethodDelete,
			wantPath:   workspacePath,
			wantQuery:  "cascade=true",
		},
		{
			name:  "a resource search",
			reply: stubResponse{status: http.StatusOK, body: listBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).Resources().Search().
					Types("workspace").IDs("ws_1").NameContains("def").Under("organization", testOrgID).
					SortBy(nanoclient.ResourceSortFieldName, nanoclient.ASC).Limit(10).Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/resources/search",
			wantBody: `{"filter":{"resource_types":["workspace"],"ids":["ws_1"],"name_contains":"def",` +
				`"parent":{"type":"organization","id":"org_1"}},"pagination":{"limit":10},` +
				`"sort_by":"name","sort_direction":"ASC"}`,
		},
		{
			name:  "a role assignment on a resource",
			reply: stubResponse{status: http.StatusOK, body: resourceMemberBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := workspace(c).Members().Assign(ctx, testUserID, testRoleID)
				return err
			},
			wantMethod: http.MethodPut,
			wantPath:   workspacePath + "/members/pusr_1",
			wantBody:   `{"role_ids":["prole_1"]}`,
		},
		{
			name:  "a member's roles on a resource",
			reply: stubResponse{status: http.StatusOK, body: resourceMemberBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := workspace(c).Members().Get(ctx, testUserID)
				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   workspacePath + "/members/pusr_1",
		},
		{
			name:  "a member removed from a resource",
			reply: stubResponse{status: http.StatusNoContent},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				return workspace(c).Members().Remove(ctx, testUserID)
			},
			wantMethod: http.MethodDelete,
			wantPath:   workspacePath + "/members/pusr_1",
		},
		{
			name:  "a resource member search",
			reply: stubResponse{status: http.StatusOK, body: listBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := workspace(c).Members().Search().RoleIDs(testRoleID).
					SortBy(nanoclient.ResourceMemberSortFieldCreatedAt, nanoclient.DESC).Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   workspacePath + "/members/search",
			wantBody:   `{"filter":{"role_ids":["prole_1"]},"sort_by":"created_at","sort_direction":"DESC"}`,
		},
		{
			name:  "access on a resource handle keeps a parent named after it",
			reply: stubResponse{status: http.StatusOK, body: `{"decision":true,"context":{},"evaluations":[{"decision":true}]}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				document := c.Organization(testOrgID).Resource("document", "d_1")
				_, err := c.AccessForUser("user_1").On(document).Under("folder", "f_1").Can(ctx, "document:view")
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/access/v1/evaluations",
			wantBody: `{"evaluations":[{"action":{"name":"document:view"}}],` +
				`"resource":{"id":"d_1","properties":{"organization_id":"org_1",` +
				`"parent":{"id":"f_1","type":"folder"}},"type":"document"},"subject":{"id":"user_1","type":"user"}}`,
		},
		{
			name:  "access on a resource handle",
			reply: stubResponse{status: http.StatusOK, body: `{"decision":true,"context":{},"evaluations":[{"decision":true}]}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.AccessForUser("user_1").On(workspace(c)).Can(ctx, "flow:run")
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/access/v1/evaluations",
			wantBody: `{"evaluations":[{"action":{"name":"flow:run"}}],` +
				`"resource":{"id":"ws_1","properties":{"organization_id":"org_1"},"type":"workspace"},` +
				`"subject":{"id":"user_1","type":"user"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, baseURL := newStubServer(t, tt.reply)
			c := newTestClient(t, baseURL, 1)

			if err := tt.call(t.Context(), c); err != nil {
				t.Fatalf("call: %v", err)
			}

			calls := stub.calls()
			if len(calls) != 1 {
				t.Fatalf("server saw %d requests, want exactly one", len(calls))
			}
			got := calls[0]
			if got.method != tt.wantMethod || got.path != tt.wantPath || got.query != tt.wantQuery {
				t.Errorf("request = %s %s?%s, want %s %s?%s",
					got.method, got.path, got.query, tt.wantMethod, tt.wantPath, tt.wantQuery)
			}
			if tt.wantBody != "" {
				assertJSONEqual(t, got.body, tt.wantBody)
			}
		})
	}
}

func TestCanAnswersTheDecision(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  bool
	}{
		{"allowed", `{"decision":true,"context":{},"evaluations":[{"decision":true}]}`, true},
		{"denied", `{"decision":false,"context":{"reason":"MISSING_PERMISSION"},"evaluations":[{"decision":false}]}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, baseURL := newStubServer(t, stubResponse{status: http.StatusOK, body: tt.reply})
			c := newTestClient(t, baseURL, 1)

			got, err := c.AccessForUser("user_1").On(c.Organization(testOrgID).Resource("workspace", "ws_1")).
				Can(t.Context(), "flow:run")

			if err != nil {
				t.Fatalf("Can: %v", err)
			}
			if got != tt.want {
				t.Errorf("Can = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceGetOfAnUnregisteredResourceIsNotFound(t *testing.T) {
	_, baseURL := newStubServer(t, stubResponse{
		status: http.StatusNotFound,
		body:   `{"errors":[{"code":"RESOURCE_NOT_FOUND","message":"not found"}]}`,
	})
	c := newTestClient(t, baseURL, 3)

	_, err := c.Organization(testOrgID).Resource("workspace", "ws_1").Get(t.Context())

	if !errors.Is(err, anchorsdk.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestResourceSearchAllReadsEveryPage(t *testing.T) {
	stub, baseURL := newStubServer(t,
		stubResponse{status: http.StatusOK, body: `{"count":1,"total":2,"next_cursor":"c1","items":[` + resourceBody + `]}`},
		stubResponse{status: http.StatusOK, body: `{"count":1,"total":2,"items":[` + resourceBody + `]}`},
	)
	c := newTestClient(t, baseURL, 1)

	seen := 0
	for _, err := range c.Organization(testOrgID).Resources().Search().Types("workspace").All(t.Context()) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		seen++
	}

	if seen != 2 {
		t.Errorf("read %d resources, want 2", seen)
	}
	calls := stub.calls()
	if len(calls) != 2 {
		t.Fatalf("server saw %d requests, want 2", len(calls))
	}
	assertJSONEqual(t, calls[1].body, `{"filter":{"resource_types":["workspace"]},"pagination":{"cursor":"c1"}}`)
}

func TestResourceCallsRetryWhatIsSafe(t *testing.T) {
	unavailable := stubResponse{status: http.StatusServiceUnavailable, body: `{"errors":[{"code":"UNAVAILABLE"}]}`}
	notFound := stubResponse{status: http.StatusNotFound, body: `{"errors":[{"code":"RESOURCE_NOT_FOUND"}]}`}
	inUse := stubResponse{status: http.StatusConflict, body: `{"errors":[{"code":"RESOURCE_IN_USE"}]}`}
	tests := []struct {
		name      string
		replies   []stubResponse
		call      func(context.Context, *anchorsdk.Resource) error
		wantErr   error
		wantCalls int
	}{
		{
			name:    "a put retried after an outage succeeds",
			replies: []stubResponse{unavailable, {status: http.StatusOK, body: resourceBody}},
			call: func(ctx context.Context, r *anchorsdk.Resource) error {
				_, err := r.Put("Default").Do(ctx)
				return err
			},
			wantCalls: 2,
		},
		{
			name:      "a delete whose first attempt failed and whose retry finds nothing succeeds",
			replies:   []stubResponse{unavailable, notFound},
			call:      func(ctx context.Context, r *anchorsdk.Resource) error { return r.Delete(ctx) },
			wantCalls: 2,
		},
		{
			name:      "a delete of a resource never registered is not found",
			replies:   []stubResponse{notFound},
			call:      func(ctx context.Context, r *anchorsdk.Resource) error { return r.Delete(ctx) },
			wantErr:   anchorsdk.ErrNotFound,
			wantCalls: 1,
		},
		{
			name:      "a refused delete is a conflict and is not retried",
			replies:   []stubResponse{inUse},
			call:      func(ctx context.Context, r *anchorsdk.Resource) error { return r.Delete(ctx) },
			wantErr:   anchorsdk.ErrConflict,
			wantCalls: 1,
		},
		{
			name:    "a remove whose first attempt failed and whose retry finds nothing succeeds",
			replies: []stubResponse{unavailable, notFound},
			call: func(ctx context.Context, r *anchorsdk.Resource) error {
				return r.Members().Remove(ctx, testUserID)
			},
			wantCalls: 2,
		},
		{
			name: "a put that assigns under an unregistered parent is a conflict and is not retried",
			replies: []stubResponse{{
				status: http.StatusConflict, body: `{"errors":[{"code":"PARENT_NOT_REGISTERED"}]}`,
			}},
			call: func(ctx context.Context, r *anchorsdk.Resource) error {
				_, err := r.Put("Default").Under("folder", "f_missing").Assign(ctx, testUserID, testRoleID)
				return err
			},
			wantErr:   anchorsdk.ErrConflict,
			wantCalls: 1,
		},
		{
			name:    "a put that assigns, retried after an outage, succeeds",
			replies: []stubResponse{unavailable, {status: http.StatusOK, body: resourceMemberBody}},
			call: func(ctx context.Context, r *anchorsdk.Resource) error {
				_, err := r.Put("Default").Assign(ctx, testUserID, testRoleID)
				return err
			},
			wantCalls: 2,
		},
		{
			name: "a refused assignment is invalid and is not retried",
			replies: []stubResponse{{
				status: http.StatusBadRequest, body: `{"errors":[{"code":"ROLE_SCOPE_MISMATCH"}]}`,
			}},
			call: func(ctx context.Context, r *anchorsdk.Resource) error {
				_, err := r.Members().Assign(ctx, testUserID, testRoleID)
				return err
			},
			wantErr:   anchorsdk.ErrInvalid,
			wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, baseURL := newStubServer(t, tt.replies...)
			c := newTestClient(t, baseURL, 3)

			err := tt.call(t.Context(), c.Organization(testOrgID).Resource("workspace", "ws_1"))

			if tt.wantErr == nil && err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := len(stub.calls()); got != tt.wantCalls {
				t.Errorf("server saw %d requests, want %d", got, tt.wantCalls)
			}
		})
	}
}

func TestResourceMemberSearchAllReadsEveryPage(t *testing.T) {
	stub, baseURL := newStubServer(t,
		stubResponse{status: http.StatusOK, body: `{"count":1,"total":2,"next_cursor":"c1","items":[` + resourceMemberBody + `]}`},
		stubResponse{status: http.StatusOK, body: `{"count":1,"total":2,"items":[` + resourceMemberBody + `]}`},
	)
	c := newTestClient(t, baseURL, 1)

	seen := 0
	for _, err := range c.Organization(testOrgID).Resource("workspace", "ws_1").Members().Search().All(t.Context()) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		seen++
	}

	if seen != 2 {
		t.Errorf("read %d members, want 2", seen)
	}
	assertJSONEqual(t, stub.calls()[1].body, `{"pagination":{"cursor":"c1"}}`)
}
