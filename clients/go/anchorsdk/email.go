package anchorsdk

import (
	"context"
	"fmt"
	"maps"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Email is the fluent entry point for sending transactional email. Obtain one
// with [Client.Email]:
//
//	err := c.Email().
//	    Template("welcome").
//	    To("new.user@company.com").
//	    Dedupe(signupID).
//	    Var("name", "Bob").
//	    Send(ctx)
//
// It centralizes the send policy — bounded retry, and trusting the persisted
// record's own status over the HTTP code — so every caller gets the same
// behaviour.
type Email struct{ c *Client }

// Email returns the fluent email sender bound to this client's product.
func (c *Client) Email() Email { return Email{c: c} }

// Template starts a send against the given published template slug.
func (e Email) Template(slug string) *SendBuilder {
	return &SendBuilder{c: e.c, req: nanoclient.EmailSendRequest{TemplateSlug: new(slug)}}
}

// TemplateID starts a send against a template referenced by ID.
func (e Email) TemplateID(templateID string) *SendBuilder {
	return &SendBuilder{c: e.c, req: nanoclient.EmailSendRequest{TemplateId: new(templateID)}}
}

// SendBuilder accumulates a single email send. Setter methods chain; [SendBuilder.Send]
// dispatches. A builder is single-use and not safe for concurrent mutation.
type SendBuilder struct {
	c   *Client
	req nanoclient.EmailSendRequest
}

// To sets the recipient address.
func (b *SendBuilder) To(address string) *SendBuilder {
	b.req.ToAddress = openapi_types.Email(address)
	return b
}

// ToName sets the recipient display name.
func (b *SendBuilder) ToName(name string) *SendBuilder {
	b.req.ToName = new(name)
	return b
}

// Dedupe sets the idempotency key. Anchor de-duplicates sends per (product, key);
// a repeat with the same key re-dispatches only if the prior record FAILED.
func (b *SendBuilder) Dedupe(key string) *SendBuilder {
	b.req.DedupeKey = new(key)
	return b
}

// Var sets a single template variable, allocating the variable map on first use.
func (b *SendBuilder) Var(key string, value any) *SendBuilder {
	if b.req.Variables == nil {
		b.req.Variables = &map[string]any{}
	}
	(*b.req.Variables)[key] = value
	return b
}

// Vars merges every entry from m into the template variables. A nil or empty map
// is a no-op.
func (b *SendBuilder) Vars(m map[string]any) *SendBuilder {
	if len(m) == 0 {
		return b
	}
	if b.req.Variables == nil {
		b.req.Variables = &map[string]any{}
	}
	maps.Copy(*b.req.Variables, m)
	return b
}

// Draft sends against the template's DRAFT version instead of PUBLISHED.
func (b *SendBuilder) Draft() *SendBuilder {
	b.req.UseDraft = new(true)
	return b
}

// Send dispatches the email under the client's retry policy.
//
// Only transient failures consume retries. A permanent failure — any 4xx other
// than 429, or a 201 whose persisted record status is FAILED — returns on the
// first attempt, so a caller holding an advisory lock does not spin. Classify it
// with errors.Is(err, [ErrPermanent]).
func (b *SendBuilder) Send(ctx context.Context) error {
	return b.c.retry.do(ctx, b.dispatch)
}

// dispatch performs one send attempt and classifies the outcome.
func (b *SendBuilder) dispatch(ctx context.Context) error {
	const op = "Email.Send"

	resp, err := b.c.api.SendEmailWithResponse(ctx, b.c.productID, b.req)
	if err != nil {
		return transportError(op, err)
	}

	code := resp.StatusCode()
	record, err := decode(op, code, resp.Body, resp.JSON201)
	if err != nil {
		// A 2xx whose body carries no record is not worth another attempt: Anchor
		// has already accepted the send, so retrying risks a duplicate dispatch.
		if code >= 200 && code < 300 {
			return permanentError(op, code, "send accepted but the response carried no record")
		}
		return err
	}

	// 201 does not mean delivered: a deduped replay of a previously FAILED record
	// also returns 201. Trust the record's own status, not the HTTP code.
	if record.Status == nanoclient.EmailSendStatusFAILED {
		return permanentError(op, code, fmt.Sprintf("send to %s recorded as %s", b.req.ToAddress, record.Status))
	}

	return nil
}
