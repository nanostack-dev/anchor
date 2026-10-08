//nolint:testpackage // Tests use internal Stripe payload types to inspect the CLI boundary.
package stripeprototype

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type commandResult struct {
	output string
	err    error
}

type recordingRunner struct {
	results []commandResult
	args    [][]string
}

func (r *recordingRunner) Run(_ context.Context, args []string) ([]byte, error) {
	r.args = append(r.args, append([]string(nil), args...))
	if len(r.results) == 0 {
		return nil, errors.New("unexpected Stripe CLI command")
	}
	result := r.results[0]
	r.results = r.results[1:]
	return []byte(result.output), result.err
}

func TestCLIAccountRejectsForeignAccountAndLiveMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		identity string
	}{
		{"foreign account", `{"account_id":"acct_foreign","display_name":"Another sandbox","mode":"test"}`},
		{"live account", `{"account_id":"acct_expected","display_name":"New Business","mode":"live"}`},
		{"missing mode", `{"account_id":"acct_expected"}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runner := &recordingRunner{results: []commandResult{{output: test.identity}}}
			cli, err := NewCLI("acct_expected", runner)
			require.NoError(t, err)
			err = cli.Request(t.Context(), "post", "/v1/customers", nil, "customer-intent", &stripeObject{})
			require.Error(t, err)
			require.Len(t, runner.args, 1)
			assert.Equal(t, "whoami", runner.args[0][0])
		})
	}
}

func TestCLIRequestPinsSandboxAndKeepsParametersAsArguments(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{results: []commandResult{
		{output: `{"account_id":"acct_expected","display_name":"New Business","mode":"test"}`},
		{output: `{"id":"cus_test","object":"customer","livemode":false}`},
	}}
	cli, err := NewCLI("acct_expected", runner)
	require.NoError(t, err)
	name := "Organization ; $(echo untrusted) `echo also-untrusted`"
	var result stripeObject
	err = cli.Request(t.Context(), "post", "/v1/customers", map[string]string{
		"name": name, "metadata[anchor_organization_id]": "organization-id",
	}, "customer-intent", &result)
	require.NoError(t, err)
	assert.Equal(t, "cus_test", result.ID)
	require.Len(t, runner.args, 2)
	assert.Equal(t, []string{
		"post", "/v1/customers", "--color", "off", "--stripe-version", StripeVersion,
		"--stripe-account", "acct_expected", "--confirm", "--idempotency", "customer-intent",
		"-d", "metadata[anchor_organization_id]=organization-id", "-d", "name=" + name,
	}, runner.args[1])
	assert.NotContains(t, runner.args[1], "--live")
	assert.NotContains(t, runner.args[1], "--api-key")
}

func TestCLIRequestRejectsErrorEnvelopesWithoutProcessFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		response string
		message  string
	}{
		{"API error", `{"error":{"message":"No such customer"}}`, "No such customer"},
		{"live object", `{"id":"cus_live","livemode":true}`, "live object"},
		{"invalid JSON", `not JSON`, "invalid API response"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runner := &recordingRunner{results: []commandResult{
				{output: `{"account_id":"acct_expected","mode":"test"}`},
				{output: test.response},
			}}
			cli, err := NewCLI("acct_expected", runner)
			require.NoError(t, err)
			err = cli.Request(t.Context(), "get", "/v1/customers/cus_test", nil, "", &stripeObject{})
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestCLIRequestRejectsInvalidCommandsBeforeCallingStripe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		method string
		path   string
	}{
		{"patch", "/v1/customers/cus_test"},
		{"get", "https://example.com/v1/customers"},
		{"post", "/v2/core/accounts"},
	}
	for _, test := range cases {
		t.Run(test.method+test.path, func(t *testing.T) {
			t.Parallel()
			runner := &recordingRunner{}
			cli, err := NewCLI("acct_expected", runner)
			require.NoError(t, err)
			err = cli.Request(t.Context(), test.method, test.path, nil, "", nil)
			require.Error(t, err)
			assert.Empty(t, runner.args)
		})
	}
}

func TestCLIAccountReturnsSandboxIdentity(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{
		results: []commandResult{
			{output: `{"account_id":"acct_expected","display_name":"New Business","mode":"test"}`},
		},
	}
	cli, err := NewCLI("acct_expected", runner)
	require.NoError(t, err)
	account, err := cli.Account(t.Context())
	require.NoError(t, err)
	assert.Equal(t, Account{ID: "acct_expected", Name: "New Business", Mode: Sandbox}, account)
}
