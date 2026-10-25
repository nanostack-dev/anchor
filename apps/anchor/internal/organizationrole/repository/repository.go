// Package repository stores the roles each Organization owns. They live in
// product_roles beside the role templates, told apart by organization_id.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/pgkit/pglock"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/organizationrole"
	"anchor/internal/mapper"
)

// ErrDefaultExists is returned when two writers race to mark an
// Organization's default role.
var ErrDefaultExists = errors.New("organization already has a default role")

// ErrKeyExists is returned when another role of the Organization holds the key.
var ErrKeyExists = errors.New("organization already has a role with that key")

const (
	organizationDefaultIndex = "idx_product_roles_organization_default"
	organizationKeyIndex     = "idx_product_roles_organization_lower_key"
)

// TemplatePair names a role template an Organization has no copy of.
type TemplatePair struct {
	OrganizationID string `alias:"organizations.id"`
	TemplateID     string `alias:"product_roles.id"`
}

type Repository interface {
	// Find reads an Organization's role by a public ID: a template's ID names
	// the copy, a custom role's ID names itself, and so does a copy's own ID.
	Find(
		ctx context.Context,
		productID, organizationID, roleID string,
	) (functional.Option[organizationrole.Role], error)
	// Resolve reads the role a request names inside an Organization: the
	// Organization's own role with that public ID (a template's ID names its
	// copy), or else the Product's role template itself, for an Organization
	// that has no copy of it yet.
	Resolve(
		ctx context.Context,
		productID, organizationID, roleID string,
	) (functional.Option[organizationrole.Role], error)
	// FindChosenDefault reads the role the Organization chose as its default.
	FindChosenDefault(
		ctx context.Context,
		productID, organizationID string,
	) (functional.Option[organizationrole.Role], error)
	// FindDefaultTemplateID reads the Product's default role template.
	FindDefaultTemplateID(ctx context.Context, productID string) (functional.Option[string], error)
	Search(
		ctx context.Context, productID, organizationID string,
		request search.Request[organizationrole.Filter, organizationrole.SortField],
	) (search.Result[organizationrole.Role], error)
	// MissingCopies lists up to limit (organization, template) pairs with no
	// copy, for one Organization, or for every Organization of the product
	// when organizationID is empty. templateID, when set, narrows to it.
	MissingCopies(
		ctx context.Context,
		productID, organizationID, templateID string,
		limit int64,
	) ([]TemplatePair, error)
	// InsertCopies copies each template into its Organization and returns how
	// many copies it made. It reads the templates under a share lock, so a
	// template change commits wholly before or after it. A pair copied
	// concurrently, or whose key another role of the Organization holds, is
	// skipped.
	InsertCopies(ctx context.Context, productID string, pairs []TemplatePair) (int, error)
	// SyncCopies makes every copy of the template match it: key, name,
	// description, scope and permissions.
	SyncCopies(ctx context.Context, productID, templateID string) error
	SetDefault(ctx context.Context, productID, organizationID, rowID string, isDefault bool) error
	ClearDefault(ctx context.Context, productID, organizationID string) error
	// RepointMemberships moves memberships that still hold a template onto
	// their Organization's copy, in one Organization, or in every
	// Organization of the product when organizationID is empty.
	RepointMemberships(ctx context.Context, productID, organizationID string) (int64, error)
	// ShareRole holds a key share lock on an Organization's own role for the
	// transaction and reports whether it still exists.
	ShareRole(ctx context.Context, productID, rowID string) (bool, error)
	// LockRole holds an Organization's role row for the transaction.
	LockRole(ctx context.Context, productID, organizationID, rowID string) error
	// ShareTemplate holds a share lock on the template for the transaction and
	// reports whether it still exists.
	ShareTemplate(ctx context.Context, productID, templateID string) (bool, error)
	// TemplateIDsInternal lists every role template of the product. Only the
	// startup backfill calls it.
	TemplateIDsInternal(ctx context.Context, productID string) ([]string, error)
	// BackfillTemplateKeysInternal gives a key to every template a release
	// before keys wrote. Only the startup backfill calls it.
	BackfillTemplateKeysInternal(ctx context.Context, productID string) (int64, error)

	// LockOrganization holds the Organization's row for the transaction, so
	// custom roles are counted against the limit one writer at a time.
	LockOrganization(ctx context.Context, productID, organizationID string) (bool, error)
	// LockKey holds a role key of the product for the transaction, so a role
	// template and a custom role can't take the same key at once.
	LockKey(ctx context.Context, productID, key string) error
	// KeyTaken reports whether a role template of the product, or a custom
	// role of any of its Organizations, holds the key. A non-empty
	// organizationID narrows the custom roles to that Organization.
	KeyTaken(ctx context.Context, productID, organizationID, key string, templates bool) (bool, error)
	CountCustom(ctx context.Context, productID, organizationID string) (int64, error)
	CreateCustom(ctx context.Context, role organizationrole.Role) error
	// Rewrite sets the role's name, description and customized flag.
	Rewrite(ctx context.Context, role organizationrole.Role) error
	// ReplacePermissions makes the role bundle exactly the named permissions.
	ReplacePermissions(ctx context.Context, productID, rowID string, names []string) error
	// Reset returns a copy to its template's name, description and
	// permissions, and clears its customized flag.
	Reset(ctx context.Context, productID, rowID, templateID string) error
	DeleteCustom(ctx context.Context, productID, organizationID, rowID string) error
	// CountHolders counts the memberships that hold the role and the pending
	// invitations that name it.
	CountHolders(ctx context.Context, productID, organizationID string, role organizationrole.Role) (int64, error)
	// DetachCopies turns the template's copies that a member holds, a pending
	// invitation names, or the Organization customized into custom roles of
	// their Organization, moves those invitations onto them, and returns them.
	DetachCopies(ctx context.Context, productID, templateID string) ([]organizationrole.Role, error)
	// OrganizationsOnTemplate lists the Organizations where a membership holds
	// the template itself or a pending invitation names it.
	OrganizationsOnTemplate(ctx context.Context, productID, templateID string) ([]string, error)
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

type roleRow struct {
	model.ProductRoles
	Permissions []model.ProductRoleResourcePermissions `alias:"product_role_resource_permissions"`
}

func toDomain(row model.ProductRoles, permissions []model.ProductRoleResourcePermissions) organizationrole.Role {
	return organizationrole.Role{
		RowID:          row.ID,
		ProductID:      row.ProductID,
		OrganizationID: functional.FromPtr(row.OrganizationID).OrElse(""),
		TemplateID:     row.TemplateID,
		Key:            mapper.RoleKey(row),
		Name:           row.Name,
		Description:    functional.FromPtr(row.Description).OrElse(""),
		ResourceTypeID: row.ResourceTypeID,
		IsDefault:      row.IsDefault,
		Customized:     row.Customized,
		Permissions: functional.Slice(permissions).Map(
			func(p model.ProductRoleResourcePermissions) string { return p.PermissionName },
		),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func inOrganization(productID, organizationID string) postgres.BoolExpression {
	return table.ProductRoles.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductRoles.OrganizationID.EQ(postgres.String(organizationID)))
}

// byPublicID also accepts a copy's own row ID, which no response exposes but
// memberships store.
func byPublicID(ids ...string) postgres.BoolExpression {
	list := jetx.ToStringExpressions(ids)
	return table.ProductRoles.PublicID.IN(list...).OR(table.ProductRoles.ID.IN(list...))
}

func (r *repositoryImpl) findOne(
	ctx context.Context, where postgres.BoolExpression,
) (functional.Option[organizationrole.Role], error) {
	stmt := postgres.SELECT(
		table.ProductRoles.AllColumns,
		table.ProductRoleResourcePermissions.AllColumns,
	).FROM(
		table.ProductRoles.LEFT_JOIN(
			table.ProductRoleResourcePermissions,
			table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
		),
	).WHERE(where)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, func(row roleRow) organizationrole.Role {
		return toDomain(row.ProductRoles, row.Permissions)
	})
}

func (r *repositoryImpl) Find(
	ctx context.Context, productID, organizationID, roleID string,
) (functional.Option[organizationrole.Role], error) {
	return r.findOne(ctx, inOrganization(productID, organizationID).AND(byPublicID(roleID)))
}

func (r *repositoryImpl) Resolve(
	ctx context.Context, productID, organizationID, roleID string,
) (functional.Option[organizationrole.Role], error) {
	if roleID == "" {
		return functional.None[organizationrole.Role](), nil
	}
	inProduct := table.ProductRoles.ProductID.EQ(postgres.String(productID))
	own, err := r.findOne(ctx, inProduct.
		AND(table.ProductRoles.OrganizationID.EQ(postgres.String(organizationID))).
		AND(table.ProductRoles.PublicID.EQ(postgres.String(roleID)).OR(table.ProductRoles.ID.EQ(postgres.String(roleID)))),
	)
	if err != nil || own.IsPresent() {
		return own, err
	}
	return r.findOne(ctx, inProduct.
		AND(table.ProductRoles.OrganizationID.IS_NULL()).
		AND(table.ProductRoles.ID.EQ(postgres.String(roleID))))
}

func (r *repositoryImpl) FindChosenDefault(
	ctx context.Context, productID, organizationID string,
) (functional.Option[organizationrole.Role], error) {
	return r.findOne(ctx, inOrganization(productID, organizationID).AND(table.ProductRoles.IsDefault.IS_TRUE()))
}

func (r *repositoryImpl) FindDefaultTemplateID(
	ctx context.Context,
	productID string,
) (functional.Option[string], error) {
	return transactor.QueryOptionalMap(ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoles.OrganizationID.IS_NULL()).
				AND(table.ProductRoles.IsDefault.IS_TRUE()),
		),
		func(row model.ProductRoles) string { return row.ID },
	)
}

