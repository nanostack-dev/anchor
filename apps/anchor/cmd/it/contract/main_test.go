//go:build contract

// Package contract_test is the golden contract suite: it boots the real Anchor
// server in-process (the same generated wiring, router, middleware, error
// handler and logger as cmd/main.go), replays the production gate flows and
// the framework seam cases against it, and compares each response and its log
// lines with a golden file under testdata/golden. See README.md.
package contract_test

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	itshared "anchor/cmd/it/shared"
)

// update regenerates the golden files instead of comparing with them.
var update = flag.Bool("update", false, "rewrite the golden files under testdata/golden")

// Owner credentials for the first registration, which initialises the
// platform tenant. The address is the garbage mailbox; the container is
// thrown away after the run.
const (
	ownerEmail    = "garbage+contract-owner@nanostack.dev"
	ownerPassword = "Contract!Owner1Aa"
)

// suite holds the process-wide state the cases share.
var suite struct {
	logs        *logSink
	baseURL     string
	ownerToken  string
	bootElapsed time.Duration
}

func TestMain(m *testing.M) {
	// Production runs in UTC containers. Without this, a time.Now() value the
	// server serialises (licence migrated_at, the log time field) carries the
	// developer's zone offset, and the goldens would depend on the machine.
	time.Local = time.UTC //nolint:reassign // pin the server's zone to production's

	// The framework logger writes to os.Stdout, captured when the App is built.
	// Point os.Stdout at a pipe until the app is up so every log line the
	// server writes reaches the sink; the testing package gets the real
	// stdout back before m.Run.
	sink, err := newLogSink()
	if err != nil {
		fmt.Fprintln(os.Stderr, "contract: log sink:", err)
		os.Exit(1)
	}
	suite.logs = sink
	realStdout := os.Stdout
	os.Stdout = sink.writer //nolint:reassign // the framework logger writes to os.Stdout; the suite captures it
	sink.forward = realStdout
	bootStart := time.Now()

	itshared.RunTestMain(m, itshared.TestConfig{
		EnableRedis: true,
		AfterInit: func() {
			suite.bootElapsed = time.Since(bootStart)
			os.Stdout = realStdout //nolint:reassign // give the testing package its stdout back
			suite.baseURL = itshared.ServerURL
			if bootstrapErr := bootstrapOwner(); bootstrapErr != nil {
				fmt.Fprintln(os.Stderr, "contract: owner bootstrap:", bootstrapErr)
				os.Exit(1)
			}
		},
	})
}

// bootstrapOwner registers the platform owner (the first registration needs no
// invitation code) and logs it in.
func bootstrapOwner() error {
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, ownerEmail, ownerPassword)
	resp, err := rawDo(context.Background(), http.MethodPost, suite.baseURL+"/v1/auth/register",
		map[string]string{"Content-Type": "application/json"}, body, "contract-bootstrap-register")
	if err != nil {
		return err
	}
	if resp.status != http.StatusOK {
		return fmt.Errorf("register: status %d: %s", resp.status, resp.body)
	}
	token, ok := resp.json()["accessToken"].(string)
	if !ok || strings.TrimSpace(token) == "" {
		return fmt.Errorf("register: no accessToken in %s", resp.body)
	}
	suite.ownerToken = token
	return seedSecondPlatformUser()
}

// seedSecondPlatformUser gives the tenant a second platform user, as on dev:
// the cursor-search gate flow pages through at least two.
func seedSecondPlatformUser() error {
	auth := map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + suite.ownerToken}
	invite, err := rawDo(context.Background(), http.MethodPost, suite.baseURL+"/v1/platform-invitations", auth,
		`{"email":"garbage+contract-admin@nanostack.dev"}`, "contract-bootstrap-invite")
	if err != nil {
		return err
	}
	code, _ := invite.json()["code"].(string)
	if invite.status != http.StatusCreated || code == "" {
		return fmt.Errorf("platform invitation: status %d: %s", invite.status, invite.body)
	}
	body := fmt.Sprintf(`{"email":"garbage+contract-admin@nanostack.dev","password":%q,"invitation_code":%q}`,
		ownerPassword, code)
	resp, err := rawDo(context.Background(), http.MethodPost, suite.baseURL+"/v1/auth/register",
		map[string]string{"Content-Type": "application/json"}, body, "contract-bootstrap-register-admin")
	if err != nil {
		return err
	}
	if resp.status != http.StatusOK {
		return fmt.Errorf("register admin: status %d: %s", resp.status, resp.body)
	}
	return nil
}
