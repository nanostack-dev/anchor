package anchorsdk_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
)

const (
	// licenseBodyV1 and licenseBodyV2 are two states of the same organization's
	// license, distinguished by max_flows, so a test can tell a cached read
	// from a re-fetched one by which value comes back.
	licenseBodyV1 = `{"id":"olic_1","organization_id":"org_1","product_id":"prd_test","template_id":"tpl_1",` +
		`"instantiated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z",` +
		`"updated_at":"2026-01-01T00:00:00Z","values":{"max_flows":100,"premium_support":true}}`
	licenseBodyV2 = `{"id":"olic_1","organization_id":"org_1","product_id":"prd_test","template_id":"tpl_1",` +
		`"instantiated_at":"2026-01-01T00:00:00Z","created_at":"2026-01-01T00:00:00Z",` +
		`"updated_at":"2026-02-01T00:00:00Z","values":{"max_flows":500,"premium_support":true}}`

	licenseDiffBody = `{"organization_id":"org_1","template_id":"tpl_1","count":1,"differences":[` +
		`{"field":"max_flows","kind":"changed","license_value":500,"template_value":100}]}`

	usageObservationBody = `{"id":"uob_1","organization_id":"org_1","product_id":"prd_test","key":"max_flows",` +
		`"value":37,"observed_at":"2026-01-01T00:00:00Z"}`
)

