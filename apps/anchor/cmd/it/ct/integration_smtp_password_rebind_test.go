package ct_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

type smtpCaseSMTPStub struct {
	listener   net.Listener
	authReply  string
	mu         sync.Mutex
	authValues []string
}

func newSecAuditSMTPStub(t *testing.T, authReply string) *smtpCaseSMTPStub {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stub := &smtpCaseSMTPStub{listener: listener, authReply: authReply}
	t.Cleanup(func() { _ = listener.Close() })
	go stub.serve()
	return stub
}

func (s *smtpCaseSMTPStub) port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *smtpCaseSMTPStub) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *smtpCaseSMTPStub) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	reader := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line)) }
	write("220 stub ESMTP\r\n")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			write("250-stub\r\n250 AUTH PLAIN LOGIN\r\n")
		case strings.HasPrefix(upper, "AUTH PLAIN "):
			s.mu.Lock()
			s.authValues = append(s.authValues, strings.TrimSpace(line[len("AUTH PLAIN "):]))
			s.mu.Unlock()
			write(s.authReply)
		case line == "*":
			write("501 cancelled\r\n")
		case strings.HasPrefix(upper, "QUIT"):
			write("221 bye\r\n")
			return
		default:
			write("250 ok\r\n")
		}
	}
}

func (s *smtpCaseSMTPStub) decodedAuths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.authValues))
	for _, v := range s.authValues {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			out = append(out, "undecodable:"+v)
			continue
		}
		out = append(out, string(decoded))
	}
	return out
}

func smtpCaseSMTPConfigBody(port int, password string) map[string]any {
	cfg := map[string]any{
		"host":         "127.0.0.1",
		"port":         port,
		"encryption":   "NONE",
		"auth_method":  "PLAIN",
		"username":     "apikey",
		"from_address": "noreply@example.test",
	}
	if password != "" {
		cfg["password"] = password
	}
	return cfg
}

func TestSMTPUpdateToAnotherHostNeedsThePassword(t *testing.T) {
	t.Parallel()

	tenantAlias := "tenant.secaudit." + itshared.Faker.UUID().V4()
	productAlias := "product.secaudit." + itshared.Faker.UUID().V4()
	adminAlias := "admin.secaudit." + itshared.Faker.UUID().V4()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantAlias, Isolated: true}).
		Product(itdsl.ProductOpts{Alias: productAlias, TenantAlias: tenantAlias}).
		PlatformAdmin(itdsl.PlatformAdminOpts{Alias: adminAlias, TenantAlias: tenantAlias}).
		Build()
	product := state.Product(productAlias)
	owner := product.OwnerAuthenticatedClient()
	admin := state.PlatformUser(adminAlias).AuthenticatedClient

	stubA := newSecAuditSMTPStub(t, "235 2.7.0 ok\r\n")
	stubB := newSecAuditSMTPStub(t, "535 5.7.8 rejected\r\n")

	secret := "dummy-owner-secret-" + itshared.Faker.UUID().V4()
	ctx := context.Background()

	createBody, err := json.Marshal(map[string]any{
		"provider_type": "SMTP",
		"config":        smtpCaseSMTPConfigBody(stubA.port(), secret),
	})
	require.NoError(t, err)
	createResp, err := owner.CreateIntegrationInstanceWithBodyWithResponse(
		ctx, product.ProductID, "application/json", strings.NewReader(string(createBody)),
	)
	require.NoError(t, err)
	require.Equal(t, 201, createResp.StatusCode(), string(createResp.Body))
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createResp.Body, &created))
	require.NotEmpty(t, created.ID)

	expectedPlain := "\x00apikey\x00" + secret
	getStatus := func() (string, string) {
		resp, getErr := owner.GetIntegrationInstanceWithResponse(ctx, product.ProductID, created.ID)
		if getErr != nil || resp.JSON200 == nil {
			return "", ""
		}
		lastErr := ""
		if resp.JSON200.LastError != nil {
			lastErr = *resp.JSON200.LastError
		}
		return string(resp.JSON200.Status), lastErr
	}

	require.Eventually(t, func() bool {
		status, _ := getStatus()
		if status != "ACTIVE" {
			return false
		}
		return slices.Contains(stubA.decodedAuths(), expectedPlain)
	}, 45*time.Second, 200*time.Millisecond, "baseline: owner-entered password should authenticate to original host A")

	updateBody, err := json.Marshal(map[string]any{
		"config": smtpCaseSMTPConfigBody(stubB.port(), ""),
	})
	require.NoError(t, err)
	require.NotContains(t, string(updateBody), "password")
	updateResp, err := admin.UpdateIntegrationInstanceWithBodyWithResponse(
		ctx, product.ProductID, created.ID, "application/json", strings.NewReader(string(updateBody)),
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, updateResp.StatusCode(), string(updateResp.Body))
	time.Sleep(time.Second)
	for _, decoded := range stubB.decodedAuths() {
		assert.NotContains(t, decoded, secret, "the stored password reached the new host")
	}
}
