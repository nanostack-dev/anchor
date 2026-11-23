package ct_test

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"
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

func TestSMTPUpdateMustNotBypassConnectionVerification(t *testing.T) {
	t.Parallel()

	closedPort := func(t *testing.T) int {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := ln.Addr().(*net.TCPAddr).Port
		require.NoError(t, ln.Close())
		return port
	}

	smtpConfig := func(port int) ct.SmtpIntegrationConfig {
		return ct.SmtpIntegrationConfig{
			Host:        new("127.0.0.1"),
			Port:        new(port),
			Encryption:  new(ct.SmtpIntegrationConfigEncryptionNONE),
			AuthMethod:  new(ct.SmtpIntegrationConfigAuthMethodPLAIN),
			Username:    new("user"),
			Password:    new("pass"),
			FromAddress: new("noreply@example.test"),
		}
	}

	createFailingSMTP := func(t *testing.T, client *ct.ClientWithResponses, productID string, port int) string {
		t.Helper()
		ctx := t.Context()
		var createBody ct.CreateIntegrationInstanceJSONRequestBody
		require.NoError(t, createBody.FromSmtpIntegrationInstanceCreateRequest(ct.SmtpIntegrationInstanceCreateRequest{
			ProviderType: ct.SmtpIntegrationInstanceCreateRequestProviderTypeSMTP,
			Config:       smtpConfig(port),
		}))

		created, err := client.CreateIntegrationInstanceWithResponse(ctx, productID, createBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		instanceID := created.JSON201.Id

		require.Eventually(t, func() bool {
			got, getErr := client.GetIntegrationInstanceWithResponse(ctx, productID, instanceID)
			return getErr == nil && got.JSON200 != nil && got.JSON200.Status == ct.IntegrationInstanceStatusERROR
		}, 15*time.Second, 100*time.Millisecond)
		return instanceID
	}

	t.Run("Setting webhook_secret on SMTP is refused and keeps ERROR status", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		ctx := t.Context()
		instanceID := createFailingSMTP(t, client, product.ProductID, closedPort(t))

		secret := "whsec_not_supported_for_smtp"
		secretResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{WebhookSecret: &secret},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, secretResp.StatusCode(), string(secretResp.Body))
		require.NotNil(t, secretResp.JSON400, string(secretResp.Body))
		require.NotEmpty(t, secretResp.JSON400.Errors)
		assert.Equal(t, "INTEGRATION_WEBHOOK_SECRET_UNSUPPORTED", secretResp.JSON400.Errors[0].Code)

		got, err := client.GetIntegrationInstanceWithResponse(ctx, product.ProductID, instanceID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.StatusCode())
		assert.Equal(t, ct.IntegrationInstanceStatusERROR, got.JSON200.Status)
	})

	t.Run("Re-enabling an unverified SMTP instance verifies it again", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		ctx := t.Context()
		instanceID := createFailingSMTP(t, client, product.ProductID, closedPort(t))

		disabled := false
		disableResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: &disabled},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, disableResp.StatusCode(), string(disableResp.Body))
		assert.Equal(t, ct.IntegrationInstanceStatusINACTIVE, disableResp.JSON200.Status)

		enabled := true
		enableResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: &enabled},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, enableResp.StatusCode(), string(enableResp.Body))
		assert.Equal(t, ct.IntegrationInstanceStatusCONFIGURING, enableResp.JSON200.Status)

		require.Eventually(t, func() bool {
			got, getErr := client.GetIntegrationInstanceWithResponse(ctx, product.ProductID, instanceID)
			return getErr == nil && got.JSON200 != nil && got.JSON200.Status == ct.IntegrationInstanceStatusERROR
		}, 15*time.Second, 100*time.Millisecond)
	})

	t.Run("Updating config on a disabled SMTP instance stays INACTIVE", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		ctx := t.Context()
		port := closedPort(t)
		instanceID := createFailingSMTP(t, client, product.ProductID, port)

		disabled := false
		disableResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: &disabled},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, disableResp.StatusCode())
		require.Equal(t, ct.IntegrationInstanceStatusINACTIVE, disableResp.JSON200.Status)

		var updateCfg ct.IntegrationProviderConfig
		require.NoError(t, updateCfg.FromSmtpIntegrationConfig(smtpConfig(port)))
		updateResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{Config: &updateCfg},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, updateResp.StatusCode(), string(updateResp.Body))
		assert.Equal(t, ct.IntegrationInstanceStatusINACTIVE, updateResp.JSON200.Status)
		assert.False(t, updateResp.JSON200.IsEnabled)
	})

	t.Run("A slow verification run cannot activate a newer config after disable and reconfigure", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		ctx := t.Context()

		slowPort, accepted, release := startSlowSMTPPeer(t)

		var createBody ct.CreateIntegrationInstanceJSONRequestBody
		require.NoError(t, createBody.FromSmtpIntegrationInstanceCreateRequest(ct.SmtpIntegrationInstanceCreateRequest{
			ProviderType: ct.SmtpIntegrationInstanceCreateRequestProviderTypeSMTP,
			Config:       smtpConfig(slowPort),
		}))
		created, err := client.CreateIntegrationInstanceWithResponse(ctx, product.ProductID, createBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		instanceID := created.JSON201.Id

		// Wait until the slow verification run has dialed the peer.
		select {
		case <-accepted:
		case <-time.After(10 * time.Second):
			t.Fatal("slow SMTP peer was never dialed")
		}

		// Disable, then point config at an unreachable port while disabled.
		disabled := false
		disableResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: &disabled},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, disableResp.StatusCode())
		require.Equal(t, ct.IntegrationInstanceStatusINACTIVE, disableResp.JSON200.Status)

		var badCfg ct.IntegrationProviderConfig
		require.NoError(t, badCfg.FromSmtpIntegrationConfig(smtpConfig(closedPort(t))))
		reconfigResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{Config: &badCfg},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, reconfigResp.StatusCode())
		require.Equal(t, ct.IntegrationInstanceStatusINACTIVE, reconfigResp.JSON200.Status)

		// Let the old peer succeed if the old run was still waiting, then give it time to settle.
		close(release)
		time.Sleep(300 * time.Millisecond)

		stillInactive, err := client.GetIntegrationInstanceWithResponse(ctx, product.ProductID, instanceID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, stillInactive.StatusCode())
		assert.Equal(t, ct.IntegrationInstanceStatusINACTIVE, stillInactive.JSON200.Status,
			"stale verification run must not overwrite newer disabled config with ACTIVE")

		// Re-enabling must enter CONFIGURING and fail against the unreachable port, never ACTIVE.
		enabled := true
		reEnableResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: &enabled},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, reEnableResp.StatusCode())
		assert.Equal(t, ct.IntegrationInstanceStatusCONFIGURING, reEnableResp.JSON200.Status)

		require.Eventually(t, func() bool {
			got, getErr := client.GetIntegrationInstanceWithResponse(ctx, product.ProductID, instanceID)
			return getErr == nil && got.JSON200 != nil && got.JSON200.Status == ct.IntegrationInstanceStatusERROR
		}, 15*time.Second, 100*time.Millisecond)
	})

	t.Run("A no-op update while verifying does not strand SMTP in CONFIGURING", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		ctx := t.Context()

		slowPort, accepted, release := startSlowSMTPPeer(t)

		var createBody ct.CreateIntegrationInstanceJSONRequestBody
		require.NoError(t, createBody.FromSmtpIntegrationInstanceCreateRequest(ct.SmtpIntegrationInstanceCreateRequest{
			ProviderType: ct.SmtpIntegrationInstanceCreateRequestProviderTypeSMTP,
			Config:       smtpConfig(slowPort),
		}))
		created, err := client.CreateIntegrationInstanceWithResponse(ctx, product.ProductID, createBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		instanceID := created.JSON201.Id

		select {
		case <-accepted:
		case <-time.After(10 * time.Second):
			t.Fatal("slow SMTP peer was never dialed")
		}

		// No-op update while CONFIGURING bumps updated_at; a fresh verification must still reach ACTIVE.
		noopResp, err := client.UpdateIntegrationInstanceWithResponse(
			ctx, product.ProductID, instanceID,
			ct.UpdateIntegrationInstanceJSONRequestBody{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, noopResp.StatusCode(), string(noopResp.Body))
		assert.Equal(t, ct.IntegrationInstanceStatusCONFIGURING, noopResp.JSON200.Status)

		close(release)

		require.Eventually(t, func() bool {
			got, getErr := client.GetIntegrationInstanceWithResponse(ctx, product.ProductID, instanceID)
			return getErr == nil && got.JSON200 != nil && got.JSON200.Status == ct.IntegrationInstanceStatusACTIVE
		}, 15*time.Second, 100*time.Millisecond)
	})
}

func startSlowSMTPPeer(t *testing.T) (int, <-chan struct{}, chan struct{}) {
	t.Helper()
	accepted := make(chan struct{}, 4)
	release := make(chan struct{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		_ = listener.Close()
	})
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveSlowSMTPConn(conn, accepted, release)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, accepted, release
}

func serveSlowSMTPConn(c net.Conn, accepted chan<- struct{}, release <-chan struct{}) {
	defer func() { _ = c.Close() }()
	select {
	case accepted <- struct{}{}:
	default:
	}
	<-release
	_, _ = c.Write([]byte("220 slow.example.test ESMTP\r\n"))
	reader := bufio.NewReader(c)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return
		}
		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			_, _ = c.Write([]byte("250-slow.example.test\r\n250 AUTH PLAIN\r\n"))
		case strings.HasPrefix(upper, "AUTH"):
			_, _ = c.Write([]byte("235 Authentication succeeded\r\n"))
		case strings.HasPrefix(upper, "QUIT"):
			_, _ = c.Write([]byte("221 Bye\r\n"))
			return
		default:
			_, _ = c.Write([]byte("250 OK\r\n"))
		}
	}
}
