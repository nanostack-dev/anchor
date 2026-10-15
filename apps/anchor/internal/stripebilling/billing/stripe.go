package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/form"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

const StripeVersion = stripe.APIVersion

type StripeGateway interface {
	Account(context.Context) (Account, error)
	Client() *stripe.Client
}

type CommandRunner interface {
	Run(context.Context, []string) ([]byte, error)
}

const (
	identityTimeout = 30 * time.Second
	apiTimeout      = 45 * time.Second
)

type stripeCommand struct{}

func (stripeCommand) Run(ctx context.Context, args []string) ([]byte, error) {
	// Arguments are passed directly to the fixed Stripe binary, without a shell.
	command := exec.CommandContext(
		ctx,
		"stripe",
		args...)
	command.Env = functional.Slice(os.Environ()).Filter(func(value string) bool {
		return !strings.HasPrefix(value, "STRIPE_API_KEY=")
	})
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, errors.New("stripe CLI request failed; check sandbox login with stripe login")
	}
	return output, nil
}

type CLI struct {
	expectedAccount string
	runner          CommandRunner
	client          *stripe.Client
}

func NewCLI(accountID string, runner CommandRunner) (*CLI, error) {
	if err := validate.ValidateStruct(struct {
		AccountID string `validate:"required,startswith=acct_"`
	}{accountID}); err != nil {
		return nil, err
	}
	if runner == nil {
		runner = stripeCommand{}
	}
	cli := &CLI{expectedAccount: accountID, runner: runner}
	backend := &guardedBackend{backend: &cliBackend{cli: cli}, accountID: accountID, identity: cli.Account}
	cli.client = stripe.NewClient(
		"",
		stripe.WithBackends(&stripe.Backends{API: backend, Connect: backend, Uploads: backend}),
	)
	return cli, nil
}

func (c *CLI) Account(ctx context.Context) (Account, error) {
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	output, err := c.runner.Run(ctx, []string{"whoami", "--format", "json", "--color", "off"})
	if err != nil {
		return Account{}, err
	}
	var identity struct {
		ID   string `json:"account_id"`
		Name string `json:"display_name"`
		Mode string `json:"mode"`
	}
	if err = json.Unmarshal(output, &identity); err != nil {
		return Account{}, errors.New("stripe CLI did not return its account identity")
	}
	if identity.ID != c.expectedAccount || identity.Mode != "test" {
		return Account{}, errors.New("stripe CLI must select the configured sandbox account; live mode is refused")
	}
	return Account{ID: identity.ID, Name: identity.Name, Mode: "sandbox"}, nil
}

func (c *CLI) Client() *stripe.Client { return c.client }

type cliBackend struct {
	unsupportedBackend
	cli *CLI
}

func (b *cliBackend) Call(
	method, path, _ string,
	params stripe.ParamsContainer,
	result stripe.LastResponseSetter,
) error {
	values := &form.Values{}
	form.AppendTo(values, params)
	return b.CallRaw(method, path, "", []byte(values.Encode()), params.GetParams(), result)
}

func (b *cliBackend) CallRaw(
	method, path, _ string,
	body []byte,
	params *stripe.Params,
	result stripe.LastResponseSetter,
) error {
	ctx, cancel := context.WithTimeout(params.Context, apiTimeout)
	defer cancel()
	args := []string{
		strings.ToLower(method),
		path,
		"--color",
		"off",
		"--stripe-version",
		StripeVersion,
		"--stripe-account",
		b.cli.expectedAccount,
	}
	if method != http.MethodGet {
		args = append(args, "--confirm")
	}
	if params.IdempotencyKey != nil {
		args = append(args, "--idempotency", *params.IdempotencyKey)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return errors.New("stripe SDK parameters could not be encoded for the local CLI")
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range values[key] {
			args = append(args, "-d", key+"="+value)
		}
	}
	output, err := b.cli.runner.Run(ctx, args)
	if err != nil {
		return err
	}
	if len(output) > maximumStripeResponseBytes {
		return errors.New("stripe CLI response exceeds the permitted size")
	}
	var envelope struct {
		Error *stripe.Error `json:"error"`
	}
	if err = json.Unmarshal(output, &envelope); err != nil {
		return errors.New("stripe CLI returned an invalid API response")
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if err = refuseLivePayload(output); err != nil {
		return err
	}
	if err = json.Unmarshal(output, result); err != nil {
		return errors.New("stripe CLI returned an incompatible SDK response")
	}
	return nil
}
