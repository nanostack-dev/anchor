package anchorsdk_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
)

const (
	testProductID = "prd_test"
	testAPIKey    = "anchor_prd_apikey_secret"
	testOrgID     = "org_1"
	testUserID    = "pusr_1"
	testRoleID    = "prole_1"

	authHeader = "X-Product-Api-Key"

	orgBody  = `{"id":"org_1","name":"Acme"}`
	listBody = `{"count":0,"items":[],"total":0}`

	// apiErrBody is the ApiErrorResponse Anchor returns on failure. Error stubs
	// must carry it: for a status the spec declares a JSON body for, the
	// generated client discards the entire response when that body will not
	// parse, leaving the SDK with a transport error and no status code.
	apiErrBody = `{"errors":[{"code":"VALIDATION_ERROR","message":"name is required","field":"name"}]}`
)

// stubResponse is one scripted reply. An empty contentType means JSON.
type stubResponse struct {
	status      int
	body        string
	contentType string
}

// recordedRequest is what the SDK actually put on the wire.
type recordedRequest struct {
	method string
	path   string
	body   []byte
	header http.Header
}

// stubServer replies from a script, repeating the last entry once the script is
// exhausted, and records every request it received.
type stubServer struct {
	mu        sync.Mutex
	responses []stubResponse
	requests  []recordedRequest
}

// newStubServer starts a server scripted with the given replies. It is closed
// when the test ends.
func newStubServer(t *testing.T, responses ...stubResponse) (*stubServer, string) {
	t.Helper()

	stub := &stubServer{responses: responses}
	srv := httptest.NewServer(stub)
	t.Cleanup(srv.Close)

	return stub, srv.URL
}

func (s *stubServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	index := min(len(s.requests), len(s.responses)-1)
	s.requests = append(s.requests, recordedRequest{
		method: r.Method,
		path:   r.URL.Path,
		body:   body,
		header: r.Header.Clone(),
	})
	s.mu.Unlock()

	reply := stubResponse{status: http.StatusOK}
	if index >= 0 {
		reply = s.responses[index]
	}

	contentType := reply.contentType
	if contentType == "" {
		contentType = "application/json"
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(reply.status)
	_, _ = io.WriteString(w, reply.body)
}

// calls returns every request received so far.
func (s *stubServer) calls() []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]recordedRequest(nil), s.requests...)
}

