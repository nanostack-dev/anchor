package anchorsdk

import (
	"context"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// Licensing is the product-scoped licensing facade, for the operations that
// address a set of organizations rather than one. Obtain it with
// [Client.Licensing]. Everything scoped to a single organization lives on
// [License] instead, reached through [Client.Organization].
type Licensing struct{ c *Client }

// Licensing returns the product-scoped licensing facade.
func (c *Client) Licensing() Licensing { return Licensing{c: c} }

// Migrate starts moving a set of organizations onto templateID: each license
// becomes a fresh copy of that template's values, with its provenance
// restamped. It is the one operation that changes which template an
// organization is on — [License.Adjust] deliberately cannot, so a bespoke
// change is never mistaken for a tier change.
//
//	run, err := c.Licensing().Migrate(proID).
//	    FromTemplate(betaID).
//	    Do(ctx)
//
// Name the organizations with [LicenseMigrationBuilder.Organizations] or select
// them with [LicenseMigrationBuilder.FromTemplate] — exactly one, never both.
// At most 500 organizations per run; a larger selection is refused with its
// count rather than truncated.
//
// A license field whose value differs from the template the organization
// currently holds is carried forward onto the new license, so a bespoke
// arrangement survives the move. [LicenseMigrationBuilder.DiscardDifferences]
// takes the target template whole instead.
func (l Licensing) Migrate(templateID string) *LicenseMigrationBuilder {
	return &LicenseMigrationBuilder{
		c:   l.c,
		req: nanoclient.OrganizationLicenseMigrationRequest{TemplateId: templateID},
	}
}

// LicenseMigrationBuilder accumulates one migration run. Setter methods chain;
// [LicenseMigrationBuilder.Do] sends. A builder is single-use and not safe for
// concurrent mutation.
type LicenseMigrationBuilder struct {
	c   *Client
	req nanoclient.OrganizationLicenseMigrationRequest
}

// Organizations names the organizations to move.
func (b *LicenseMigrationBuilder) Organizations(organizationIDs ...string) *LicenseMigrationBuilder {
	b.req.OrganizationIds = &organizationIDs
	return b
}

// FromTemplate selects every organization currently on templateID. An archived
// template is accepted, and is the common case: this is how a withdrawn tier is
// emptied. The selection is resolved inside the request, so an organization
// instantiated a moment ago cannot be missed.
func (b *LicenseMigrationBuilder) FromTemplate(templateID string) *LicenseMigrationBuilder {
	b.req.FromTemplateId = new(templateID)
	return b
}

// DiscardDifferences takes the target template whole, dropping every value that
// differs from the template the organization currently holds. Without it those
// values are carried forward.
//
// Anchor cannot tell a bespoke adjustment from a template that moved after the
// copy was taken, so neither choice is safe by itself. Compare the two
// templates before reaching for this: a value it discards is gone.
func (b *LicenseMigrationBuilder) DiscardDifferences() *LicenseMigrationBuilder {
	b.req.OnDifference = new(nanoclient.DISCARD)
	return b
}

// Do runs the migration and returns what happened to each organization. The
// call succeeds when the run completed; individual organizations may still have
// been skipped or failed, so read `Results` rather than treating a nil error as
// "everything moved".
//
// Every organization actually moved has its cached license read dropped, so the
// next [License.Get] through this client sees the new values rather than
// waiting out the TTL.
func (b *LicenseMigrationBuilder) Do(
	ctx context.Context,
) (*nanoclient.OrganizationLicenseMigrationResponse, error) {
	const op = "Licensing.Migrate"

	c := b.c

	migration, err := retrying(
		ctx, c,
		func(ctx context.Context) (*nanoclient.OrganizationLicenseMigrationResponse, error) {
			resp, err := c.api.MigrateOrganizationLicensesWithResponse(ctx, c.productID, b.req)
			if err != nil {
				return nil, transportError(op, err)
			}
			return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
		},
	)
	if err != nil {
		return nil, err
	}

	for _, result := range migration.Results {
		if result.Outcome == nanoclient.LicenseMigrationOutcomeCHANGED {
			c.licenses.delete(result.OrganizationId)
		}
	}

	return migration, nil
}