func (r *repositoryImpl) Search(
	ctx context.Context, productID, organizationID string,
	request search.Request[organizationrole.Filter, organizationrole.SortField],
) (search.Result[organizationrole.Role], error) {
	where := inOrganization(productID, organizationID)
	if request.FullTextSearch != nil {
		pattern := postgres.String("%" + strings.ToLower(*request.FullTextSearch) + "%")
		where = where.AND(
			postgres.LOWER(table.ProductRoles.Name).LIKE(pattern).
				OR(postgres.LOWER(table.ProductRoles.Key).LIKE(pattern)),
		)
	}
	if request.Filter != nil {
		where = where.AND(filterCondition(*request.Filter))
	}

	order, err := transactor.StrictKeys(
		request.Sort,
		map[organizationrole.SortField]transactor.Key[model.ProductRoles]{
			organizationrole.SortFieldCreatedAt: transactor.TimeKey(
				table.ProductRoles.CreatedAt, func(row model.ProductRoles) time.Time { return row.CreatedAt },
			),
			organizationrole.SortFieldKey: transactor.NullableStringKey(
				table.ProductRoles.Key, func(row model.ProductRoles) *string { return row.Key },
			),
			organizationrole.SortFieldName: transactor.StringKey(
				table.ProductRoles.Name, func(row model.ProductRoles) string { return row.Name },
			),
		},
		transactor.StringKey(table.ProductRoles.PublicID, func(row model.ProductRoles) string {
			return functional.FromPtr(row.PublicID).OrElse(row.ID)
		}),
	)
	if err != nil {
		return search.Result[organizationrole.Role]{}, err
	}
	page, err := transactor.Page(
		r.db, func(row model.ProductRoles) model.ProductRoles { return row }, table.ProductRoles.AllColumns,
	).
		From(table.ProductRoles).
		Where(where).
		Keyset(order).
		Run(ctx, request.Pagination).
		Value()
	if err != nil {
		return search.Result[organizationrole.Role]{}, err
	}
	permissions, err := r.permissionsOf(ctx, functional.Slice(page.Items).Map(
		func(row model.ProductRoles) string { return row.ID }))
	if err != nil {
		return search.Result[organizationrole.Role]{}, err
	}
	items := functional.Slice(page.Items).Map(func(row model.ProductRoles) organizationrole.Role {
		return toDomain(row, permissions[row.ID])
	})
	return search.Result[organizationrole.Role]{
		Items: items, Total: page.Total, Count: len(items), NextCursor: page.NextCursor,
	}, nil
}

