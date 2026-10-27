package role

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Type tells a role the Product manages from an Organization's own role.
type Type string

// Ref is the role a member holds, as reads show it. A member who holds no
// role has none: a nil *Ref, never an empty one.
type Ref struct {
	ID   string
	Name string
}

const (
	// TypeManaged is a role the Product manages. Every Organization of the
	// Product references it by the same ID, so a change reaches all of them.
	TypeManaged Type = "MANAGED"
	// TypeCustom is a role one Organization owns.
	TypeCustom Type = "CUSTOM"
)

type ProductRole struct {
	ID        string
	ProductID string
	// OrganizationID is set on a custom role only.
	OrganizationID string
	Type           Type
	Name           string
	Description    string
	ResourceTypeID string
	// ResourceType is the slug of the scope, filled by the service.
	ResourceType string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Permissions  []ProductRolePermission
}

// NewManaged starts a role the Product manages, with a new ID.
func NewManaged(productID, name, description string) ProductRole {
	r := ProductRole{ProductID: productID, Type: TypeManaged, Name: name, Description: description}
	r.GenerateID()
	return r
}

// NewCustom starts a role that the Organization owns, with a new ID.
func NewCustom(productID, organizationID, name, description string) ProductRole {
	r := ProductRole{
		ProductID: productID, OrganizationID: organizationID, Type: TypeCustom, Name: name, Description: description,
	}
	r.GenerateID()
	return r
}

// TypeOf reads a stored type. A row a release before types wrote has none,
// and that release wrote only roles the Product manages.
func TypeOf(stored *string) Type {
	if stored == nil {
		return TypeManaged
	}
	return Type(*stored)
}

// AssignableIn reports whether a member of the Organization may hold the
// role: a managed role, or a custom role of that Organization.
func (r *ProductRole) AssignableIn(organizationID string) bool {
	return r.Type == TypeManaged || r.OrganizationID == organizationID
}

// GenerateID sets the role's ID to a new prefixed KSUID.
func (r *ProductRole) GenerateID() {
	r.ID = ids.MustNew("product_role")
}

type ProductRolePermission struct {
	ID             string
	ProductRoleID  string
	ProductID      string
	PermissionName string
}

// GenerateID sets the permission's ID to a new prefixed KSUID.
func (p *ProductRolePermission) GenerateID() {
	p.ID = ids.MustNew("product_role_resource_permission")
}
