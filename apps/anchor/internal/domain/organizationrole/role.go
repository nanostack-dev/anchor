// Package organizationrole holds the roles each Organization owns (ADR-0022
// D8): a copy of each of its Product's role templates, and later its own
// custom roles.
package organizationrole

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/resourcetype"
	"anchor/internal/searchx"
)

// Origin says where an Organization role comes from, for display only.
type Origin string

const (
	OriginTemplate Origin = "template"
	OriginCustom   Origin = "custom"
)

type Role struct {
	// RowID is the role's own row; memberships point at it. A copy is
	// addressed by its template's ID instead, see PublicID.
	RowID          string
	ProductID      string
	OrganizationID string
	TemplateID     *string
	Key            string
	Name           string
	Description    string
	ResourceTypeID *string
	ResourceType   string
	IsDefault      bool
	// Customized marks a copy the Organization changed: it keeps its own
	// bundle when its template changes, until it is reset (ADR-0022 D10).
	Customized  bool
	Permissions []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PublicID is the ID callers see and send: a copy keeps its template's ID
// inside its Organization, so the role IDs released clients hold keep
// working; a custom role has its own.
func (r Role) PublicID() string {
	if r.TemplateID != nil {
		return *r.TemplateID
	}
	return r.RowID
}

func (r Role) Origin() Origin {
	if r.TemplateID != nil {
		return OriginTemplate
	}
	return OriginCustom
}

// NewRowID returns a new prefixed KSUID for an Organization role's row.
func NewRowID() string {
	return ids.MustNew("product_role")
}

type Filter struct {
	IDs           []string `validate:"omitempty,max=100,dive,notblank"`
	Keys          []string `validate:"omitempty,max=100,dive,notblank"`
	ResourceTypes []string `validate:"omitempty,max=50,dive,notblank"`
	// Scopes is what the service resolves ResourceTypes to.
	Scopes resourcetype.ScopeFilter
}

type SortField string

const (
	SortFieldCreatedAt SortField = "created_at"
	SortFieldKey       SortField = "key"
	SortFieldName      SortField = "name"
)

type SearchInput struct {
	ProductID      string                            `validate:"required,notblank"`
	OrganizationID string                            `validate:"required,notblank"`
	Request        search.Request[Filter, SortField] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request, SortFieldCreatedAt, SortFieldKey, SortFieldName)
}

// MaxCustomRoles caps the custom roles one Organization holds (ADR-0022 D10).
const MaxCustomRoles = 50

// CreateInput is a custom role of an Organization (ADR-0022 D9).
type CreateInput struct {
	ProductID      string   `validate:"required,notblank"`
	OrganizationID string   `validate:"required,notblank"`
	Key            string   `validate:"omitempty,notblank,max=100"`
	Name           string   `validate:"required,notblank,min=2,max=100"`
	Description    *string  `validate:"omitempty,max=500"`
	ResourceType   string   `validate:"max=49"`
	Permissions    []string `validate:"max=100,unique,dive,notblank"`
}

type UpdateInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	RoleID         string `validate:"required,notblank"`
	IsDefault      *bool
	Name           *string  `validate:"omitempty,notblank,min=2,max=100"`
	Description    *string  `validate:"omitempty,max=500"`
	Permissions    []string `validate:"omitempty,max=100,unique,dive,notblank"`
	// PermissionsSet tells an empty Permissions from an omitted one.
	PermissionsSet bool
}

// ChangesBundle reports whether the update rewrites the role itself, which
// customizes a copy of a template.
func (in UpdateInput) ChangesBundle() bool {
	return in.Name != nil || in.Description != nil || in.PermissionsSet
}

var (
	ErrNotFound = fault.NotFound(
		"ROLE_NOT_FOUND",
		"This organization has no role with that identifier.",
	)

	ErrDefaultNotOrganizationScoped = fault.BadRequest(
		"DEFAULT_ROLE_NOT_ORGANIZATION_SCOPED",
		"A default role is the role a member gets on joining, so it is scoped to organization.",
	)

	ErrDefaultConflict = fault.Conflict(
		"DEFAULT_ROLE_CONFLICT",
		"Another request changed the default role. Try again.",
	)

	ErrRoleRequired = fault.BadRequest(
		"ROLE_REQUIRED",
		"Name a role: this organization has no default role.",
	)

	ErrCustomRolesDisabled = fault.BadRequest(
		"CUSTOM_ROLES_DISABLED",
		"This product doesn't allow custom roles on that resource type.",
	)

	ErrTemplateCopyNotDeletable = fault.Conflict(
		"TEMPLATE_COPY_NOT_DELETABLE",
		"A copy of a role template goes with its template. Reset it or delete the template instead.",
	)

	ErrNotATemplateCopy = fault.Conflict(
		"NOT_A_TEMPLATE_COPY",
		"Only a copy of a role template can be reset.",
	)
)

// NewCustomRoleLimitError answers a custom role past the Organization's limit.
func NewCustomRoleLimitError() *fault.Error {
	return fault.Conflict(
		"CUSTOM_ROLE_LIMIT_REACHED",
		"This organization already has the most custom roles it can hold.",
	).Metadata(map[string]any{"limit": MaxCustomRoles})
}

// NewKeyExistsError answers a custom role key that another role of the
// Organization, or a role template of its Product, already holds.
func NewKeyExistsError(key string) *fault.Error {
	return fault.Conflict(
		"ROLE_KEY_DUPLICATE",
		"A role of this organization, or a role template of its product, already has that key.",
	).Metadata(map[string]any{"key": key})
}

// WithScopes resolves the filter's resource types against the product's tree.
func (f Filter) WithScopes(tree resourcetype.Tree) Filter {
	f.Scopes = tree.ResolveScopes(f.ResourceTypes)
	return f
}

// NewBodyRoleNotFoundError answers a role_id in a request body that names no
// role of the Organization and no role template of its Product. The code
// differs from the 404 a role_id path parameter answers: one code at two
// statuses breaks a client that switches on code alone.
func NewBodyRoleNotFoundError(roleID string) *fault.Error {
	return fault.BadRequest(
		"ROLE_NOT_FOUND_IN_REQUEST",
		"This product has no role with that identifier.",
	).Metadata(map[string]any{"role_id": roleID})
}
