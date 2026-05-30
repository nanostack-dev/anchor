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
