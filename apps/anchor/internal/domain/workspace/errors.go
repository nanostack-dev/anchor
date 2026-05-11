package workspace

import "github.com/nanostack-dev/shared/toolkit"

func NewNameExistsError(name, organizationID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"WORKSPACE_NAME_DUPLICATE",
		"Workspace with this name already exists in the organization",
		map[string]interface{}{
			"name":            name,
			"organization_id": organizationID,
		},
	)
}
