package source

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"

	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/model"
	dbmodel "anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	rtservice "anchor/internal/resourcetype/service"
)

// Catalog gives each Product the model its catalog compiles to: its resource
// types and its permissions with their scope. Roles are data, so a role change
// never changes the model.
type Catalog struct {
	tables        Tables
	resourceTypes rtservice.Service
}

func NewCatalog(tables Tables, resourceTypes rtservice.Service) Catalog {
	return Catalog{tables: tables, resourceTypes: resourceTypes}
}

// ChangesModel reports whether the event changes a Product's catalog, and so
// its model.
func ChangesModel(event events.Event) bool {
	return isOneOf(event, events.ProductResourcePermissionCreated, events.ProductResourcePermissionUpdated,
		events.ProductResourcePermissionDeleted,
		events.ProductResourceTypeCreated, events.ProductResourceTypeUpdated, events.ProductResourceTypeDeleted)
}

func (c Catalog) Model(ctx context.Context, productID string) (fgasync.Model, error) {
	catalog, err := c.read(ctx, productID)
	if err != nil {
		return fgasync.Model{}, err
	}
	return fgasync.Model{
		Hash: model.Hash(catalog),
		Compile: func() (*openfgav1.AuthorizationModel, error) {
			compiled, compileErr := model.Compile(catalog)
			return compiled.Proto, compileErr
		},
	}, nil
}

func (c Catalog) read(ctx context.Context, productID string) (model.Catalog, error) {
	tree, err := c.resourceTypes.Tree(ctx, productID)
	if err != nil {
		return model.Catalog{}, err
	}
	permissions, err := transactor.Query[[]dbmodel.ProductResourcePermissions](ctx, c.tables.db,
		table.ProductResourcePermissions.SELECT(
			table.ProductResourcePermissions.Name, table.ProductResourcePermissions.ResourceTypeID,
		).WHERE(table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID))),
	).Value()
	if err != nil {
		return model.Catalog{}, err
	}
	var catalog model.Catalog
	for _, t := range tree.Types() {
		catalog.Types = append(catalog.Types, model.Type{
			Slug: t.Slug, ParentTypes: t.ParentTypes, NestsInItself: t.NestsInItself,
		})
	}
	for _, p := range permissions {
		scope := tree.ByID(p.ResourceTypeID).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).
			OrElse(resourcetype.OrganizationSlug)
		catalog.Permissions = append(catalog.Permissions, model.Permission{Name: p.Name, Scope: scope})
	}
	return catalog, nil
}
