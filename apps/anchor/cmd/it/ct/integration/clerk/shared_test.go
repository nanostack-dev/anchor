package ct_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	dslfactory "anchor/cmd/it/shared/dsl/factory"
)

const clerkTestWebhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

func clerkOfficialUserCreatedPayload() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"backup_code_enabled":             false,
			"banned":                          false,
			"create_organization_enabled":     true,
			"create_organizations_limit":      nil,
			"created_at":                      float64(1716883200000),
			"delete_self_enabled":             true,
			"email_addresses":                 []interface{}{},
			"enterprise_accounts":             []interface{}{},
			"external_accounts":               []interface{}{},
			"external_id":                     nil,
			"first_name":                      "John",
			"has_image":                       true,
			"id":                              "user_2g7np7Hrk0SN6kj5EDMLDaKNL0S",
			"image_url":                       "https://img.clerk.com/xxxxxx",
			"last_active_at":                  float64(1716883200000),
			"last_name":                       "Doe",
			"last_sign_in_at":                 float64(1716883200000),
			"legal_accepted_at":               float64(1716883200000),
			"locked":                          false,
			"lockout_expires_in_seconds":      nil,
			"mfa_disabled_at":                 nil,
			"mfa_enabled_at":                  nil,
			"object":                          "user",
			"passkeys":                        []interface{}{},
			"password_enabled":                true,
			"phone_numbers":                   []interface{}{},
			"primary_email_address_id":        "idn_2g7np7Hrk0SN6kj5EDMLDaKNL0S",
			"primary_phone_number_id":         nil,
			"primary_web3_wallet_id":          nil,
			"private_metadata":                nil,
			"profile_image_url":               "https://img.clerk.com/xxxxxx",
			"public_metadata":                 map[string]interface{}{},
			"saml_accounts":                   []interface{}{},
			"totp_enabled":                    false,
			"two_factor_enabled":              false,
			"unsafe_metadata":                 map[string]interface{}{},
			"updated_at":                      float64(1716883200000),
			"username":                        nil,
			"verification_attempts_remaining": nil,
			"web3_wallets":                    []interface{}{},
		},
		"event_attributes": map[string]interface{}{
			"http_request": map[string]interface{}{
				"client_ip":  "192.168.1.100",
				"user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
			},
		},
		"instance_id": "ins_2g7np7Hrk0SN6kj5EDMLDaKNL0S",
		"object":      "event",
		"timestamp":   float64(1716883200),
		"type":        "user.created",
	}
}

func clerkOfficialUserUpdatedPayload() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"birthday":   "",
			"created_at": float64(1654012591514),
			"email_addresses": []interface{}{
				map[string]interface{}{
					"email_address": "example@example.org",
					"id":            "idn_29w83yL7CwVlJXylYLxcslromF1",
					"linked_to":     []interface{}{},
					"object":        "email_address",
					"reserved":      true,
					"verification": map[string]interface{}{
						"attempts":  nil,
						"expire_at": nil,
						"status":    "verified",
						"strategy":  "admin",
					},
				},
			},
			"external_accounts":        []interface{}{},
			"external_id":              nil,
			"first_name":               "Example",
			"gender":                   "",
			"id":                       "user_29w83sxmDNGwOuEthce5gg56FcC",
			"image_url":                "https://img.clerk.com/xxxxxx",
			"last_name":                nil,
			"last_sign_in_at":          nil,
			"object":                   "user",
			"password_enabled":         true,
			"phone_numbers":            []interface{}{},
			"primary_email_address_id": "idn_29w83yL7CwVlJXylYLxcslromF1",
			"primary_phone_number_id":  nil,
			"primary_web3_wallet_id":   nil,
			"private_metadata":         map[string]interface{}{},
			"profile_image_url":        "https://www.gravatar.com/avatar?d=mp",
			"public_metadata":          map[string]interface{}{},
			"two_factor_enabled":       false,
			"unsafe_metadata":          map[string]interface{}{},
			"updated_at":               float64(1654012824306),
			"username":                 nil,
			"web3_wallets":             []interface{}{},
		},
		"event_attributes": map[string]interface{}{
			"http_request": map[string]interface{}{
				"client_ip":  "0.0.0.0",
				"user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
			},
		},
		"object":    "event",
		"timestamp": float64(1654012824306),
		"type":      "user.updated",
	}
}

