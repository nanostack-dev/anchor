//nolint:testpackage // sets the unexported retry delay.
package smtp

import (
	"bufio"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSMTPServer answers each connection's AUTH with the next reply in
// authReplies (the last one repeats) and accepts the message after a 235.
type fakeSMTPServer struct {
	listener    net.Listener
	conns       atomic.Int32
	authReplies []string
	delivered   atomic.Int32
	wg          sync.WaitGroup
}

func newFakeSMTPServer(t *testing.T, authReplies ...string) *fakeSMTPServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakeSMTPServer{listener: l, authReplies: authReplies}
	s.wg.Go(func() {
		for {
			c, acceptErr := l.Accept()
			if acceptErr != nil {
				return
			}
			n := int(s.conns.Add(1))
			s.wg.Go(func() { s.serve(c, n) })
		}
	})
	t.Cleanup(func() {
		_ = l.Close()
		s.wg.Wait()
	})
	return s
}

func (s *fakeSMTPServer) serve(c net.Conn, n int) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	w := func(line string) { _, _ = c.Write([]byte(line + "\r\n")) }
	w("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			w("250-fake")
			w("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "AUTH"):
			w(s.authReplies[min(n, len(s.authReplies))-1])
		case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
			w("250 OK")
		case cmd == "DATA":
			w("354 go ahead")
			for {
				l, readErr := r.ReadString('\n')
				if readErr != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
			}
			s.delivered.Add(1)
			w("250 queued")
		case cmd == "QUIT":
			w("221 bye")
			return
		default:
			w("500 unknown")
		}
	}
}

func sendThroughFake(t *testing.T, s *fakeSMTPServer) error {
	t.Helper()
	authRetryDelay = 0
	host, portStr, err := net.SplitHostPort(s.listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	p := newTestProvider(t)
	cfg := Config{
		Host: host, Port: port, Username: "u", Password: "p",
		FromAddress: "from@example.com", Encryption: EncryptionNone, AuthMethod: AuthMethodPlain,
	}
	return p.Send(t.Context(), &integration.Instance{ConfigJSON: mustMarshalConfig(t, cfg)}, provider.OutboundMessage{
		To:      []provider.Address{{Email: "to@example.com"}},
		Subject: "hi",
		Text:    "hello",
	})
}

//nolint:paralleltest // mutates the package-level retry delay.
func TestSendRetriesTransientAuthReplyOnce(t *testing.T) {
	s := newFakeSMTPServer(t, "454 4.7.0 Temporary authentication failure", "235 ok")
	require.NoError(t, sendThroughFake(t, s))
	assert.Equal(t, int32(2), s.conns.Load())
	assert.Equal(t, int32(1), s.delivered.Load())
}

//nolint:paralleltest // mutates the package-level retry delay.
func TestSendGivesUpAfterSecondTransientAuthReply(t *testing.T) {
	s := newFakeSMTPServer(t, "454 4.7.0 Temporary authentication failure")
	err := sendThroughFake(t, s)
	require.Error(t, err)
	var protoErr *textproto.Error
	require.ErrorAs(t, err, &protoErr)
	assert.Equal(t, 454, protoErr.Code)
	assert.Contains(t, err.Error(), "smtp auth:")
	assert.Equal(t, int32(2), s.conns.Load())
	assert.Equal(t, int32(0), s.delivered.Load())
}

//nolint:paralleltest // mutates the package-level retry delay.
func TestSendDoesNotRetryPermanentAuthReply(t *testing.T) {
	s := newFakeSMTPServer(t, "535 5.7.8 Authentication credentials invalid")
	err := sendThroughFake(t, s)
	require.Error(t, err)
	var protoErr *textproto.Error
	require.ErrorAs(t, err, &protoErr)
	assert.Equal(t, 535, protoErr.Code)
	assert.Equal(t, int32(1), s.conns.Load())
}
