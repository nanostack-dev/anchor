package stripeprototype

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

const StripeVersion = "2026-09-30.endive"

type StripeGateway interface {
	Account(context.Context) (Account, error)
	Request(context.Context, string, string, map[string]string, string, any) error
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
	return &CLI{expectedAccount: accountID, runner: runner}, nil
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

func (c *CLI) Request(
	ctx context.Context,
	method, path string,
	params map[string]string,
	idempotency string,
	result any,
) error {
	if err := validate.ValidateStruct(struct {
		Method string `validate:"oneof=get post delete"`
		Path   string `validate:"required,startswith=/v1/"`
	}{method, path}); err != nil {
		return err
	}
	if _, err := c.Account(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	args := []string{
		method,
		path,
		"--color",
		"off",
		"--stripe-version",
		StripeVersion,
		"--stripe-account",
		c.expectedAccount,
	}
	if method != "get" {
		args = append(args, "--confirm")
	}
	if idempotency != "" {
		args = append(args, "--idempotency", idempotency)
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "-d", key+"="+params[key])
	}
	output, err := c.runner.Run(ctx, args)
	if err != nil {
		return err
	}
	var envelope struct {
		Live  bool `json:"livemode"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(output, &envelope); err != nil {
		return errors.New("stripe CLI returned an invalid API response")
	}
	if envelope.Error != nil {
		return fmt.Errorf("stripe: %s", envelope.Error.Message)
	}
	if envelope.Live {
		return errors.New("stripe returned a live object; sandbox prototype refused it")
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(output, result)
}

type stripeObject struct {
	ID       string            `json:"id"`
	URL      string            `json:"url"`
	Status   string            `json:"status"`
	Live     bool              `json:"livemode"`
	Metadata map[string]string `json:"metadata"`
}

// Stripe expandable references can be returned as an ID or an expanded object.
type stripeReference string

func (reference *stripeReference) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		*reference = stripeReference(id)
		return nil
	}
	var object struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	*reference = stripeReference(object.ID)
	return nil
}

type checkoutSession struct {
	stripeObject
	Customer          stripeReference `json:"customer"`
	Subscription      stripeReference `json:"subscription"`
	ClientReferenceID string          `json:"client_reference_id"`
}

type subscription struct {
	ID                string            `json:"id"`
	Created           int64             `json:"created"`
	Customer          string            `json:"customer"`
	Status            string            `json:"status"`
	Live              bool              `json:"livemode"`
	CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
	CancelAt          int64             `json:"cancel_at"`
	Metadata          map[string]string `json:"metadata"`
	PendingUpdate     json.RawMessage   `json:"pending_update"`
	Items             struct {
		Data []struct {
			ID               string `json:"id"`
			CurrentPeriodEnd int64  `json:"current_period_end"`
			Price            struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}