func clerkOfficialUserDeletedPayload() map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"deleted": true,
			"id":      "user_29wBMCtzATuFJut8jO2VNTVekS4",
			"object":  "user",
		},
		"event_attributes": map[string]interface{}{
			"http_request": map[string]interface{}{
				"client_ip":  "0.0.0.0",
				"user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
			},
		},
		"object":    "event",
		"timestamp": float64(1661861640000),
		"type":      "user.deleted",
	}
}

func clonePayload(t *testing.T, src map[string]interface{}) map[string]interface{} {
	t.Helper()

	raw, err := json.Marshal(src)
	require.NoError(t, err)

	var cloned map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &cloned))
	return cloned
}

func payloadDataMap(t *testing.T, payload map[string]interface{}) map[string]interface{} {
	t.Helper()

	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	return data
}

func clerkUserCreatedPayload(t *testing.T, externalID, email, firstName, lastName string) map[string]interface{} {
	t.Helper()

	payload := clonePayload(t, clerkOfficialUserCreatedPayload())
	data := payloadDataMap(t, payload)
	primaryEmailID := "idn_" + externalID

	data["id"] = externalID
	data["first_name"] = firstName
	data["last_name"] = lastName
	data["primary_email_address_id"] = primaryEmailID
	data["email_addresses"] = []interface{}{
		map[string]interface{}{
			"id":            primaryEmailID,
			"email_address": email,
			"object":        "email_address",
			"linked_to":     []interface{}{},
			"reserved":      false,
			"verification": map[string]interface{}{
				"attempts":  nil,
				"expire_at": nil,
				"status":    "verified",
				"strategy":  "admin",
			},
		},
	}

	return payload
}

func clerkUserUpdatedPayload(t *testing.T, externalID, email, firstName, lastName string) map[string]interface{} {
	t.Helper()

	payload := clonePayload(t, clerkOfficialUserUpdatedPayload())
	data := payloadDataMap(t, payload)
	primaryEmailID := "idn_" + externalID

	data["id"] = externalID
	data["first_name"] = firstName
	data["last_name"] = lastName
	data["primary_email_address_id"] = primaryEmailID
	data["email_addresses"] = []interface{}{
		map[string]interface{}{
			"email_address": email,
			"id":            primaryEmailID,
			"linked_to":     []interface{}{},
			"object":        "email_address",
			"reserved":      true,
			"verification": map[string]interface{}{
				"attempts":  nil,
				"expire_at": nil,
				"status":    "verified",
				"strategy":  "admin",
			},
		},
	}

	return payload
}

func clerkUserDeletedPayload(t *testing.T, externalID string) map[string]interface{} {
	t.Helper()

	payload := clonePayload(t, clerkOfficialUserDeletedPayload())
	data := payloadDataMap(t, payload)
	data["id"] = externalID
	return payload
}

func signSvixPayload(payload []byte, secret string) (string, string, string) {
	msgID := "msg_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	secretKey := secret
	if len(secretKey) > 6 && secretKey[:6] == "whsec_" {
		secretKey = secretKey[6:]
	}
	secretBytes, _ := base64.StdEncoding.DecodeString(secretKey)

	toSign := fmt.Sprintf("%s.%s.%s", msgID, timestamp, string(payload))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(toSign))
	signature := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return msgID, timestamp, signature
}

func svixHeaderEditor(payload []byte, secret string) ct.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		msgID, ts, sig := signSvixPayload(payload, secret)
		req.Header.Set("Svix-Id", msgID)
		req.Header.Set("Svix-Timestamp", ts)
		req.Header.Set("Svix-Signature", sig)
		return nil
	}
}