func (r *repositoryImpl) permissionsOf(
	ctx context.Context, roleIDs []string,
) (map[string]functional.Seq[model.ProductRoleResourcePermissions], error) {
	if len(roleIDs) == 0 {
		return map[string]functional.Seq[model.ProductRoleResourcePermissions]{}, nil
	}
	rows, err := transactor.Query[[]model.ProductRoleResourcePermissions](ctx, r.db,
		table.ProductRoleResourcePermissions.SELECT(table.ProductRoleResourcePermissions.AllColumns).
			WHERE(table.ProductRoleResourcePermissions.ProductRoleID.IN(jetx.ToStringExpressions(roleIDs)...)).
			ORDER_BY(table.ProductRoleResourcePermissions.PermissionName.ASC()),
	).Value()
	if err != nil {
		return nil, err
	}
	return functional.Slice(rows).GroupBy(
		func(row model.ProductRoleResourcePermissions) string { return row.ProductRoleID },
	), nil
}

func (r *repositoryImpl) MissingCopies(
	ctx context.Context, productID, organizationID, templateID string, limit int64,
) ([]TemplatePair, error) {
	copies := table.ProductRoles.AS("copies")
	copyOrganization := copies.OrganizationID
	copyTemplate := copies.TemplateID
	where := table.Organizations.ProductID.EQ(postgres.String(productID)).
		AND(postgres.NOT(postgres.EXISTS(
			postgres.SELECT(postgres.Int(1)).FROM(copies).WHERE(
				copyOrganization.EQ(table.Organizations.ID).AND(copyTemplate.EQ(table.ProductRoles.ID)),
			),
		)))
	if organizationID != "" {
		where = where.AND(table.Organizations.ID.EQ(postgres.String(organizationID)))
	}
	if templateID != "" {
		where = where.AND(table.ProductRoles.ID.EQ(postgres.String(templateID)))
	}
	stmt := postgres.SELECT(
		table.Organizations.ID.AS("organizations.id"),
		table.ProductRoles.ID.AS("product_roles.id"),
	).FROM(
		table.Organizations.INNER_JOIN(
			table.ProductRoles,
			table.ProductRoles.ProductID.EQ(table.Organizations.ProductID).
				AND(table.ProductRoles.OrganizationID.IS_NULL()),
		),
	).WHERE(where).ORDER_BY(table.Organizations.ID.ASC(), table.ProductRoles.ID.ASC()).LIMIT(limit)
	return transactor.Query[[]TemplatePair](ctx, r.db, stmt).Value()
}

