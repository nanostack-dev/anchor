package role

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type ProductRole struct {
	ID          string
	ProductID   string
	Name        string
	Description string
	// ResourceTypeID is nil only on a row a release before resource types
	// wrote during a rolling deploy; it counts as organization.
	ResourceTypeID *string
	// ResourceType is the slug of the scope, filled by the service.
	ResourceType string
	// Key names the role template in every Organization: each copy keeps it.
	Key         string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Permissions []ProductRolePermission
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
