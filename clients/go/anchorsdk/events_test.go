package anchorsdk_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
)

func TestWebhookHandlerOrganizationCreated(t *testing.T) {
	t.Parallel()

	secret := mustSigningSecret(t)
	var got anchorsdk.OrganizationCreated
	handler, err := anchorsdk.Events(secret)
	if err != nil {
		t.Fatal(err)
	}
	handler.OrganizationCreated(func(_ context.Context, event anchorsdk.OrganizationCreated) error {
		got = event
		return nil
	})

	body := []byte(
		`{"type":"organization.created","timestamp":"2026-09-01T00:00:00.000000000Z","data":{"organization_id":"org_1"}}`,
	)
	now := time.Now()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/anchor", bytes.NewReader(body))
	req.Header.Set("Webhook-Id", "pevt_test")
	req.Header.Set("Webhook-Timestamp", strconv.FormatInt(now.Unix(), 10))
	req.Header.Set("Webhook-Signature", signForTest(t, secret, "pevt_test", now, body))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d", recorder.Code)
	}
	if got.OrganizationID != "org_1" {
		t.Fatalf("organization id: %q", got.OrganizationID)
	}
}

func TestWebhookHandlerRejectsTamperedBody(t *testing.T) {
	t.Parallel()

	secret := mustSigningSecret(t)
	handler, err := anchorsdk.Events(secret)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(
		`{"type":"organization.created","timestamp":"2026-09-01T00:00:00Z","data":{"organization_id":"org_1"}}`,
	)
	now := time.Now()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"tampered":true}`)))
	req.Header.Set("Webhook-Id", "pevt_test")
	req.Header.Set("Webhook-Timestamp", strconv.FormatInt(now.Unix(), 10))
	req.Header.Set("Webhook-Signature", signForTest(t, secret, "pevt_test", now, body))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status %d", recorder.Code)
	}
}

func TestWebhookHandlerProductRoleCreated(t *testing.T) {
	t.Parallel()

	secret := mustSigningSecret(t)
	var got anchorsdk.ProductRoleCreated
	handler, err := anchorsdk.Events(secret)
	if err != nil {
		t.Fatal(err)
	}
	handler.ProductRoleCreated(func(_ context.Context, event anchorsdk.ProductRoleCreated) error {
		got = event
		return nil
	})

	body := []byte(
		`{"type":"product.role.created","timestamp":"2026-09-01T00:00:00.000000000Z","data":{"role_id":"product_role_1"}}`,
	)
	now := time.Now()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/anchor", bytes.NewReader(body))
	req.Header.Set("Webhook-Id", "pevt_test")
	req.Header.Set("Webhook-Timestamp", strconv.FormatInt(now.Unix(), 10))
	req.Header.Set("Webhook-Signature", signForTest(t, secret, "pevt_test", now, body))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d", recorder.Code)
	}
	if got.RoleID != "product_role_1" {
		t.Fatalf("role id: %q", got.RoleID)
	}
}

func TestEventTypesCatalog(t *testing.T) {
	t.Parallel()
	if len(anchorsdk.EventTypes()) == 0 {
		t.Fatal("catalog must not be empty")
	}
}

func mustSigningSecret(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return "whsec_" + base64.StdEncoding.EncodeToString(key)
}

func signForTest(t *testing.T, secret, msgID string, timestamp time.Time, body []byte) string {
	t.Helper()
	signature, err := anchorsdk.SignWebhookForTest(secret, msgID, timestamp, body)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}