func (r *repositoryImpl) InsertCopies(ctx context.Context, productID string, pairs []TemplatePair) (int, error) {
	if len(pairs) == 0 {
		return 0, nil
	}
	templateIDs := functional.Distinct(functional.Slice(pairs).Map(func(p TemplatePair) string { return p.TemplateID }))
	templates, err := transactor.Query[[]model.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.AllColumns).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoles.OrganizationID.IS_NULL()).
				AND(table.ProductRoles.ID.IN(jetx.ToStringExpressions(templateIDs)...)),
		).ORDER_BY(table.ProductRoles.ID.ASC()).FOR(postgres.SHARE()),
	).Value()
	if err != nil {
		return 0, err
	}
	byID := functional.Slice(templates).ToMap(func(t model.ProductRoles) string { return t.ID })
	permissions, err := r.permissionsOf(ctx, templateIDs)
	if err != nil {
		return 0, err
	}

	held, err := r.heldKeys(ctx, productID, pairs)
	if err != nil {
		return 0, err
	}
	var copies []model.ProductRoles
	for _, pair := range pairs {
		template, ok := byID[pair.TemplateID]
		if !ok {
			continue
		}
		key := mapper.RoleKey(template)
		if held[pair.OrganizationID+"\x00"+strings.ToLower(key)] {
			continue
		}
		organizationID, templateID := pair.OrganizationID, template.ID
		copies = append(copies, model.ProductRoles{
			ID:             organizationrole.NewRowID(),
			ProductID:      productID,
			OrganizationID: &organizationID,
			TemplateID:     &templateID,
			Key:            &key,
			Name:           template.Name,
			Description:    template.Description,
			ResourceTypeID: template.ResourceTypeID,
		})
	}
	if len(copies) == 0 {
		return 0, nil
	}
	inserted, err := transactor.Query[[]model.ProductRoles](ctx, r.db,
		table.ProductRoles.INSERT(
			table.ProductRoles.ID, table.ProductRoles.ProductID, table.ProductRoles.OrganizationID,
			table.ProductRoles.TemplateID, table.ProductRoles.Key, table.ProductRoles.Name,
			table.ProductRoles.Description, table.ProductRoles.ResourceTypeID,
		).MODELS(copies).
			// Two writers copying one template into one Organization at once
			// meet on the copy's (organization, template) index or on its key
			// index, whichever Postgres checks first; either way the other
			// writer made the copy. Keys of an Organization's own roles never
			// clash with a template's: the key rules refuse that up front.
			ON_CONFLICT().DO_NOTHING().
			RETURNING(table.ProductRoles.ID, table.ProductRoles.TemplateID),
	).Value()
	if err != nil {
		return 0, err
	}
	var rows []model.ProductRoleResourcePermissions
	for _, copied := range inserted {
		for _, permission := range permissions[functional.FromPtr(copied.TemplateID).OrElse("")] {
			rows = append(rows, model.ProductRoleResourcePermissions{
				ID:             ids.MustNew("product_role_resource_permission"),
				ProductID:      productID,
				ProductRoleID:  copied.ID,
				PermissionName: permission.PermissionName,
			})
		}
	}
	return len(inserted), r.insertPermissions(ctx, rows)
}

// heldKeys lists the keys the Organizations of the pairs already hold. A
// custom role made while a release that didn't know custom keys ran may hold a
// template's key; that template is then not copied into its Organization,
// rather than failing every write there.
func (r *repositoryImpl) heldKeys(
	ctx context.Context,
	productID string,
	pairs []TemplatePair,
) (map[string]bool, error) {
	organizations := functional.Distinct(
		functional.Slice(pairs).Map(func(p TemplatePair) string { return p.OrganizationID }),
	)
	rows, err := transactor.Query[[]model.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.OrganizationID, table.ProductRoles.Key).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoles.OrganizationID.IN(jetx.ToStringExpressions(organizations)...)),
		),
	).Value()
	if err != nil {
		return nil, err
	}
	held := make(map[string]bool, len(rows))
	for _, row := range rows {
		held[functional.FromPtr(row.OrganizationID).OrElse("")+"\x00"+strings.ToLower(functional.FromPtr(row.Key).OrElse(""))] = true
	}
	return held, nil
}

func (r *repositoryImpl) insertPermissions(ctx context.Context, rows []model.ProductRoleResourcePermissions) error {
	if len(rows) == 0 {
		return nil
	}
	return transactor.Exec(ctx, r.db,
		table.ProductRoleResourcePermissions.INSERT(
			table.ProductRoleResourcePermissions.ID, table.ProductRoleResourcePermissions.ProductID,
			table.ProductRoleResourcePermissions.ProductRoleID, table.ProductRoleResourcePermissions.PermissionName,
		).MODELS(rows).ON_CONFLICT().DO_NOTHING(),
	).Err()
}

type copyPermission struct {
	CopyID         string `alias:"copies.id"`
	PermissionName string `alias:"product_role_resource_permissions.permission_name"`
}

