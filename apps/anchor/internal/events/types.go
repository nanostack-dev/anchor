package events

import (
	"encoding/json"
	"strings"
)

type Type string

const (
	OrganizationCreated Type = "organization.created"
	OrganizationUpdated Type = "organization.updated"
	OrganizationDeleted Type = "organization.deleted"

	MembershipCreated Type = "organization.membership.created"
	MembershipUpdated Type = "organization.membership.updated"
	MembershipDeleted Type = "organization.membership.deleted"

	OrganizationInvitationCreated  Type = "organization.invitation.created"
	OrganizationInvitationUpdated  Type = "organization.invitation.updated"
	OrganizationInvitationDeleted  Type = "organization.invitation.deleted"
	OrganizationInvitationAccepted Type = "organization.invitation.accepted"

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

	OrganizationRoleCreated Type = "organization.role.created"
	OrganizationRoleUpdated Type = "organization.role.updated"
	OrganizationRoleDeleted Type = "organization.role.deleted"

	ProductResourcePermissionCreated Type = "product.resource_permission.created"
	ProductResourcePermissionUpdated Type = "product.resource_permission.updated"
	ProductResourcePermissionDeleted Type = "product.resource_permission.deleted"

	ProductResourceTypeCreated Type = "product.resource_type.created"
	ProductResourceTypeUpdated Type = "product.resource_type.updated"
	ProductResourceTypeDeleted Type = "product.resource_type.deleted"

	ResourceCreated Type = "resource.created"
	ResourceUpdated Type = "resource.updated"
	ResourceDeleted Type = "resource.deleted"

	ResourceRoleAssignmentCreated Type = "resource.role_assignment.created"
	ResourceRoleAssignmentUpdated Type = "resource.role_assignment.updated"
	ResourceRoleAssignmentDeleted Type = "resource.role_assignment.deleted"

	ClerkUserCreated Type = "clerk.user.created"
	ClerkUserUpdated Type = "clerk.user.updated"
	ClerkUserDeleted Type = "clerk.user.deleted"
)

const (
	FieldOrganizationID = "organization_id"
	FieldProductUserID  = "product_user_id"
	FieldInvitationID   = "invitation_id"
	FieldAPIKeyID       = "api_key_id"
	FieldRoleID         = "role_id"
	FieldPermissionName = "permission_name"
	FieldResourceTypeID = "resource_type_id"
	FieldResourceType   = "resource_type"
	FieldResourceID     = "resource_id"
	// FieldResourceRowID is Anchor's row ID of a resource. It names the
	// resource in the relationship store for the observers; the emitter drops
	// it from the data a product endpoint receives.
	FieldResourceRowID = "resource_row_id"
	// FieldCascadedOrganizationIDs lists, joined by JoinIDs, the
	// Organizations whose memberships a product user or product role delete
	// takes through the foreign keys, without membership events. It is for
	// the observers only, like FieldResourceRowID. A delete that leaves it
	// out makes every Organization's assignments stale; an empty value names
	// none.
	FieldCascadedOrganizationIDs = "cascaded_organization_ids"
)

// isInternalField reports a field the observers read and a product never receives.
func isInternalField(field string) bool {
	return field == FieldResourceRowID || field == FieldCascadedOrganizationIDs
}

// idSeparator joins IDs in one field. Anchor's IDs are prefixed KSUIDs, which
// never hold it.
const idSeparator = ","

// JoinIDs packs IDs into one data field.
func JoinIDs(ids []string) string { return strings.Join(ids, idSeparator) }

// SplitIDs unpacks a field JoinIDs wrote; an empty field holds no ID.
func SplitIDs(field string) []string {
	if field == "" {
		return nil
	}
	return strings.Split(field, idSeparator)
}

type Data map[string]string

func OrganizationRoleData(organizationID, roleID string) Data {
	return Data{
		FieldOrganizationID: organizationID,
		FieldRoleID:         roleID,
	}
}

func InvitationData(organizationID, invitationID string) Data {
	return Data{
		FieldOrganizationID: organizationID,
		FieldInvitationID:   invitationID,
	}
}

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
