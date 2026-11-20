package ct_test

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainintegration "anchor/internal/domain/integration"
)

// slowAuthSMTPStub answers the first AUTH late with a transient 454 and every
// later AUTH with 235, then accepts the message. It reproduces a relay whose
// authentication server is slow and briefly unavailable.
type slowAuthSMTPStub struct {
	listener  net.Listener
	firstWait time.Duration
	firstAuth chan struct{}
	auths     atomic.Int32
	delivered atomic.Int32
}

func newSlowAuthSMTPStub(t *testing.T, firstWait time.Duration) *slowAuthSMTPStub {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stub := &slowAuthSMTPStub{listener: listener, firstWait: firstWait, firstAuth: make(chan struct{})}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go stub.handle(conn)
		}
	}()
	return stub
}

func (s *slowAuthSMTPStub) handle(conn net.Conn) {
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
		upper := strings.ToUpper(strings.TrimRight(line, "\r\n"))
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			write("250-stub\r\n250 AUTH PLAIN LOGIN\r\n")
		case strings.HasPrefix(upper, "AUTH"):
			if s.auths.Add(1) == 1 {
				close(s.firstAuth)
				time.Sleep(s.firstWait)
				write("454 4.7.0 Temporary authentication failure\r\n")
				continue
			}
			write("235 2.7.0 Authentication successful\r\n")
		case upper == "DATA":
			write("354 go ahead\r\n")
			for {
				body, readErr := reader.ReadString('\n')
				if readErr != nil {
					return
				}
				if body == ".\r\n" {
					break
				}
			}
			s.delivered.Add(1)
			write("250 queued\r\n")
		case strings.HasPrefix(upper, "QUIT"):
			write("221 bye\r\n")
			return
		default:
			write("250 ok\r\n")
		}
	}
}

func (s *slowAuthSMTPStub) port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// TestEmailSendFinishesAfterClientHangUp covers the 2026-11-20 recurrence of the
// SMTP AUTH postmortem: the caller gave up while the relay was slow, so the
// single AUTH retry was skipped and the send ended FAILED behind a duplicate 201.
func TestEmailSendFinishesAfterClientHangUp(t *testing.T) {
	t.Parallel()

	t.Run("A send whose caller hangs up still retries the transient AUTH reply and ends SENT", func(t *testing.T) {
		t.Parallel()

		tc := newEmailTestCtx(t)
		stub := newSlowAuthSMTPStub(t, time.Second)
		instance := domainintegration.Instance{
			PlatformTenantID: tc.tenantID, ProductID: tc.product.ProductID,
			ProviderType: domainintegration.ProviderTypeSMTP, ConfigJSON: smtpConfigJSON(t, "127.0.0.1", stub.port()),
			ConfigVersion: 1, IsEnabled: true, Status: domainintegration.StatusActive,
		}
		instance.GenerateID()
		_, err := IntegrationRepo.Create(context.Background(), instance)
		require.NoError(t, err)

		client := tc.product.OwnerAuthenticatedClient()
		tplID := publishTemplate(t, client, tc, "Hello", "<p>Hi</p>")
		dedupeKey := "hangup-" + uniqueSlug()

		// The caller hangs up once the relay holds AUTH: the send record exists
		// and the dispatch is under way, as in the 2026-11-20 recurrence.
		hangUpCtx, hangUp := context.WithTimeout(context.Background(), 30*time.Second)
		defer hangUp()
		go func() {
			<-stub.firstAuth
			hangUp()
		}()
		_, sendErr := client.SendEmailWithResponse(hangUpCtx, tc.product.ProductID, ct.SendEmailJSONRequestBody{
			TemplateId: &tplID,
			ToAddress:  openapi_types.Email("bob@example.com"),
			DedupeKey:  &dedupeKey,
		})
		require.Error(t, sendErr, "the caller hangs up before the relay answers AUTH")

		status := ""
		require.Eventually(t, func() bool {
			list, listErr := client.SearchEmailSendsWithResponse(context.Background(), tc.product.ProductID,
				ct.SearchEmailSendsJSONRequestBody{})
			if listErr != nil || list.StatusCode() != http.StatusOK || list.JSON200 == nil {
				return false
			}
			for _, item := range list.JSON200.Items {
				if item.DedupeKey != nil && *item.DedupeKey == dedupeKey {
					status = string(item.Status)
				}
			}
			return status == "SENT" || status == "FAILED"
		}, 15*time.Second, 100*time.Millisecond)

		assert.Equal(t, "SENT", status, "the dispatch outlives the caller's request")
		assert.EqualValues(t, 2, stub.auths.Load(), "the transient AUTH reply is retried once")
		assert.EqualValues(t, 1, stub.delivered.Load(), "the message is delivered exactly once")
	})
}