func (r *repositoryImpl) SyncCopies(ctx context.Context, productID, templateID string) error {
	template, err := transactor.QueryOptional[model.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.AllColumns).WHERE(
			table.ProductRoles.ID.EQ(postgres.String(templateID)).
				AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))).
				AND(table.ProductRoles.OrganizationID.IS_NULL()),
		),
	)
	if err != nil || template.IsAbsent() {
		return err
	}
	tpl := template.Value()
	key := mapper.RoleKey(tpl)
	isCopy := table.ProductRoles.TemplateID.EQ(postgres.String(templateID)).
		AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))).
		AND(table.ProductRoles.Customized.IS_FALSE())
	synced := model.ProductRoles{
		Key:            &key,
		Name:           tpl.Name,
		Description:    tpl.Description,
		ResourceTypeID: tpl.ResourceTypeID,
	}
	if err = transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(
			table.ProductRoles.Key, table.ProductRoles.Name,
			table.ProductRoles.Description, table.ProductRoles.ResourceTypeID,
		).
			MODEL(synced).
			WHERE(isCopy.AND(
				table.ProductRoles.Key.IS_DISTINCT_FROM(postgres.String(key)).
					OR(table.ProductRoles.Name.IS_DISTINCT_FROM(postgres.String(tpl.Name))).
					OR(table.ProductRoles.Description.IS_DISTINCT_FROM(nullableString(tpl.Description))).
					OR(table.ProductRoles.ResourceTypeID.IS_DISTINCT_FROM(nullableString(tpl.ResourceTypeID))),
			)),
	).Err(); err != nil {
		return err
	}

	granted := table.ProductRoleResourcePermissions.SELECT(table.ProductRoleResourcePermissions.PermissionName).
		WHERE(table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(templateID)))
	if err = transactor.Exec(ctx, r.db,
		table.ProductRoleResourcePermissions.DELETE().WHERE(
			table.ProductRoleResourcePermissions.ProductRoleID.IN(
				table.ProductRoles.SELECT(table.ProductRoles.ID).WHERE(isCopy),
			).AND(table.ProductRoleResourcePermissions.PermissionName.NOT_IN(granted)),
		),
	).Err(); err != nil {
		return err
	}

	copies := table.ProductRoles.AS("copies")
	held := table.ProductRoleResourcePermissions.AS("held")
	missing, err := transactor.Query[[]copyPermission](ctx, r.db,
		postgres.SELECT(
			copies.ID.AS("copies.id"),
			table.ProductRoleResourcePermissions.PermissionName.AS("product_role_resource_permissions.permission_name"),
		).FROM(
			copies.INNER_JOIN(table.ProductRoleResourcePermissions,
				table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(templateID))),
		).WHERE(
			copies.TemplateID.EQ(postgres.String(templateID)).
				AND(copies.Customized.IS_FALSE()).
				AND(postgres.NOT(postgres.EXISTS(
					postgres.SELECT(postgres.Int(1)).FROM(held).WHERE(
						held.ProductRoleID.
							EQ(copies.ID).
							AND(held.PermissionName.
								EQ(table.ProductRoleResourcePermissions.PermissionName)),
					),
				))),
		),
	).Value()
	if err != nil {
		return err
	}
	return r.insertPermissions(ctx, functional.Slice(missing).Map(
		func(m copyPermission) model.ProductRoleResourcePermissions {
			return model.ProductRoleResourcePermissions{
				ID:             ids.MustNew("product_role_resource_permission"),
				ProductID:      productID,
				ProductRoleID:  m.CopyID,
				PermissionName: m.PermissionName,
			}
		}))
}

func (r *repositoryImpl) ClearDefault(ctx context.Context, productID, organizationID string) error {
	return transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.IsDefault).SET(postgres.Bool(false)).WHERE(
			inOrganization(productID, organizationID).AND(table.ProductRoles.IsDefault.IS_TRUE()),
		),
	).Err()
}

func (r *repositoryImpl) SetDefault(
	ctx context.Context, productID, organizationID, rowID string, isDefault bool,
) error {
	return transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.IsDefault).SET(postgres.Bool(isDefault)).WHERE(
			inOrganization(productID, organizationID).AND(table.ProductRoles.ID.EQ(postgres.String(rowID))),
		),
	).OnUnique(ErrDefaultExists, organizationDefaultIndex).Err()
}

func (r *repositoryImpl) RepointMemberships(ctx context.Context, productID, organizationID string) (int64, error) {
	where := table.ProductRoles.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductRoles.TemplateID.EQ(table.OrganizationMemberships.ProductRoleID)).
		AND(table.ProductRoles.OrganizationID.EQ(table.OrganizationMemberships.OrganizationID))
	if organizationID != "" {
		where = where.AND(table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID)))
	}
	stmt := table.OrganizationMemberships.UPDATE(table.OrganizationMemberships.ProductRoleID).
		SET(table.ProductRoles.ID).
		FROM(table.ProductRoles).
		WHERE(where)
	return execRows(ctx, r.db, stmt)
}

func (r *repositoryImpl) ShareRole(ctx context.Context, productID, rowID string) (bool, error) {
	found, err := transactor.QueryOptional[model.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID).WHERE(
			table.ProductRoles.ID.EQ(postgres.String(rowID)).
				AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))),
		).FOR(postgres.KEY_SHARE()),
	)
	return found.IsPresent(), err
}

