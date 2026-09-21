package events

import "encoding/json"

type Type string

const (
	OrganizationCreated Type = "organization.created"
	OrganizationUpdated Type = "organization.updated"
	OrganizationDeleted Type = "organization.deleted"

	MembershipCreated Type = "organization.membership.created"
	MembershipUpdated Type = "organization.membership.updated"
	MembershipDeleted Type = "organization.membership.deleted"

	WorkspaceCreated Type = "workspace.created"
	WorkspaceUpdated Type = "workspace.updated"
	WorkspaceDeleted Type = "workspace.deleted"

	OrganizationAPIKeyCreated Type = "organization.api_key.created"
	OrganizationAPIKeyUpdated Type = "organization.api_key.updated"
	OrganizationAPIKeyDeleted Type = "organization.api_key.deleted"

	ProductUserCreated Type = "product_user.created"
	ProductUserUpdated Type = "product_user.updated"
	ProductUserDeleted Type = "product_user.deleted"

	OrganizationLicenseUpdated Type = "organization.license.updated"

	ProductRoleCreated Type = "product.role.created"
	ProductRoleUpdated Type = "product.role.updated"
	ProductRoleDeleted Type = "product.role.deleted"

	ProductResourcePermissionCreated Type = "product.resource_permission.created"
	ProductResourcePermissionUpdated Type = "product.resource_permission.updated"
	ProductResourcePermissionDeleted Type = "product.resource_permission.deleted"
)

const (
	FieldOrganizationID = "organization_id"
	FieldProductUserID  = "product_user_id"
	FieldWorkspaceID    = "workspace_id"
	FieldAPIKeyID       = "api_key_id"
	FieldRoleID         = "role_id"
	FieldPermissionName = "permission_name"
)

type Data map[string]string

type Event struct {
	Type      Type   `validate:"required"`
	ProductID string `validate:"required,notblank"`
	Data      Data   `validate:"required"`
}

type Envelope struct {
	Type      Type            `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}
