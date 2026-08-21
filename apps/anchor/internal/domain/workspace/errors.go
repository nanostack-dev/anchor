package workspace

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

func NewNameExistsError(name, organizationID string) *fault.Error {
	return fault.Conflict("WORKSPACE_NAME_DUPLICATE", "Workspace with this name already exists in the organization").
		Metadata(map[string]any{
			"name":            name,
			"organization_id": organizationID,
		})
}