// newLicenseTestClient is [newTestClient] plus a cache policy: License.Get is
// the only facade in this package that caches, so it is the only one that
// needs a knob the shared helper does not expose. Retries are fixed at a
// single attempt — retry behaviour itself is already covered by TestRetry,
// and every cache test below asserts an exact request count, which a retried
// attempt would throw off.
func newLicenseTestClient(t *testing.T, baseURL string, cache anchorsdk.LicenseCachePolicy) *anchorsdk.Client {
	t.Helper()

	c, err := anchorsdk.New(anchorsdk.Config{
		BaseURL:       baseURL,
		ProductID:     testProductID,
		ProductAPIKey: testAPIKey,
		Retry: anchorsdk.RetryPolicy{
			MaxAttempts: 1,
			BaseDelay:   time.Millisecond,
			MaxDelay:    time.Millisecond,
		},
		LicenseCache: cache,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return c
}

func TestLicenseRoutesAndRequests(t *testing.T) {
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		reply      stubResponse
		call       func(context.Context, *anchorsdk.Client) error
		wantMethod string
		wantPath   string
		wantBody   string // empty means no request body to check (a GET)
	}{
		{
			name:  "get",
			reply: stubResponse{status: http.StatusOK, body: licenseBodyV1},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().Get(ctx)
				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/products/prd_test/organizations/org_1/license",
		},
		{
			name:  "diff",
			reply: stubResponse{status: http.StatusOK, body: licenseDiffBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().Diff(ctx)
				return err
			},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/products/prd_test/organizations/org_1/license/diff",
		},
		{
			name:  "instantiate",
			reply: stubResponse{status: http.StatusCreated, body: licenseBodyV1},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().Instantiate(ctx, "tpl_1")
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/license",
			wantBody:   `{"template_id":"tpl_1"}`,
		},
		{
			name:  "adjust merges Set and Values into one request",
			reply: stubResponse{status: http.StatusOK, body: licenseBodyV2},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().Adjust().
					Set("max_flows", 500).
					Values(map[string]any{"region": "us-east-1"}).
					Do(ctx)
				return err
			},
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/products/prd_test/organizations/org_1/license",
			wantBody:   `{"values":{"max_flows":500,"region":"us-east-1"}}`,
		},
		{
			name:  "report usage as a gauge sends no window",
			reply: stubResponse{status: http.StatusCreated, body: usageObservationBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().ReportUsage("max_flows", 37).Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/license/usage",
			wantBody:   `{"key":"max_flows","value":37}`,
		},
		{
			name:  "report usage as a closed windowed counter",
			reply: stubResponse{status: http.StatusCreated, body: usageObservationBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().ReportUsage("monthly_runs", 412).
					From(from).
					To(to).
					Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/license/usage",
			wantBody:   `{"key":"monthly_runs","value":412,"from":"2026-01-01T00:00:00Z","to":"2026-02-01T00:00:00Z"}`,
		},
		{
			name:  "report usage with an open window sends from without to",
			reply: stubResponse{status: http.StatusCreated, body: usageObservationBody},
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().ReportUsage("monthly_runs", 412).
					From(from).
					Do(ctx)
				return err
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/products/prd_test/organizations/org_1/license/usage",
			wantBody:   `{"key":"monthly_runs","value":412,"from":"2026-01-01T00:00:00Z"}`,
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
			if tt.wantBody != "" {
				assertJSONEqual(t, calls[0].body, tt.wantBody)
			}
		})
	}
}

func TestLicenseGetDecodesEveryFieldValue(t *testing.T) {
	_, baseURL := newStubServer(t, stubResponse{status: http.StatusOK, body: licenseBodyV1})
	c := newTestClient(t, baseURL, 1)

	snap, err := c.Organization(testOrgID).License().Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if snap.Id != "olic_1" || snap.OrganizationId != testOrgID || snap.TemplateId != "tpl_1" {
		t.Errorf("identity fields = %+v, want olic_1/org_1/tpl_1", snap.OrganizationLicenseResponse)
	}
	if got := snap.Values["max_flows"]; got != float64(100) {
		t.Errorf("Values[max_flows] = %v (%T), want 100", got, got)
	}
	if got := snap.Values["premium_support"]; got != true {
		t.Errorf("Values[premium_support] = %v, want true", got)
	}
	if snap.Stale {
		t.Error("a live read must not be marked Stale")
	}
}

// TestLicenseErrorClassification exercises License.Get, whose only declared
// JSON status is 200 (see DESIGN.md: "Not every operation declares the same
// error statuses"), so every status below is undeclared for this route and
// must still classify correctly from the raw body the SDK re-parses itself.
func TestLicenseErrorClassification(t *testing.T) {
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
			wantIsNot: []error{anchorsdk.ErrNotFound},
		},
		{
			name:   "unauthorized",
			status: http.StatusUnauthorized,
			wantIs: []error{anchorsdk.ErrUnauthorized, anchorsdk.ErrPermanent},
		},
		{
			name:   "forbidden",
			status: http.StatusForbidden,
			wantIs: []error{anchorsdk.ErrForbidden, anchorsdk.ErrPermanent},
		},
		{
			name:   "not found",
			status: http.StatusNotFound,
			wantIs: []error{anchorsdk.ErrNotFound, anchorsdk.ErrPermanent},
		},
		{
			name:   "conflict",
			status: http.StatusConflict,
			wantIs: []error{anchorsdk.ErrConflict, anchorsdk.ErrPermanent},
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

			_, err := c.Organization(testOrgID).License().Get(t.Context())
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
			if len(apiErr.Details) != 1 || apiErr.Details[0].Code != "VALIDATION_ERROR" {
				t.Errorf("Details = %#v, want the one VALIDATION_ERROR detail carried by apiErrBody", apiErr.Details)
			}
		})
	}
}

// TestLicenseDeclaredStatusUnparseableBodyLosesStatusCode pins the trap
// documented in DESIGN.md and nanostack-dev/anchor#72: Adjust, Instantiate,
// and ReportUsage all declare a JSON body (BadRequest, an alias of
// ApiErrorResponse) for 400. The generated parser unmarshals that body before
// AdjustOrganizationLicenseWithResponse (etc.) ever returns, and when the
// body does not parse as JSON, the parser discards the response entirely —
// the SDK sees only a transport-shaped error, with the real 400 lost. Nothing
// on this side of the generated client can fix that; the point of this test
// is to prove the SDK's own test suite is not accidentally hiding it by
// always stubbing well-formed bodies.
func TestLicenseDeclaredStatusUnparseableBodyLosesStatusCode(t *testing.T) {
	_, baseURL := newStubServer(t, stubResponse{status: http.StatusBadRequest, body: "not valid json"})
	c := newTestClient(t, baseURL, 1)

	_, err := c.Organization(testOrgID).License().Adjust().Set("max_flows", 500).Do(t.Context())
	if err == nil {
		t.Fatal("want an error, got nil")
	}

	var apiErr *anchorsdk.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("errors.As did not yield *anchorsdk.Error, got %T", err)
	}
	if apiErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 — the generated parser discarded it on this path", apiErr.StatusCode)
	}
	if errors.Is(err, anchorsdk.ErrPermanent) {
		t.Error("a lost status code must not be misreported as permanent — " +
			"that is exactly what makes this trap dangerous: a real 400 would be retried")
	}
}

