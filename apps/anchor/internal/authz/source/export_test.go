package source

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
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

// ResourceAssignmentTuples are the assignments held on a resource.
func ResourceAssignmentTuples(slug, rowID string, holders ...Holder) []fga.Tuple {
	members := make([]member, 0, len(holders))
	for _, h := range holders {
		members = append(members, member{productUserID: h.ProductUserID, roleRowID: h.RoleRowID})
	}
	return linkedAssignments(model.ResourceObject(slug, rowID), rowID, members)
}

func AssignmentRoleTuple(assignment string) functional.Option[fga.Tuple] {
	return assignmentRoleTuple(assignment)
}

// ResourceParentTuple is the parent tuple of a resource; an empty parent
// row ID puts it at the top of its Organization.
func ResourceParentTuple(organizationID, slug, rowID, parentSlug, parentRowID string) fga.Tuple {
	row := resourceRow{ID: rowID, OrganizationID: organizationID, Type: slug}
	if parentRowID != "" {
		row.ParentID, row.ParentType = &parentRowID, &parentSlug
	}
	return resourceParentTuple(row)
}

func RoleGrantTuples(roleRowID string, permissions []string) []fga.Tuple {
	return roleGrantTuples(roleRowID, permissions)
}