// newTestClient builds a client pointed at baseURL with a retry policy fast
// enough that the backoff does not slow the suite down.
func newTestClient(t *testing.T, baseURL string, attempts int) *anchorsdk.Client {
	t.Helper()

	c, err := anchorsdk.New(anchorsdk.Config{
		BaseURL:       baseURL,
		ProductID:     testProductID,
		ProductAPIKey: testAPIKey,
		Retry: anchorsdk.RetryPolicy{
			MaxAttempts: attempts,
			BaseDelay:   time.Millisecond,
			MaxDelay:    time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return c
}

// sendRecord renders an email send record with the given persisted status.
func sendRecord(status string) string {
	return `{"id":"esnd_1","status":"` + status + `","to_address":"a@b.com","subject":"hi","from_address":"x@y.com"}`
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantIs    []error
		wantIsNot []error
	}{
		{
			name:      "bad request is invalid and permanent",
			status:    http.StatusBadRequest,
			wantIs:    []error{anchorsdk.ErrInvalid, anchorsdk.ErrPermanent},
			wantIsNot: []error{anchorsdk.ErrNotFound, anchorsdk.ErrConflict},
		},
		{
			name:      "unprocessable entity is invalid",
			status:    http.StatusUnprocessableEntity,
			wantIs:    []error{anchorsdk.ErrInvalid, anchorsdk.ErrPermanent},
			wantIsNot: []error{anchorsdk.ErrForbidden},
		},
		{
			name:      "unauthorized",
			status:    http.StatusUnauthorized,
			wantIs:    []error{anchorsdk.ErrUnauthorized, anchorsdk.ErrPermanent},
			wantIsNot: []error{anchorsdk.ErrForbidden},
		},
		{
			name:      "forbidden",
			status:    http.StatusForbidden,
			wantIs:    []error{anchorsdk.ErrForbidden, anchorsdk.ErrPermanent},
			wantIsNot: []error{anchorsdk.ErrUnauthorized},
		},
		{
			name:      "not found",
			status:    http.StatusNotFound,
			wantIs:    []error{anchorsdk.ErrNotFound, anchorsdk.ErrPermanent},
			wantIsNot: []error{anchorsdk.ErrInvalid},
		},
		{
			name:      "conflict",
			status:    http.StatusConflict,
			wantIs:    []error{anchorsdk.ErrConflict, anchorsdk.ErrPermanent},
			wantIsNot: []error{anchorsdk.ErrNotFound},
		},
		{
			name:      "too many requests is the one retryable 4xx",
			status:    http.StatusTooManyRequests,
			wantIsNot: []error{anchorsdk.ErrPermanent},
		},
		{
			name:      "server error is not permanent",
			status:    http.StatusInternalServerError,
			wantIsNot: []error{anchorsdk.ErrPermanent, anchorsdk.ErrInvalid},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, baseURL := newStubServer(t, stubResponse{status: tt.status, body: apiErrBody})
			c := newTestClient(t, baseURL, 1)

			_, err := c.Organizations().Get(t.Context(), testOrgID)
			if err == nil {
				t.Fatal("want an error, got nil")
			}

			for _, sentinel := range tt.wantIs {
				if !errors.Is(err, sentinel) {
					t.Errorf("errors.Is(err, %v) = false, want true (err: %v)", sentinel, err)
				}
			}
			for _, sentinel := range tt.wantIsNot {
				if errors.Is(err, sentinel) {
					t.Errorf("errors.Is(err, %v) = true, want false (err: %v)", sentinel, err)
				}
			}

			var apiErr *anchorsdk.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("errors.As did not yield *anchorsdk.Error, got %T", err)
			}
			if apiErr.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.status)
			}
			if apiErr.Op != "Organizations.Get" {
				t.Errorf("Op = %q, want %q", apiErr.Op, "Organizations.Get")
			}

			if len(apiErr.Details) != 1 {
				t.Fatalf("Details = %#v, want exactly one entry", apiErr.Details)
			}
			got := apiErr.Details[0]
			want := anchorsdk.Detail{Code: "VALIDATION_ERROR", Message: "name is required", Field: "name"}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Details[0] = %#v, want %#v", got, want)
			}
		})
	}
}

func TestRetry(t *testing.T) {
	const attempts = 3

	ok := stubResponse{status: http.StatusOK, body: orgBody}

	tests := []struct {
		name         string
		responses    []stubResponse
		wantErr      bool
		wantRequests int
	}{
		{
			name:         "succeeds on the first attempt",
			responses:    []stubResponse{ok},
			wantRequests: 1,
		},
		{
			name:         "retries a server error then succeeds",
			responses:    []stubResponse{{status: http.StatusInternalServerError, body: apiErrBody}, ok},
			wantRequests: 2,
		},
		{
			name:         "retries a rate limit then succeeds",
			responses:    []stubResponse{{status: http.StatusTooManyRequests, body: apiErrBody}, ok},
			wantRequests: 2,
		},
		{
			name:         "gives up after the configured attempts",
			responses:    []stubResponse{{status: http.StatusServiceUnavailable, body: apiErrBody}},
			wantErr:      true,
			wantRequests: attempts,
		},
		{
			name:         "does not retry a not found",
			responses:    []stubResponse{{status: http.StatusNotFound, body: apiErrBody}},
			wantErr:      true,
			wantRequests: 1,
		},
		{
			name:         "does not retry a bad request",
			responses:    []stubResponse{{status: http.StatusBadRequest, body: apiErrBody}},
			wantErr:      true,
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, baseURL := newStubServer(t, tt.responses...)
			c := newTestClient(t, baseURL, attempts)

			org, err := c.Organizations().Get(t.Context(), testOrgID)

			switch {
			case tt.wantErr && err == nil:
				t.Fatal("want an error, got nil")
			case !tt.wantErr && err != nil:
				t.Fatalf("want success, got %v", err)
			case !tt.wantErr && org.Id != testOrgID:
				t.Errorf("org.Id = %q, want %q", org.Id, testOrgID)
			}

			if got := len(stub.calls()); got != tt.wantRequests {
				t.Errorf("server saw %d requests, want %d", got, tt.wantRequests)
			}
		})
	}
}

