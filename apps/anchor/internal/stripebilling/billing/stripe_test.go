//nolint:testpackage // Tests exercise the internal guarded SDK backend at the CLI boundary.
package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v87"
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
			_, err = cli.Client().V1Customers.Create(
				t.Context(),
				&stripe.CustomerCreateParams{IdempotencyKey: stripe.String("customer-intent")},
			)
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
	result, err := cli.Client().V1Customers.Create(t.Context(), &stripe.CustomerCreateParams{
		IdempotencyKey: stripe.String("customer-intent"),
		Name:           stripe.String(name),
		Metadata:       map[string]string{"anchor_organization_id": "organization-id"},
	})
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

func TestCLIRefundUsesOfficialSDKParametersAndStableIdempotency(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{results: []commandResult{
		{output: `{"account_id":"acct_expected","mode":"test"}`},
		{output: `{"id":"re_expected","charge":"ch_expected","amount":1500,"currency":"usd","status":"pending"}`},
	}}
	cli, err := NewCLI("acct_expected", runner)
	require.NoError(t, err)
	refund, err := cli.Client().V1Refunds.Create(t.Context(), &stripe.RefundCreateParams{
		IdempotencyKey: new("saved-charge-intent"), Charge: new("ch_expected"), Amount: new(int64(1500)),
		Metadata: map[string]string{refundActionMetadata: "action_expected"},
	})
	require.NoError(t, err)
	require.Equal(t, stripe.RefundStatusPending, refund.Status)
	require.Equal(t, []string{
		"post",
		"/v1/refunds",
		"--color",
		"off",
		"--stripe-version",
		StripeVersion,
		"--stripe-account",
		"acct_expected",
		"--confirm",
		"--idempotency",
		"saved-charge-intent",
		"-d",
		"amount=1500",
		"-d",
		"charge=ch_expected",
		"-d",
		"metadata[anchor_refund_action_id]=action_expected",
	}, runner.args[1])
}

func TestCLIRequestRejectsErrorEnvelopesWithoutProcessFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		response string
		message  string
	}{
		{"API error", `{"error":{"message":"No such customer"}}`, "No such customer"},
		{"OAuth credential error", `{"error":{"message":"Invalid API key provided: oak_fixture"}}`, "[redacted]"},
		{"live object", `{"id":"cus_live","livemode":true}`, "check the integration connection"},
		{"invalid JSON", `not JSON`, "check the integration connection"},
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
			_, err = cli.Client().V1Customers.Retrieve(t.Context(), "cus_test", nil)
			require.ErrorContains(t, err, test.message)
			assert.NotContains(t, err.Error(), "oak_fixture")
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
			backend := &guardedBackend{
				backend:   &cliBackend{cli: cli},
				accountID: "acct_expected",
				identity:  cli.Account,
			}
			err = backend.Call(
				test.method,
				test.path,
				"",
				&stripe.CustomerRetrieveParams{Context: t.Context()},
				&stripe.Customer{},
			)
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
