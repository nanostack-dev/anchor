package workspace

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

func NewNameExistsError(name, organizationID string) *fault.Error {
	return fault.Conflict("WORKSPACE_NAME_DUPLICATE", "Workspace with this name already exists in the organization").
		Metadata(map[string]any{
			"name":            name,
			"organization_id": organizationID,
		})
}

var ErrWorkspaceNotFound = fault.NotFound(
	"WORKSPACE_NOT_FOUND",
	"This organization has no workspace with that identifier",
)

var ErrMembershipNotFound = fault.NotFound(
	"WORKSPACE_MEMBERSHIP_NOT_FOUND",
	"This workspace has no role assigned to that product user",
)

// NewNotOrganizationMemberError answers a workspace role assigned to a product
// user who is not a member of the workspace's organization.
func NewNotOrganizationMemberError(productUserID, organizationID string) *fault.Error {
	return fault.Conflict(
		"WORKSPACE_MEMBER_NOT_IN_ORGANIZATION",
		"Only a member of the organization can hold a role in one of its workspaces",
	).Metadata(map[string]any{
		"product_user_id": productUserID,
		"organization_id": organizationID,
	})
}
