package projection

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/authz/fga"
)

// Holder is a product user and the role they hold in an Organization.
type Holder struct {
	ProductUserID string
	RoleRowID     string
}

func AssignmentTuples(organizationID string, holders ...Holder) []fga.Tuple {
	members := make([]member, 0, len(holders))
	for _, h := range holders {
		members = append(members, member{productUserID: h.ProductUserID, roleRowID: h.RoleRowID})
	}
	return assignmentTuples(organizationID, members)
}

func AssignmentRoleTuple(assignment string) functional.Option[fga.Tuple] {
	return assignmentRoleTuple(assignment)
}

func RoleGrantTuples(roleRowID string, permissions []string) []fga.Tuple {
	return roleGrantTuples(roleRowID, permissions)
}