func TestRetryStopsOnCancelledContext(t *testing.T) {
	stub, baseURL := newStubServer(t, stubResponse{status: http.StatusInternalServerError})
	c := newTestClient(t, baseURL, 3)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := c.Organizations().Get(ctx, testOrgID); err == nil {
		t.Fatal("want an error, got nil")
	}
	if got := len(stub.calls()); got != 0 {
		t.Errorf("server saw %d requests, want 0", got)
	}
}

func TestTransportErrorIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	baseURL := srv.URL
	srv.Close() // Nothing is listening, so every attempt fails in transport.

	c := newTestClient(t, baseURL, 2)

	_, err := c.Organizations().Get(t.Context(), testOrgID)
	if err == nil {
		t.Fatal("want an error, got nil")
	}
	if errors.Is(err, anchorsdk.ErrPermanent) {
		t.Errorf("a transport failure must stay retryable, got %v", err)
	}

	var apiErr *anchorsdk.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("errors.As did not yield *anchorsdk.Error, got %T", err)
	}
	if apiErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 for a request that never completed", apiErr.StatusCode)
	}
	if apiErr.Unwrap() == nil {
		t.Error("Unwrap() = nil, want the underlying transport error")
	}
}

func TestEmailSend(t *testing.T) {
	const attempts = 3

	tests := []struct {
		name          string
		responses     []stubResponse
		wantErr       bool
		wantPermanent bool
		wantRequests  int
	}{
		{
			name:         "sent",
			responses:    []stubResponse{{status: http.StatusCreated, body: sendRecord("SENT")}},
			wantRequests: 1,
		},
		{
			name:         "queued counts as accepted",
			responses:    []stubResponse{{status: http.StatusCreated, body: sendRecord("QUEUED")}},
			wantRequests: 1,
		},
		{
			name:          "a created record with status FAILED is permanent",
			responses:     []stubResponse{{status: http.StatusCreated, body: sendRecord("FAILED")}},
			wantErr:       true,
			wantPermanent: true,
			wantRequests:  1,
		},
		{
			name:          "a bad request is permanent",
			responses:     []stubResponse{{status: http.StatusBadRequest, body: apiErrBody}},
			wantErr:       true,
			wantPermanent: true,
			wantRequests:  1,
		},
		{
			name: "a server error is retried",
			responses: []stubResponse{
				{status: http.StatusInternalServerError, body: apiErrBody},
				{status: http.StatusCreated, body: sendRecord("SENT")},
			},
			wantRequests: 2,
		},
		{
			name:          "an undecodable success is permanent",
			responses:     []stubResponse{{status: http.StatusCreated, body: "accepted", contentType: "text/plain"}},
			wantErr:       true,
			wantPermanent: true,
			wantRequests:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, baseURL := newStubServer(t, tt.responses...)
			c := newTestClient(t, baseURL, attempts)

			err := c.Email().Template("welcome").To("a@b.com").Send(t.Context())

			switch {
			case tt.wantErr && err == nil:
				t.Fatal("want an error, got nil")
			case !tt.wantErr && err != nil:
				t.Fatalf("want success, got %v", err)
			}

			if tt.wantErr && errors.Is(err, anchorsdk.ErrPermanent) != tt.wantPermanent {
				t.Errorf("errors.Is(err, ErrPermanent) = %t, want %t (err: %v)",
					!tt.wantPermanent, tt.wantPermanent, err)
			}
			if got := len(stub.calls()); got != tt.wantRequests {
				t.Errorf("server saw %d requests, want %d", got, tt.wantRequests)
			}
		})
	}
}