func (r *repositoryImpl) LockRole(ctx context.Context, productID, organizationID, rowID string) error {
	return transactor.Exec(ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID).WHERE(
			inOrganization(productID, organizationID).AND(table.ProductRoles.ID.EQ(postgres.String(rowID))),
		).FOR(postgres.UPDATE()),
	).Err()
}

func (r *repositoryImpl) ShareTemplate(ctx context.Context, productID, templateID string) (bool, error) {
	found, err := transactor.QueryOptional[model.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID).WHERE(
			table.ProductRoles.ID.EQ(postgres.String(templateID)).
				AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))).
				AND(table.ProductRoles.OrganizationID.IS_NULL()),
		).FOR(postgres.SHARE()),
	)
	return found.IsPresent(), err
}

func (r *repositoryImpl) TemplateIDsInternal(ctx context.Context, productID string) ([]string, error) {
	return transactor.QueryMapSlice(ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.ID).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoles.OrganizationID.IS_NULL()),
		).ORDER_BY(table.ProductRoles.ID.ASC()),
		func(row model.ProductRoles) string { return row.ID },
	).Value()
}

func (r *repositoryImpl) BackfillTemplateKeysInternal(ctx context.Context, productID string) (int64, error) {
	others := table.ProductRoles.AS("others")
	unkeyed := table.ProductRoles.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductRoles.OrganizationID.IS_NULL()).
		AND(table.ProductRoles.Key.IS_NULL())
	byName, err := execRows(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.Key).SET(postgres.LOWER(table.ProductRoles.Name)).WHERE(
			unkeyed.AND(postgres.NOT(postgres.EXISTS(
				postgres.SELECT(postgres.Int(1)).FROM(others).WHERE(
					others.ProductID.EQ(postgres.String(productID)).
						AND(others.OrganizationID.IS_NULL().OR(others.TemplateID.IS_NULL())).
						AND(postgres.LOWER(others.Key).
							EQ(postgres.LOWER(table.ProductRoles.Name))),
				),
			))),
		),
	)
	if err != nil {
		return byName, err
	}
	// A name another template or a custom role already uses as its key falls
	// back to the template's own ID, which nothing else can hold.
	byID, err := execRows(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.Key).SET(table.ProductRoles.ID).WHERE(unkeyed),
	)
	return byName + byID, err
}

// nullableString is a value for a comparison; a fresh NULL each time, since
// jet's shared NULL must not be wrapped.
func nullableString(value *string) postgres.StringExpression {
	if value == nil {
		return postgres.StringExp(postgres.Raw("NULL"))
	}
	return postgres.String(*value)
}