func createClerkIntegrationInstance(
	t *testing.T,
	productContext *itdsl.ProductContext,
) *ct.IntegrationInstanceResponse {
	t.Helper()

	createBody := ct.CreateIntegrationInstanceJSONRequestBody{}
	require.NoError(
		t,
		createBody.FromClerkIntegrationInstanceCreateRequest(
			ct.ClerkIntegrationInstanceCreateRequest{ProviderType: "CLERK"},
		),
	)

	resp, err := productContext.OwnerAuthenticatedClient().CreateIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		createBody,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode())
	require.NotNil(t, resp.JSON201)

	return resp.JSON201
}

func updateClerkIntegrationInstance(
	t *testing.T,
	productContext *itdsl.ProductContext,
	integrationInstanceID string,
	request ct.UpdateIntegrationInstanceJSONRequestBody,
) *ct.IntegrationInstanceResponse {
	t.Helper()

	resp, err := productContext.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		integrationInstanceID,
		request,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)

	return resp.JSON200
}

func createActiveClerkIntegrationInstance(
	t *testing.T,
	productContext *itdsl.ProductContext,
) *ct.IntegrationInstanceResponse {
	t.Helper()

	instance := createClerkIntegrationInstance(t, productContext)
	return updateClerkIntegrationInstance(
		t,
		productContext,
		instance.Id,
		ct.UpdateIntegrationInstanceJSONRequestBody{WebhookSecret: ptr.Ptr(clerkTestWebhookSecret)},
	)
}

func getIntegrationInstance(
	t *testing.T,
	productContext *itdsl.ProductContext,
	integrationInstanceID string,
) *ct.IntegrationInstanceResponse {
	t.Helper()

	resp, err := productContext.OwnerAuthenticatedClient().GetIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		integrationInstanceID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)

	return resp.JSON200
}

func listIntegrationInstances(
	t *testing.T,
	productContext *itdsl.ProductContext,
) *ct.IntegrationInstanceListResponse {
	t.Helper()

	resp, err := productContext.OwnerAuthenticatedClient().ListIntegrationInstancesWithResponse(
		context.Background(),
		productContext.ProductID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)

	return resp.JSON200
}

func listIntegrationAuditLogs(
	t *testing.T,
	productContext *itdsl.ProductContext,
	integrationInstanceID string,
) *ct.IntegrationAuditLogListResponse {
	t.Helper()

	resp, err := productContext.OwnerAuthenticatedClient().ListIntegrationAuditLogsWithResponse(
		context.Background(),
		productContext.ProductID,
		integrationInstanceID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)

	return resp.JSON200
}

func sendClerkWebhook(
	t *testing.T,
	productID string,
	payload map[string]interface{},
	secret string,
) *ct.IngestWebhookResponse {
	t.Helper()

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, sendErr := dslfactory.NewNoAuthClient(t, itshared.ServerURL).IngestWebhookWithBodyWithResponse(
		context.Background(),
		productID,
		"CLERK",
		"application/json",
		io.NopCloser(bytes.NewReader(payloadBytes)),
		svixHeaderEditor(payloadBytes, secret),
	)
	require.NoError(t, sendErr)

	return resp
}

func searchProductUsers(
	t *testing.T,
	productContext *itdsl.ProductContext,
) *ct.SearchProductUsersResponse {
	t.Helper()

	apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
	resp, err := apiKeyClient.SearchProductUsersWithResponse(
		context.Background(),
		productContext.ProductID,
		ct.SearchProductUsersJSONRequestBody{},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)

	return resp
}
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}

	itshared.RunTestMain(
		m, itshared.TestConfig{
			EnableRedis:             true,
			PopulateRepositories:    true,
			APIKeyService:           &itshared.APIKeyService,
			PermissionRepository:    &itshared.PermissionRepository,
			ProductRepository:       &itshared.ProductRepository,
			ProductUserRepository:   &itshared.ProductUserRepository,
			OrgMembershipRepository: &itshared.OrgMembershipRepository,
			TenantRepository:        &itshared.TenantRepository,
			UserRepository:          &itshared.UserRepository,
			PlatformUserRepository:  &itshared.PlatformTenantUserRepo,
			JWTHelper:               &itshared.JWTHelper,
		},
	)
}