// TestLicenseDeclaredStatusRealPayloadClassifiesCorrectly is the contrasting
// case for the trap above: the same declared-400 routes, given the real
// ApiErrorResponse-shaped body Anchor actually sends, classify the same as
// every other facade's error handling.
func TestLicenseDeclaredStatusRealPayloadClassifiesCorrectly(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *anchorsdk.Client) error
	}{
		{
			name: "adjust",
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().Adjust().Set("max_flows", 500).Do(ctx)
				return err
			},
		},
		{
			name: "instantiate",
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().Instantiate(ctx, "tpl_1")
				return err
			},
		},
		{
			name: "report usage",
			call: func(ctx context.Context, c *anchorsdk.Client) error {
				_, err := c.Organization(testOrgID).License().ReportUsage("max_flows", 37).Do(ctx)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, baseURL := newStubServer(t, stubResponse{status: http.StatusBadRequest, body: apiErrBody})
			c := newTestClient(t, baseURL, 1)

			err := tt.call(t.Context(), c)
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			if !errors.Is(err, anchorsdk.ErrInvalid) || !errors.Is(err, anchorsdk.ErrPermanent) {
				t.Errorf("want ErrInvalid and ErrPermanent with a real payload, got %v", err)
			}

			var apiErr *anchorsdk.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("errors.As did not yield *anchorsdk.Error, got %T", err)
			}
			if apiErr.StatusCode != http.StatusBadRequest {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
			}
			if len(apiErr.Details) != 1 || apiErr.Details[0].Code != "VALIDATION_ERROR" {
				t.Errorf("Details = %#v, want the one VALIDATION_ERROR detail carried by apiErrBody", apiErr.Details)
			}
		})
	}
}

func TestLicenseGetColdCacheReturnsError(t *testing.T) {
	_, baseURL := newStubServer(t, stubResponse{status: http.StatusInternalServerError, body: apiErrBody})
	c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{})

	if _, err := c.Organization(testOrgID).License().Get(t.Context()); err == nil {
		t.Fatal("want an error on a cold cache with no prior value to fall back to, got nil")
	}
}

func TestLicenseGetFailsOpenOnTransientRefreshFailure(t *testing.T) {
	stub, baseURL := newStubServer(t,
		stubResponse{status: http.StatusOK, body: licenseBodyV1},
		stubResponse{status: http.StatusInternalServerError, body: apiErrBody},
	)
	c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{TTL: 10 * time.Millisecond})
	org := c.Organization(testOrgID)

	first, err := org.License().Get(t.Context())
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if first.Stale {
		t.Error("the first, live read must not be marked Stale")
	}

	time.Sleep(50 * time.Millisecond) // outlive the 10ms TTL

	second, err := org.License().Get(t.Context())
	if err != nil {
		t.Fatalf("Get after a transient refresh failure: want the stale cache, got error %v", err)
	}
	if !second.Stale {
		t.Error("Stale = false, want true — the live refresh failed and this must be the cached value")
	}
	if second.Values["max_flows"] != first.Values["max_flows"] {
		t.Errorf("stale Values[max_flows] = %v, want the first read's %v",
			second.Values["max_flows"], first.Values["max_flows"])
	}
	if got := len(stub.calls()); got != 2 {
		t.Errorf("server saw %d requests, want 2 (the live read, then the failed refresh attempt)", got)
	}
}

func TestLicenseGetStrictModeFailsClosed(t *testing.T) {
	_, baseURL := newStubServer(t,
		stubResponse{status: http.StatusOK, body: licenseBodyV1},
		stubResponse{status: http.StatusInternalServerError, body: apiErrBody},
	)
	c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{
		TTL:    10 * time.Millisecond,
		Strict: true,
	})
	org := c.Organization(testOrgID)

	if _, err := org.License().Get(t.Context()); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, err := org.License().Get(t.Context())
	if err == nil {
		t.Fatal("want the refresh error under Strict, got nil (silently served stale data)")
	}
	if errors.Is(err, anchorsdk.ErrPermanent) {
		t.Error("the underlying failure is a 500, not permanent — Strict must not relabel it")
	}
}