func execRows(ctx context.Context, db *sql.DB, stmt postgres.Statement) (int64, error) {
	result, err := stmt.ExecContext(ctx, transactor.Executor(ctx, db))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func filterCondition(filter organizationrole.Filter) postgres.BoolExpression {
	condition := postgres.Bool(true)
	if len(filter.IDs) > 0 {
		condition = condition.AND(byPublicID(filter.IDs...))
	}
	if len(filter.Keys) > 0 {
		condition = condition.AND(postgres.LOWER(table.ProductRoles.Key).IN(jetx.ToStringExpressions(
			functional.Slice(filter.Keys).Map(strings.ToLower))...))
	}
	if filter.Scopes.Active() {
		scoped := table.ProductRoles.ResourceTypeID.IN(jetx.ToStringExpressions(filter.Scopes.IDs)...)
		if filter.Scopes.IncludeUnscoped {
			scoped = scoped.OR(table.ProductRoles.ResourceTypeID.IS_NULL())
		}
		condition = condition.AND(scoped)
	}
	return condition
}

func (r *repositoryImpl) LockOrganization(ctx context.Context, productID, organizationID string) (bool, error) {
	found, err := transactor.QueryOptional[model.Organizations](ctx, r.db,
		table.Organizations.SELECT(table.Organizations.ID).WHERE(
			table.Organizations.ID.EQ(postgres.String(organizationID)).
				AND(table.Organizations.ProductID.EQ(postgres.String(productID))),
		).FOR(postgres.NO_KEY_UPDATE()),
	)
	return found.IsPresent(), err
}

func (r *repositoryImpl) LockKey(ctx context.Context, productID, key string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`,
		pglock.KeyHash("role-key:"+productID+":"+strings.ToLower(key)))
	return err
}

func (r *repositoryImpl) KeyTaken(
	ctx context.Context, productID, organizationID, key string, templates bool,
) (bool, error) {
	custom := table.ProductRoles.OrganizationID.IS_NOT_NULL().AND(table.ProductRoles.TemplateID.IS_NULL())
	if organizationID != "" {
		custom = table.ProductRoles.OrganizationID.EQ(postgres.String(organizationID))
	}
	holders := custom
	if templates {
		holders = holders.OR(table.ProductRoles.OrganizationID.IS_NULL())
	}
	count, err := transactor.QueryCount(ctx, r.db,
		table.ProductRoles.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)).
				AND(postgres.LOWER(table.ProductRoles.Key).EQ(postgres.String(strings.ToLower(key)))).
				AND(holders),
		),
	).Value()
	return count > 0, err
}

func (r *repositoryImpl) CountCustom(ctx context.Context, productID, organizationID string) (int64, error) {
	return transactor.QueryCount(ctx, r.db,
		table.ProductRoles.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			inOrganization(productID, organizationID).AND(table.ProductRoles.TemplateID.IS_NULL()),
		),
	).Value()
}

func (r *repositoryImpl) CreateCustom(ctx context.Context, role organizationrole.Role) error {
	organizationID := role.OrganizationID
	key := role.Key
	description := functional.OptionOf(role.Description, role.Description != "").ToPtr()
	if err := transactor.Exec(ctx, r.db,
		table.ProductRoles.INSERT(
			table.ProductRoles.ID, table.ProductRoles.ProductID, table.ProductRoles.OrganizationID,
			table.ProductRoles.Key, table.ProductRoles.Name, table.ProductRoles.Description,
			table.ProductRoles.ResourceTypeID,
		).MODEL(model.ProductRoles{
			ID: role.RowID, ProductID: role.ProductID, OrganizationID: &organizationID, Key: &key,
			Name: role.Name, Description: description, ResourceTypeID: role.ResourceTypeID,
		}),
	).OnUnique(ErrKeyExists, organizationKeyIndex).Err(); err != nil {
		return err
	}
	return r.ReplacePermissions(ctx, role.ProductID, role.RowID, role.Permissions)
}

func (r *repositoryImpl) Rewrite(ctx context.Context, role organizationrole.Role) error {
	description := functional.OptionOf(role.Description, role.Description != "").ToPtr()
	return transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(
			table.ProductRoles.Name, table.ProductRoles.Description, table.ProductRoles.Customized,
			table.ProductRoles.UpdatedAt,
		).SET(postgres.String(role.Name), descriptionValue(description), postgres.Bool(role.Customized), postgres.NOW()).
			WHERE(inOrganization(role.ProductID, role.OrganizationID).
				AND(table.ProductRoles.ID.EQ(postgres.String(role.RowID)))),
	).Err()
}

func descriptionValue(description *string) postgres.Expression {
	if description == nil {
		return postgres.Raw("NULL")
	}
	return postgres.String(*description)
}

func (r *repositoryImpl) ReplacePermissions(ctx context.Context, productID, rowID string, names []string) error {
	remove := table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(rowID))
	if len(names) > 0 {
		remove = remove.AND(
			table.ProductRoleResourcePermissions.PermissionName.NOT_IN(jetx.ToStringExpressions(names)...),
		)
	}
	if err := transactor.Exec(ctx, r.db, table.ProductRoleResourcePermissions.DELETE().WHERE(remove)).
		Err(); err != nil {
		return err
	}
	return r.insertPermissions(ctx, functional.Slice(names).Map(func(name string) model.ProductRoleResourcePermissions {
		return model.ProductRoleResourcePermissions{
			ID:             ids.MustNew("product_role_resource_permission"),
			ProductID:      productID,
			ProductRoleID:  rowID,
			PermissionName: name,
		}
	}))
}

func (r *repositoryImpl) Reset(ctx context.Context, productID, rowID, templateID string) error {
	if err := transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.Customized).SET(postgres.Bool(false)).WHERE(
			table.ProductRoles.ID.EQ(postgres.String(rowID)).
				AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))),
		),
	).Err(); err != nil {
		return err
	}
	return r.SyncCopies(ctx, productID, templateID)
}

func (r *repositoryImpl) DeleteCustom(ctx context.Context, productID, organizationID, rowID string) error {
	return transactor.Exec(ctx, r.db,
		table.ProductRoles.DELETE().WHERE(
			inOrganization(productID, organizationID).
				AND(table.ProductRoles.ID.EQ(postgres.String(rowID))).
				AND(table.ProductRoles.TemplateID.IS_NULL()),
		),
	).Err()
}

func (r *repositoryImpl) CountHolders(
	ctx context.Context, productID, organizationID string, role organizationrole.Role,
) (int64, error) {
	members, err := transactor.QueryCount(ctx, r.db,
		table.OrganizationMemberships.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.OrganizationMemberships.OrganizationID.EQ(postgres.String(organizationID)).
				AND(table.OrganizationMemberships.ProductRoleID.EQ(postgres.String(role.RowID))),
		),
	).Value()
	if err != nil {
		return 0, err
	}
	invited, err := transactor.QueryCount(ctx, r.db,
		table.OrganizationInvitations.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.OrganizationInvitations.ProductID.EQ(postgres.String(productID)).
				AND(table.OrganizationInvitations.OrganizationID.EQ(postgres.String(organizationID))).
				AND(table.OrganizationInvitations.ProductRoleID.EQ(postgres.String(role.PublicID()))).
				AND(pendingInvitation()),
		),
	).Value()
	return members + invited, err
}

func pendingInvitation() postgres.BoolExpression {
	return table.OrganizationInvitations.AcceptedAt.IS_NULL().
		AND(table.OrganizationInvitations.ExpiresAt.GT(postgres.TimestampzT(time.Now())))
}

func (r *repositoryImpl) OrganizationsOnTemplate(ctx context.Context, productID, templateID string) ([]string, error) {
	held, err := transactor.QueryMapSlice(ctx, r.db,
		table.OrganizationMemberships.SELECT(table.OrganizationMemberships.OrganizationID).DISTINCT().WHERE(
			table.OrganizationMemberships.ProductRoleID.EQ(postgres.String(templateID)),
		),
		func(row model.OrganizationMemberships) string { return row.OrganizationID },
	).Value()
	if err != nil {
		return nil, err
	}
	invited, err := transactor.QueryMapSlice(ctx, r.db,
		table.OrganizationInvitations.SELECT(table.OrganizationInvitations.OrganizationID).DISTINCT().WHERE(
			table.OrganizationInvitations.ProductID.EQ(postgres.String(productID)).
				AND(table.OrganizationInvitations.ProductRoleID.EQ(postgres.String(templateID))).
				AND(pendingInvitation()),
		),
		func(row model.OrganizationInvitations) string { return row.OrganizationID },
	).Value()
	if err != nil {
		return nil, err
	}
	return functional.Distinct(append(held, invited...)), nil
}

func (r *repositoryImpl) DetachCopies(
	ctx context.Context, productID, templateID string,
) ([]organizationrole.Role, error) {
	copies := table.ProductRoles.TemplateID.EQ(postgres.String(templateID)).
		AND(table.ProductRoles.ProductID.EQ(postgres.String(productID)))
	held := postgres.EXISTS(
		postgres.SELECT(postgres.Int(1)).FROM(table.OrganizationMemberships).WHERE(
			table.OrganizationMemberships.ProductRoleID.EQ(table.ProductRoles.ID),
		),
	)
	invited := postgres.EXISTS(
		postgres.SELECT(postgres.Int(1)).FROM(table.OrganizationInvitations).WHERE(
			table.OrganizationInvitations.OrganizationID.EQ(table.ProductRoles.OrganizationID).
				AND(table.OrganizationInvitations.ProductRoleID.EQ(postgres.String(templateID))).
				AND(pendingInvitation()),
		),
	)
	kept, err := transactor.Query[[]model.ProductRoles](ctx, r.db,
		table.ProductRoles.SELECT(table.ProductRoles.AllColumns).WHERE(
			copies.AND(held.OR(invited).OR(table.ProductRoles.Customized.IS_TRUE())),
		).FOR(postgres.UPDATE()),
	).Value()
	if err != nil || len(kept) == 0 {
		return nil, err
	}
	rowIDs := jetx.ToStringExpressions(
		functional.Slice(kept).Map(func(row model.ProductRoles) string { return row.ID }),
	)
	if err = transactor.Exec(ctx, r.db,
		table.OrganizationInvitations.UPDATE(table.OrganizationInvitations.ProductRoleID).
			SET(table.ProductRoles.ID).
			FROM(table.ProductRoles).
			WHERE(
				table.ProductRoles.ID.IN(rowIDs...).
					AND(table.OrganizationInvitations.OrganizationID.EQ(table.ProductRoles.OrganizationID)).
					AND(table.OrganizationInvitations.ProductRoleID.EQ(postgres.String(templateID))).
					AND(pendingInvitation()),
			),
	).Err(); err != nil {
		return nil, err
	}
	// A copy that was its Organization's default through the Product's default
	// template stays its default: the Organization chose no other.
	template := table.ProductRoles.AS("template")
	chosen := table.ProductRoles.AS("chosen")
	if err = transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.IsDefault).SET(postgres.Bool(true)).WHERE(
			table.ProductRoles.ID.IN(rowIDs...).
				AND(postgres.EXISTS(postgres.SELECT(postgres.Int(1)).FROM(template).WHERE(
					template.ID.EQ(postgres.String(templateID)).AND(template.IsDefault.IS_TRUE()),
				))).
				AND(postgres.NOT(postgres.EXISTS(postgres.SELECT(postgres.Int(1)).FROM(chosen).WHERE(
					chosen.OrganizationID.EQ(table.ProductRoles.OrganizationID).AND(chosen.IsDefault.IS_TRUE()),
				)))),
		),
	).OnUnique(ErrDefaultExists, organizationDefaultIndex).Err(); err != nil {
		return nil, err
	}
	if err = transactor.Exec(ctx, r.db,
		table.ProductRoles.UPDATE(table.ProductRoles.TemplateID, table.ProductRoles.Customized).
			SET(postgres.Raw("NULL"), postgres.Bool(false)).
			WHERE(table.ProductRoles.ID.IN(rowIDs...)),
	).Err(); err != nil {
		return nil, err
	}
	return functional.Slice(kept).Map(func(row model.ProductRoles) organizationrole.Role {
		row.TemplateID = nil
		return toDomain(row, nil)
	}), nil
}
