package projection

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/events"
)

// APIKeyGrants mirrors what each Organization API key holds: the key holds
// each of its permissions on its Organization directly. Only an active,
// unexpired key holds anything. A key that expires by time leaves no event, so
// its tuples stay until the Product's next rebuild; decisions must refuse an
// expired key before they ask the store.
type APIKeyGrants struct{ tables Tables }

func NewAPIKeyGrants(tables Tables) APIKeyGrants { return APIKeyGrants{tables: tables} }

func (APIKeyGrants) Name() string { return "organization_api_keys" }

func (APIKeyGrants) Stale(event events.Event) ([]string, error) {
	if isOneOf(event, events.OrganizationAPIKeyCreated, events.OrganizationAPIKeyUpdated,
		events.OrganizationAPIKeyDeleted, events.OrganizationDeleted) {
		return keyOf(event, events.FieldOrganizationID)
	}
	return nil, nil
}

func (k APIKeyGrants) Keys(ctx context.Context, productID string) ([]string, error) {
	return k.tables.organizationIDs(ctx, productID)
}

func (k APIKeyGrants) Desired(ctx context.Context, productID, organizationID string) ([]fga.Tuple, error) {
	grants, err := transactor.QueryMapSlice(
		ctx,
		k.tables.db,
		postgres.SELECT(table.OrganizationAPIKeyPermissions.APIKeyID, table.OrganizationAPIKeyPermissions.PermissionName).
			FROM(table.OrganizationAPIKeyPermissions.INNER_JOIN(table.OrganizationAPIKeys,
				table.OrganizationAPIKeys.ID.EQ(table.OrganizationAPIKeyPermissions.APIKeyID))).
			WHERE(table.OrganizationAPIKeyPermissions.ProductID.EQ(postgres.String(productID)).
				AND(table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID))).
				AND(table.OrganizationAPIKeys.Status.EQ(postgres.String(string(orgapikey.StatusActive)))).
				AND(table.OrganizationAPIKeys.ExpiresAt.IS_NULL().
					OR(table.OrganizationAPIKeys.ExpiresAt.GT(postgres.TimestampzT(k.tables.now()))))).
			ORDER_BY(table.OrganizationAPIKeyPermissions.APIKeyID.ASC(),
				table.OrganizationAPIKeyPermissions.PermissionName.ASC()),
		func(row dbmodel.OrganizationAPIKeyPermissions) fga.Tuple {
			return fga.Tuple{
				Object:   model.Organization(organizationID),
				Relation: model.CanRelation(row.PermissionName),
				User:     model.APIKey(row.APIKeyID),
			}
		},
	).Value()
	return grants, err
}

// Stored reads the Organization's object and keeps the API keys' tuples: the
// object's assignment links belong to Assignments.
func (APIKeyGrants) Stored(ctx context.Context, store fga.Reader, organizationID string) ([]fga.Tuple, error) {
	tuples, err := store.Read(ctx, fga.Filter{Object: model.Organization(organizationID)})
	if err != nil {
		return nil, err
	}
	return functional.Slice(tuples).Filter(func(t fga.Tuple) bool {
		return fga.TypeOf(t.User) == model.TypeAPIKey
	}), nil
}