func TestLicenseGetPermanentFailureNeverFailsOpen(t *testing.T) {
	_, baseURL := newStubServer(t,
		stubResponse{status: http.StatusOK, body: licenseBodyV1},
		stubResponse{status: http.StatusForbidden, body: apiErrBody},
	)
	// Strict is left false (the default) on purpose: even fail-open must not
	// paper over a permanent failure, so this must behave the same as Strict.
	c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{TTL: 10 * time.Millisecond})
	org := c.Organization(testOrgID)

	if _, err := org.License().Get(t.Context()); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, err := org.License().Get(t.Context())
	if err == nil {
		t.Fatal("want the 403 surfaced, got nil — a permanent failure must never fail open")
	}
	if !errors.Is(err, anchorsdk.ErrForbidden) {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestLicenseGetFreshCacheAvoidsNetworkCall(t *testing.T) {
	stub, baseURL := newStubServer(t, stubResponse{status: http.StatusOK, body: licenseBodyV1})
	c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{TTL: time.Minute})
	org := c.Organization(testOrgID)

	if _, err := org.License().Get(t.Context()); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	second, err := org.License().Get(t.Context())
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if second.Stale {
		t.Error("a fresh cache hit must not be marked Stale")
	}
	if got := len(stub.calls()); got != 1 {
		t.Errorf("server saw %d requests, want exactly 1 (the second Get should be served from cache)", got)
	}
}

func TestLicenseWritesPopulateCache(t *testing.T) {
	t.Run("instantiate", func(t *testing.T) {
		stub, baseURL := newStubServer(t, stubResponse{status: http.StatusCreated, body: licenseBodyV1})
		c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{TTL: time.Minute})
		org := c.Organization(testOrgID)

		if _, err := org.License().Instantiate(t.Context(), "tpl_1"); err != nil {
			t.Fatalf("Instantiate: %v", err)
		}

		snap, err := org.License().Get(t.Context())
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if snap.Stale {
			t.Error("Stale = true right after Instantiate, want false")
		}
		if got := len(stub.calls()); got != 1 {
			t.Errorf("server saw %d requests, want exactly 1 (Get must be served from the write-through cache)", got)
		}
	})

	t.Run("adjust", func(t *testing.T) {
		stub, baseURL := newStubServer(t, stubResponse{status: http.StatusOK, body: licenseBodyV2})
		c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{TTL: time.Minute})
		org := c.Organization(testOrgID)

		if _, err := org.License().Adjust().Set("max_flows", 500).Do(t.Context()); err != nil {
			t.Fatalf("Adjust: %v", err)
		}

		snap, err := org.License().Get(t.Context())
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got := snap.Values["max_flows"]; got != float64(500) {
			t.Errorf("Values[max_flows] = %v, want 500 (Adjust's own response)", got)
		}
		if got := len(stub.calls()); got != 1 {
			t.Errorf("server saw %d requests, want exactly 1 (Get must be served from the write-through cache)", got)
		}
	})
}

func TestLicenseInvalidateForcesRefetch(t *testing.T) {
	stub, baseURL := newStubServer(t,
		stubResponse{status: http.StatusOK, body: licenseBodyV1},
		stubResponse{status: http.StatusOK, body: licenseBodyV2},
	)
	c := newLicenseTestClient(t, baseURL, anchorsdk.LicenseCachePolicy{TTL: time.Minute})
	org := c.Organization(testOrgID)

	if _, err := org.License().Get(t.Context()); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	org.License().Invalidate()

	snap, err := org.License().Get(t.Context())
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if got := snap.Values["max_flows"]; got != float64(500) {
		t.Errorf("Values[max_flows] = %v, want 500 from the re-fetched second body", got)
	}
	if got := len(stub.calls()); got != 2 {
		t.Errorf("server saw %d requests, want 2 (Invalidate must force a live refetch)", got)
	}
}
