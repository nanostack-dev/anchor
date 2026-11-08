package ct_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSMTPVerificationKeepsThePeerReplyPrivate(t *testing.T) {
	t.Parallel()

	t.Run("peer banner is not returned in integration response", func(t *testing.T) {
		marker := "internal-banner-" + ids.MustNew("ct")
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		go func() {
			for {
				conn, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				_, _ = conn.Write([]byte("+OK " + marker + "\r\n"))
				_ = conn.Close()
			}
		}()
		port := listener.Addr().(*net.TCPAddr).Port

		status, body := createSMTPIntegrationAndWaitForVerification(t, port)
		if status == http.StatusBadRequest {
			return
		}
		t.Logf("integration response after verification: %s", body)
		assert.NotContains(t, body, marker, "peer-supplied banner text leaked to caller")
	})

	t.Run("dial outcome is not returned in integration response", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())

		status, body := createSMTPIntegrationAndWaitForVerification(t, port)
		if status == http.StatusBadRequest {
			return
		}
		t.Logf("integration response after verification: %s", body)
		assert.NotContains(t, body, "connection refused", "raw dial outcome leaked to caller")
	})
}

func createSMTPIntegrationAndWaitForVerification(t *testing.T, port int) (int, string) {
	t.Helper()
	product := createTestProductContext(t)
	client := product.OwnerAuthenticatedClient()

	var body ct.CreateIntegrationInstanceJSONRequestBody
	require.NoError(t, body.FromSmtpIntegrationInstanceCreateRequest(ct.SmtpIntegrationInstanceCreateRequest{
		ProviderType: ct.SmtpIntegrationInstanceCreateRequestProviderTypeSMTP,
		Config: ct.SmtpIntegrationConfig{
			Host:        new("127.0.0.1"),
			Port:        new(port),
			Encryption:  new(ct.SmtpIntegrationConfigEncryptionNONE),
			AuthMethod:  new(ct.SmtpIntegrationConfigAuthMethodPLAIN),
			Username:    new("secaudit-user"),
			Password:    new("secaudit-password"),
			FromAddress: new("noreply@example.test"),
		},
	}))

	created, err := client.CreateIntegrationInstanceWithResponse(context.Background(), product.ProductID, body)
	require.NoError(t, err)
	if created.StatusCode() == http.StatusBadRequest {
		return http.StatusBadRequest, string(created.Body)
	}
	require.Equal(t, http.StatusCreated, created.StatusCode(), "body: %s", string(created.Body))
	require.NotNil(t, created.JSON201)
	instanceID := created.JSON201.Id

	var latest string
	var latestStatus ct.IntegrationInstanceStatus
	require.Eventually(t, func() bool {
		got, getErr := client.GetIntegrationInstanceWithResponse(context.Background(), product.ProductID, instanceID)
		if getErr != nil || got.JSON200 == nil {
			return false
		}
		latest = string(got.Body)
		latestStatus = got.JSON200.Status
		return got.JSON200.Status != ct.IntegrationInstanceStatusCONFIGURING
	}, 45*time.Second, 250*time.Millisecond, "verification did not finish; last body: %s", latest)

	require.Equal(t, ct.IntegrationInstanceStatusERROR, latestStatus,
		"expected verification to fail against a non-SMTP peer; body: %s", latest)
	return http.StatusCreated, latest
}