func TestBuildersProduceExpectedRequests(t *testing.T) {
	expiry := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name       string
		reply      stubResponse
		call       func(context.Context, *anchorsdk.Client) error
		wantMethod string
		wantPath   string
		wantBody   string
	}{
		{
			name:  "email send",
			reply: stubResponse{status: http.StatusCreated, body: sendRecord("SENT")},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				return c.Email().
					Template("welcome").
					To("a@b.com").
					ToName("Bob").
					Dedupe("signup-1").
					Var("plan", "pro").
					Vars(map[string]any{"seats": 3}).
					Draft().
					Send(ctx)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/email/sends",
			wantBody: `{"template_slug":"welcome","to_address":"a@b.com","to_name":"Bob",` +
				`"dedupe_key":"signup-1","use_draft":true,"variables":{"plan":"pro","seats":3}}`,
		},
		{
			name:  "organization create",
			reply: stubResponse{status: http.StatusCreated, body: orgBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organizations().Create("Acme").
					Description("Leading provider").
					Meta("region", "us-east-1").
					Metadata(map[string]any{"sla_level": "gold"}).
					FoundingMember(testUserID, testRoleID).
					Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations",
			wantBody: `{"name":"Acme","description":"Leading provider",` +
				`"metadata":{"region":"us-east-1","sla_level":"gold"},` +
				`"founding_member":{"product_user_id":"pusr_1","role_id":"prole_1"}}`,
		},
		{
			name:  "organization search",
			reply: stubResponse{status: http.StatusOK, body: listBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organizations().Search().
					Query("acme").
					Names("Acme").
					Limit(20).
					Offset(40).
					SortBy(nanoclient.ProductOrganizationSearchRequestSortByName, nanoclient.DESC).
					Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/search",
			wantBody: `{"full_text_search":"acme","filter":{"names":["Acme"]},` +
				`"pagination":{"limit":20,"offset":40},"sort_by":"name","sort_direction":"DESC"}`,
		},
		{
			name:  "member add",
			reply: stubResponse{status: http.StatusCreated, body: `{"product_user_id":"pusr_1"}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).Members().Add(ctx, testUserID, testRoleID)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/members",
			wantBody:   `{"product_user_id":"pusr_1","role_id":"prole_1"}`,
		},
		{
			name:  "workspace create",
			reply: stubResponse{status: http.StatusCreated, body: `{"id":"wsp_1","name":"Production"}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).Workspaces().
					Create("Production").
					Description("Production workloads").
					Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/workspaces",
			wantBody:   `{"name":"Production","description":"Production workloads"}`,
		},
		{
			name:  "api key create",
			reply: stubResponse{status: http.StatusCreated, body: `{"id":"oak_1","value":"anchor_org_apikey_x"}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).APIKeys().
					Create("ci").
					Description("CI runner").
					Permissions("flow:read", "flow:execute").
					ExpiresAt(expiry).
					Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/api-keys",
			wantBody: `{"name":"ci","description":"CI runner",` +
				`"permissions":["flow:read","flow:execute"],"expires_at":"2030-01-02T03:04:05Z"}`,
		},
		{
			name:  "product user create",
			reply: stubResponse{status: http.StatusCreated, body: `{"id":"pusr_1","email":"a@b.com"}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Users().Create("a@b.com").Name("New User").Status(nanoclient.Active).Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/product-users",
			wantBody:   `{"email":"a@b.com","name":"New User","status":"active"}`,
		},
		{
			name:  "introspect",
			reply: stubResponse{status: http.StatusOK, body: `{"api_key":{},"permissions":[],"missing_privileges":[]}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Introspect(ctx, "anchor_org_apikey_x", "flow:read")
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/auth/introspect",
			wantBody:   `{"api_key":"anchor_org_apikey_x","required_scopes":["flow:read"]}`,
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
			if got.method != tt.wantMethod {
				t.Errorf("method = %s, want %s", got.method, tt.wantMethod)
			}
			if got.path != tt.wantPath {
				t.Errorf("path = %s, want %s", got.path, tt.wantPath)
			}
			if key := got.header.Get(authHeader); key != testAPIKey {
				t.Errorf("%s = %q, want %q", authHeader, key, testAPIKey)
			}
			assertJSONEqual(t, got.body, tt.wantBody)
		})
	}
}

func TestReadOperationsUseExpectedRoutes(t *testing.T) {
	tests := []struct {
		name       string
		reply      stubResponse
		call       func(context.Context, *anchorsdk.Client) error
		wantMethod string
		wantPath   string
	}{
		{
			name:  "organization get",
			reply: stubResponse{status: http.StatusOK, body: orgBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organizations().Get(ctx, testOrgID)
				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/products/prd_test/organizations/org_1",
		},
		{
			name:  "organization delete",
			reply: stubResponse{status: http.StatusNoContent},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				return c.Organizations().Delete(ctx, testOrgID)
			},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/products/prd_test/organizations/org_1",
		},
		{
			name:  "member list",
			reply: stubResponse{status: http.StatusOK, body: listBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).Members().List(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/members/search",
		},
		{
			name:  "member remove",
			reply: stubResponse{status: http.StatusNoContent},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				return c.Organization(testOrgID).Members().Remove(ctx, testUserID)
			},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/products/prd_test/organizations/org_1/members/pusr_1",
		},
		{
			name:  "member get with role permissions",
			reply: stubResponse{status: http.StatusOK, body: `{"product_user_id":"pusr_1"}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).Members().GetWithRolePermissions(ctx, testUserID)
				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/products/prd_test/organizations/org_1/members/pusr_1",
		},
		{
			name:  "api key list",
			reply: stubResponse{status: http.StatusOK, body: listBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).APIKeys().List(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/api-keys/search",
		},
		{
			name:  "api key validate",
			reply: stubResponse{status: http.StatusOK, body: `{"api_key":{},"permissions":[],"missing_privileges":[]}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).APIKeys().Validate(ctx, "anchor_org_apikey_x", "flow:read")
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/api-keys/validate",
		},
		{
			name:  "workspace delete",
			reply: stubResponse{status: http.StatusNoContent},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				return c.Organization(testOrgID).Workspaces().Delete(ctx, "wsp_1")
			},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/products/prd_test/organizations/org_1/workspaces/wsp_1",
		},
		{
			name:  "user organizations",
			reply: stubResponse{status: http.StatusOK, body: `{"items":[]}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Users().Organizations(ctx, testUserID)
				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/products/prd_test/product-users/pusr_1/organizations",
		},
		{
			name:  "user organization",
			reply: stubResponse{status: http.StatusOK, body: `{"joined_at":"2026-01-01T00:00:00Z"}`},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Users().Organization(ctx, testUserID, testOrgID)
				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/products/prd_test/product-users/pusr_1/organizations/org_1",
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
			if calls[0].method != tt.wantMethod {
				t.Errorf("method = %s, want %s", calls[0].method, tt.wantMethod)
			}
			if calls[0].path != tt.wantPath {
				t.Errorf("path = %s, want %s", calls[0].path, tt.wantPath)
			}
		})
	}
}

func TestNewValidatesConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     anchorsdk.Config
		wantErr bool
	}{
		{
			name:    "missing base url",
			cfg:     anchorsdk.Config{ProductID: testProductID, ProductAPIKey: testAPIKey},
			wantErr: true,
		},
		{
			name:    "missing product id",
			cfg:     anchorsdk.Config{BaseURL: "https://anchor.test", ProductAPIKey: testAPIKey},
			wantErr: true,
		},
		{
			name:    "missing product api key",
			cfg:     anchorsdk.Config{BaseURL: "https://anchor.test", ProductID: testProductID},
			wantErr: true,
		},
		{
			name: "complete",
			cfg: anchorsdk.Config{
				BaseURL:       "https://anchor.test",
				ProductID:     testProductID,
				ProductAPIKey: testAPIKey,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := anchorsdk.New(tt.cfg)

			switch {
			case tt.wantErr && err == nil:
				t.Fatal("want an error, got nil")
			case !tt.wantErr && err != nil:
				t.Fatalf("want success, got %v", err)
			case tt.wantErr:
				return
			}

			if c.ProductID() != testProductID {
				t.Errorf("ProductID() = %q, want %q", c.ProductID(), testProductID)
			}
			if c.Raw() == nil {
				t.Error("Raw() = nil, want the generated client")
			}
			if c.Organization(testOrgID).ID() != testOrgID {
				t.Errorf("Organization().ID() = %q, want %q", c.Organization(testOrgID).ID(), testOrgID)
			}
		})
	}
}

// assertJSONEqual compares two JSON documents by value, ignoring key order.
func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()

	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("request body is not JSON: %v (%s)", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("expected body is not JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("request body =\n\t%s\nwant\n\t%s", got, want)
	}
}
